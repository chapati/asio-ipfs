package main

import (
    "log"
    "fmt"
    "runtime"
    "context"
    "encoding/json"
    "math/rand"
    "time"
    "sort"
    "unsafe"

    "github.com/ipfs/go-ipfs/repo/fsrepo"
    "github.com/ipfs/go-ipfs/core"
    "github.com/ipfs/go-ipfs/core/corehttp"
    "github.com/prometheus/client_golang/prometheus/promauto"
    "github.com/prometheus/client_golang/prometheus"
    "github.com/ipfs/go-ipfs/core/coreapi"
    "github.com/ipfs/go-ipfs/plugin/loader"
    "github.com/ipfs/go-ipfs/core/commands/keyencode"
    "github.com/ipfs/go-ipfs-config"

    version "github.com/ipfs/go-ipfs"
    mprome "github.com/ipfs/go-metrics-prometheus"
    oldcmds "github.com/ipfs/go-ipfs/commands"
    coreiface "github.com/ipfs/interface-go-ipfs-core"
)

//#include <stdlib.h>
//#include <stddef.h>
//#include <stdint.h>
//#include <ipfs_error_codes.h>
import "C"

type Node struct {
    ctx context.Context
    cctx *oldcmds.Context
    node *core.IpfsNode
    api coreiface.CoreAPI
    collector *corehttp.IpfsNodeCollector
	cancel context.CancelFunc
	next_cancel_signal_id C.uint64_t
	cancel_signals map[C.uint64_t]func()
	state_cb unsafe.Pointer
	state_cb_arg unsafe.Pointer
}

// vars below can be accessed only from C thread
var (
    g_mprome_injected = false
    g_mporme_metrics = false
    g_plugins_injected = false
    g_node *Node
)

func loadPlugins(repoPath string) (*loader.PluginLoader, error) {
	plugins, err := loader.NewPluginLoader(repoPath)
	if err != nil {
		return nil, fmt.Errorf("error loading plugins: %s", err)
	}

	if err := plugins.Initialize(); err != nil {
		return nil, fmt.Errorf("error initializing plugins: %s", err)
	}

    // TODO:NEXTVERIPFS implement manual injecting plugin by plugin
    if !g_plugins_injected {
	    if err := plugins.Inject(); err != nil {
    		return nil, fmt.Errorf("error initializing plugins: %s", err)
	    }
	    g_plugins_injected = true
	} else {
	    // This is hacky AF but necessary to start/restart node
	    // TODO:NEXTVERIPFS fix what's going on here OR check that layout is not changed
	    ldptr := unsafe.Pointer(plugins)
	    stateptr := (*int)(ldptr)
	    *stateptr = 4 // plugins.state = loaderInjected
	}

	return plugins, nil
}

func buildEnv(n *Node, repoRoot string) (*oldcmds.Context, error) {
    log.Println("IPFS repository path is", repoRoot)

    plugins, err := loadPlugins(repoRoot)
    if err != nil {
        return nil, err
    }

    return &oldcmds.Context {
        ConfigRoot: repoRoot,
        LoadConfig: nil,
        ReqLog:     &oldcmds.ReqLog{},
        Plugins:    plugins,
        ConstructNode: func() (*core.IpfsNode, error) {
            if n.node == nil {
                return nil, fmt.Errorf("unexpected ConstructNode, n.node is still nil")
            }
            return n.node, nil
        },
    }, nil
}

