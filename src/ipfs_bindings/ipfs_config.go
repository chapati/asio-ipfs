package main

import (
    "os"
    "fmt"
    "log"
    "errors"
    "strings"
    "strconv"
    "path/filepath"
    "github.com/ipfs/go-ipfs-config"
    "github.com/ipfs/go-ipfs/core"
    "github.com/ipfs/go-ipfs/repo"
    "github.com/ipfs/go-ipfs/core/node/libp2p"
    ma "github.com/multiformats/go-multiaddr"
)

type AsioConfig struct {
	LowWater          int       `json:"LowWater"`
	HighWater         int       `json:"HighWater"`
	GracePeriod       string    `json:"GracePeriod"`
	AutoRelay         bool      `json:"AutoRelay"`
	RelayHop          bool      `json:"RelayHop"`
	Bootstrap         []string  `json:"Bootstrap"`
	SwarmPort         int       `json:"SwarmPort"`
	APIPort           int       `json:"APIPort"`
	GatewayPort       int       `json:"GatewayPort"`
	DefaultProfile    string    `json:"DefaultProfile"`
	StorageMax        string    `json:"StorageMax"`
	AutoNAT           bool      `json:"AutoNAT"`
	AutoNATLimit      int       `json:"AutoNATLimit"`
	AutoNATPeerLimit  int       `json:"AutoNATPeerLimit"`
	SwarmKey          string    `json:"SwarmKey"`
	RoutingType       string    `json:"RoutingType"`
	RunGC             bool      `json:"RunGC"`
}

const (
    configLock = "config.lock"
)

func formRepoPath(repoRoot string, fname string) string {
    repoPath := filepath.Clean(repoRoot)
    return filepath.Join(repoPath, fname)
}

func configUnlocked(repoRoot string) bool {
    cpath := formRepoPath(repoRoot, configLock)
    if _, err := os.Stat(cpath); errors.Is(err, os.ErrNotExist) {
        return true
    }
    return false
}

func replacePorts (addrs []string, newPort int) error {
    var replacePort = func (addr string, proto int, prefix string, newPort int) (string, error) {
        maddr, err := ma.NewMultiaddr(addr)
        if err != nil {
            return "", err
        }

        ma.ForEach(maddr, func(comp ma.Component) bool {
            if comp.Protocol().Code == proto {
                sport := comp.Value()
                what := prefix + sport
                with := prefix + strconv.FormatInt(int64(newPort), 10)
                addr = strings.Replace(addr, what, with, -1)
            }
            return true
        })

        return addr, nil
    }

    for idx, addr := range addrs {
        addr, err := replacePort(addr, ma.P_TCP, "/tcp/", newPort)
        if err != nil {
            return err
        }

        addr, err = replacePort(addr, ma.P_UDP, "/udp/", newPort)
        if err != nil {
            return err
        }

        addrs[idx] = addr
    }

    return nil
}

func updateConfig(conf *config.Config, c *AsioConfig) error {
    //
    // Swarm connection manager
    //
    conf.Swarm.ConnMgr.LowWater = c.LowWater
    conf.Swarm.ConnMgr.HighWater = c.HighWater
    conf.Swarm.ConnMgr.GracePeriod = c.GracePeriod

    conf.Swarm.EnableAutoRelay = c.AutoRelay
    conf.Swarm.EnableRelayHop = c.RelayHop
    conf.Swarm.Transports.Network.Relay = config.True
    conf.Routing.Type = c.RoutingType
    conf.Datastore.StorageMax = c.StorageMax

    if (c.AutoNAT) {
        conf.AutoNAT.ServiceMode = config.AutoNATServiceEnabled
    } else {
        conf.AutoNAT.ServiceMode = config.AutoNATServiceDisabled
    }

    conf.AutoNAT.Throttle = &config.AutoNATThrottleConfig {
        GlobalLimit: c.AutoNATLimit,
        PeerLimit: c.AutoNATPeerLimit,
    }

    //
    // Adjust Swarm port
    //
    if err := replacePorts(conf.Addresses.Swarm, c.SwarmPort); err != nil {
        return err
    }

    //
    // API port
    //
    if c.APIPort == 0 {
        // Zero port is passed, it means we do not want to spin up any API
        // Pass slice, otherwise it is marshalled to json as `:null` and casue crash on read
        conf.Addresses.API = make([]string, 0)
    } else {
        if len(conf.Addresses.API) == 0 {
            conf.Addresses.API = []string{"/ip4/127.0.0.1/tcp/0"}
        }
        if err := replacePorts(conf.Addresses.API, c.APIPort); err != nil {
            return err
        }
    }

    //
    // Gateway Port
    //
    if c.GatewayPort == 0 {
        // Zero port is passed, it means we do not want to spin up any API
        // Pass slice, otherwise it is marshalled to json as `:null` and casue crash on read
        conf.Addresses.Gateway = make([]string, 0)
    } else {
        if len(conf.Addresses.Gateway) == 0 {
            conf.Addresses.Gateway = []string{"/ip4/127.0.0.1/tcp/0"}
        }
        if err := replacePorts(conf.Addresses.Gateway, c.GatewayPort); err != nil {
            return err
        }
    }

    //
    // Bootstrap peers
    //
    if len (c.Bootstrap) == 0 {
        log.Println("WARNING: empty bootstrap peers in config. Default IPFS bootstrap peers would be used.")
    } else {
        ps, err :=  config.ParseBootstrapPeers(c.Bootstrap)
        if err != nil {
            return fmt.Errorf("Failed to parse bootstrap peers: %s", err)
        }
        conf.Bootstrap = config.BootstrapPeerStrings(ps)
    }

    return nil
}

func updateRepo(repoRoot string, acfg *AsioConfig) error {
    //
    // Manually add swarm key if any
    // N.B. If decide to switch to a public network need to update QUIC settings & ports
    //
    if acfg.SwarmKey == "" {
        return nil
    }

    spath := formRepoPath(repoRoot, "swarm.key")
    f, err := os.Create(spath)
    if err != nil {
        return err
    }
    defer f.Close()

    _, err = f.WriteString(acfg.SwarmKey)
    if err != nil {
        return err
    }

    return nil
}

func makeBuildCfg(repo repo.Repo, cfg *config.Config) *core.BuildCfg {
     ncfg := &core.BuildCfg{
        Repo:      repo,
        Permanent: true,
        Online:    true,
        DisableEncryptedConnections: false,
        // TODO:IPNS enable
        ExtraOpts: map[string]bool{
            "pubsub": false,
            "ipnsps": false,
        },
    }

    routingOption := cfg.Routing.Type
    if routingOption == "" {
        routingOption = "dht"
    }

    switch routingOption {
    case "dhtclient":
        ncfg.Routing = libp2p.DHTClientOption
    case "dht":
        ncfg.Routing = libp2p.DHTOption
    case "dhtserver":
        ncfg.Routing = libp2p.DHTServerOption
    case "none":
        ncfg.Routing = libp2p.NilRouterOption
    default:
        err := fmt.Errorf("unrecognized routing option: %s", routingOption)
        panic(err.Error())
    }

    return ncfg
}