func start_node(jsoncfg string, n *Node, repoRoot string) C.int {
    rand.Seed(time.Now().UnixNano())

    var asiocfg AsioConfig
    err := json.Unmarshal([]byte(jsoncfg), &asiocfg)

    if err != nil {
        log.Println("Failed to parse config", err);
        return C.IPFS_PARSE_CONFIG_FAIL
    }

    if !g_mprome_injected {
        err = mprome.Inject()
        if err != nil {
            log.Println("Injecting prometheus handler for metrics failed:", err)
            return C.IPFS_MPROME_INJECT_FAILED
        }
        g_mprome_injected = true
    }

    printVersion()

    //
    // Construct global environment
    //
    if n.cctx, err = buildEnv(n, repoRoot); err != nil {
        log.Println("Failed to build environment:", err)
        return C.IPFS_BUILD_ENV_FAILED
    }

    if !fsrepo.IsInitialized(n.cctx.ConfigRoot) {
        if err = initRepo(n.cctx.ConfigRoot, &asiocfg); err != nil {
            log.Println("Failed in initRepo:", err)
            return C.IPFS_CREATE_REPO_FAILED
        }
    }

    repo, err := openRepo(n.cctx.ConfigRoot, &asiocfg)
    if err != nil {
        log.Println("Failed to open repo,", err)
        return C.IPFS_OPEN_REPO_FAILED
    }

    cfg, err := repo.Config()
    if err != nil {
        log.Println("Unable to get repo config", err);
        return C.IPFS_READ_CONFIG_FAILED
    }

    n.cctx.LoadConfig = func(path string) (*config.Config, error) {
        if path != repoRoot {
            panic("n.cctx.LoadConfig repoRoot!=path")
        }
        return cfg, nil
    }

    //
    // All preparations completed. Build a node finally
    //
    start := time.Now()
    bcfg := makeBuildCfg(repo, cfg)
    elapsed := time.Since(start)
    log.Printf("IPFS plugins startup time %v", elapsed)

    start = time.Now()
    n.node, err = core.NewNode(n.ctx, bcfg)
    if err != nil {
        log.Println("Unable to create node", err);
        return C.IPFS_START_NODE_FAILED
    }
    elapsed = time.Since(start)
    log.Printf("IPFS core node startup time %v, relay mode is %v", elapsed, cfg.Swarm.Transports.Network.Relay)
    n.node.IsDaemon = true

    if n.node.PNetFingerprint != nil {
        log.Println("Swarm is limited to private network of peers with the swarm key")
        log.Printf("Swarm key fingerprint: %x\n", n.node.PNetFingerprint)
    } else {
        log.Println("Swarm is in a public network.")
    }

    printSwarmAddrs(n.node)

    err = n.cctx.Plugins.Start(n.node)
    if err != nil {
        log.Println("IPFS failed to start plugins:", err)
        return C.IPFS_PLUGINS_FAILED
    }

    gcErrc, err := maybeRunGC(n, &asiocfg)
    if err != nil {
        log.Println("IPFS failed to run GC:", err)
        return C.IPFS_GC_FAILED
    }

    apiErrc, err := maybeServeHTTPApi(n, &asiocfg)
    if err != nil {
        log.Println("IPFS failed to start API:", err)
        return C.IPFS_API_FAILED
    }

    // TODO:NEXTVERIPFS migrations: cacheMigrations, pinMigraions, fetcher
    gwErrc, err := maybeServeHTTPGateway(n, &asiocfg)
    if err != nil {
        return C.IPFS_GATEWAY_FAILED
    }

    // TODO:NEXTVERIPFS consider adding remote pinning (startPinMFS)

    //
    // Add ipfs version info to prometheus metrics and init metrics collector
    //
    if !g_mporme_metrics {
        var ipfsInfoMetric = promauto.NewGaugeVec(prometheus.GaugeOpts{
            Name: "ipfs_info",
            Help: "IPFS version information.",
        }, []string{"version", "commit"})
        ipfsInfoMetric.With(prometheus.Labels{
            "version": version.CurrentVersionNumber,
            "commit":  version.CurrentCommit,
        }).Set(1)
        g_mporme_metrics = true
    }

    n.collector = &corehttp.IpfsNodeCollector{
        Node: n.node,
    }
    prometheus.MustRegister(n.collector)

    // TODO:NEXTVERIPFS conside fuse mounts (mountFuse)
    n.api, err = coreapi.NewCoreAPI(n.node)
    if err != nil {
        log.Println("Unable to create core API %s", err)
        return C.IPFS_API_ACCESS_FAILED
    }
    printPeers(n)

    if (n.state_cb == nil) {
        panic("IPFS state callback is not provided")
    }

    if gcErrc != nil || apiErrc != nil || gwErrc != nil {
        go func () {
            totalPeers := func () uint32 {
                var res float64 = 0
                for _, val := range n.collector.PeersTotalValues() {
                    res += val
                }
                return uint32(res)
            }

            log.Println("IPFS state monitor is launched")
            pcnt := totalPeers()
            stateCB := makeStateCB(n)
            stateCB(nil, pcnt)

            var lasterr error
            for {
                select {
                case err := <- gcErrc:
                    if err != nil {
                        log.Println("IPFS gc error: ", err)
                        lasterr = err
                    } else {
                        gcErrc = nil
                    }

                case err := <- apiErrc:
                    if err != nil {
                        log.Println("IPFS API error: ", err)
                        lasterr = err
                    } else {
                        apiErrc = nil
                    }

                case err := <- gwErrc:
                    if err != nil {
                        log.Println("IPFS Gateway error: ", err)
                        lasterr = err
                    } else {
                        gwErrc = nil
                    }

                case <- time.After(time.Second * 2):
                    ncnt := totalPeers()
                    if pcnt != ncnt {
                        pcnt = ncnt
                        stateCB(nil, pcnt)
                    }
                    // IPFS:TEST
                    // lasterr = fmt.Errorf("error loading plugins: %s", err)

                case <- n.ctx.Done():
                    stateCB(lasterr, 0)
                    log.Println("IPFS state monitor is stopped")
                    return
                }

                if lasterr != nil {
                    n.cancel()
                }
            }
        } ()
    }

    log.Println("IPFS node is ready")
    return C.IPFS_SUCCESS
}

func printVersion() {
	v := version.CurrentVersionNumber
	if version.CurrentCommit != "" {
		v += "-" + version.CurrentCommit
	}
	log.Printf("go-ipfs version: %s\n", v)
	log.Printf("Repo version: %d\n", fsrepo.RepoVersion)
	log.Printf("System version: %s\n", runtime.GOARCH+"/"+runtime.GOOS)
	log.Printf("Golang version: %s\n", runtime.Version())
}

func printPeers(n *Node) {

    peers, err := n.api.Swarm().Peers(n.node.Context())
    if err != nil {
        log.Println("Failed to read swarm peers:", err)
        return
    }

    if len(peers) == 0 {
        log.Println("WARNING: IPFS was unable to connect to peers")
    } else {
        for _, peer := range peers {
            log.Printf("IPFS Peer %v %v\n", peer.ID().Pretty(), peer.Address().String())
        }
    }
}

func printSwarmAddrs(node *core.IpfsNode) {
	if !node.IsOnline {
		log.Println("Swarm listening disabled, running in offline mode.")
		return
	}

    //
    // Listening addresses
    //
    var lisAddrs []string
    ifaceAddrs, err := node.PeerHost.Network().InterfaceListenAddresses()
    if err != nil {
        log.Println("WARNING: failed to read listening addresses:", err)
    }

    for _, addr := range ifaceAddrs {
        lisAddrs = append(lisAddrs, addr.String())
    }

    sort.Strings(lisAddrs)
    for _, addr := range lisAddrs {
        log.Printf("Swarm listening on %s\n", addr)
    }

	//
	// Announcing addresses
	//
	var addrs []string
	for _, addr := range node.PeerHost.Addrs() {
		addrs = append(addrs, addr.String())
	}

	sort.Sort(sort.StringSlice(addrs))
	for _, addr := range addrs {
		log.Printf("Swarm announcing %s\n", addr)
	}
}

func allocate_node(state_cb unsafe.Pointer, state_cb_arg unsafe.Pointer) C.int {
    if g_node != nil {
        return C.IPFS_NODE_EXISTS
    }

	g_node = &Node{}
	g_node.ctx, g_node.cancel = context.WithCancel(context.Background())
	g_node.next_cancel_signal_id = 0
	g_node.cancel_signals = make(map[C.uint64_t]func())
	g_node.state_cb = state_cb
	g_node.state_cb_arg = state_cb_arg

	return C.IPFS_SUCCESS
}

//export go_asio_ipfs_start_blocking
func go_asio_ipfs_start_blocking(c_cfg *C.char, c_repoPath *C.char, state_cb unsafe.Pointer, state_cb_arg unsafe.Pointer) C.int32_t {
	if err:= allocate_node(state_cb, state_cb_arg); err != C.IPFS_SUCCESS {
	    return err
	}

    n := g_node
	repoRoot := C.GoString(c_repoPath)
	cfg := C.GoString(c_cfg)

	if err := start_node(cfg, n, repoRoot); err != C.IPFS_SUCCESS {
	    _ = go_asio_ipfs_free
	    return err
	}

	return C.IPFS_SUCCESS
}

//export go_asio_ipfs_start_async
func go_asio_ipfs_start_async(c_cfg *C.char,
                              c_repoPath *C.char,
                              state_cb unsafe.Pointer, state_cb_arg unsafe.Pointer,
                              fn unsafe.Pointer, fn_arg unsafe.Pointer) {

	if err:= allocate_node(state_cb, state_cb_arg); err != C.IPFS_SUCCESS {
        go func () {
            executeVoidCB(fn, err, fn_arg)
        }()
        return
    }

    n := g_node
	repoRoot := C.GoString(c_repoPath)
	cfg := C.GoString(c_cfg)

	go func() {
		err := start_node(cfg, n, repoRoot)
		if err != C.IPFS_SUCCESS {
            _ = go_asio_ipfs_free
        }
		executeVoidCB(fn, err, fn_arg)
	}()
}

//export go_asio_ipfs_free
func go_asio_ipfs_free() C.int {
    n := g_node
    if n == nil {
        return C.IPFS_NO_NODE
    }

	n.cancel()
	<- n.ctx.Done()

	if n.collector != nil {
        prometheus.Unregister(n.collector)
    }

    if n.node != nil {
        n.node.Close()
    }

    if n.cctx != nil && n.cctx.Plugins != nil {
        if err := n.cctx.Plugins.Close(); err != nil {
            log.Println("IPFS failed to close plugins. Next node start might fail")
        }
    }

	g_node = nil
	log.Println("IPFS node gracefully stopped")

	return C.IPFS_SUCCESS
}

// IMPORTANT: The returned value needs to be explicitly go_asio_memfree-d
//export go_asio_ipfs_node_id
func go_asio_ipfs_node_id() *C.char {
	n := g_node
	if n == nil {
	    return C.CString("")
	}

	ke, err := keyencode.KeyEncoderFromString("b58mh")
	if err != nil {
	    // should never fail, so just panic
	    panic(err)
	}

	id :=  ke.FormatID(n.node.Identity)
	cstr := C.CString(id)

    return cstr
}
