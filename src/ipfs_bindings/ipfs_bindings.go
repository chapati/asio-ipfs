// Useful links:
// https://github.com/ipfs/go-ipfs/issues/3060
// https://github.com/ipfs/examples/tree/master/examples

package main

import (
	"fmt"
	"context"
	"os"
	"sort"
	"unsafe"
	"time"
	"io"
	"strings"
	"io/ioutil"
	"encoding/json"
	"path/filepath"
	core "github.com/ipfs/go-ipfs/core"
	coreapi "github.com/ipfs/go-ipfs/core/coreapi"
	corerepo "github.com/ipfs/go-ipfs/core/corerepo"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	corepath "github.com/ipfs/interface-go-ipfs-core/path"
	corehttp "github.com/ipfs/go-ipfs/core/corehttp"
	repo "github.com/ipfs/go-ipfs/repo"
	fsrepo "github.com/ipfs/go-ipfs/repo/fsrepo"
	plugin "github.com/ipfs/go-ipfs/plugin"
	flatfs "github.com/ipfs/go-ipfs/plugin/plugins/flatfs"
	levelds "github.com/ipfs/go-ipfs/plugin/plugins/levelds"
    oldcmds "github.com/ipfs/go-ipfs/commands"
    cid "github.com/ipfs/go-cid"
    ma "github.com/multiformats/go-multiaddr"

	"github.com/ipfs/go-ipfs-config"
	"github.com/ipfs/interface-go-ipfs-core/options"

	path "github.com/ipfs/go-path"
	// peer "github.com/libp2p/go-libp2p-peer" causes deprecate error
	files "github.com/ipfs/go-ipfs-files"

	mprome "github.com/ipfs/go-metrics-prometheus"
	"github.com/prometheus/client_golang/prometheus"
)

// #cgo CFLAGS: -DIN_GO=1 -ggdb -I ${SRCDIR}/../../include
// #cgo android LDFLAGS: -Wl,--unresolved-symbols=ignore-all
//#include <stdlib.h>
//#include <stddef.h>
//#include <stdint.h>
//#include <asio_ipfs/ipfs_error_codes.h>
//
//// Don't export these functions into C or we'll get "unused function" warnings
//// (Or errors saying functions are defined more than once if the're not static).
//
//#if IN_GO
//static void execute_void_cb(void* func, int err, void* arg)
//{
//    ((void(*)(int, void*)) func)(err, arg);
//}
//static void execute_data_cb(void* func, int err, void* data, size_t size, void* arg)
//{
//    ((void(*)(int, char*, size_t, void*)) func)(err, data, size, arg);
//}
//#endif // if IN_GO
import "C"

const (
	nBitsForKeypair = 2048
	repoRoot = "./repo"
	debug = false

	// This next option makes IPNS resolution much faster:
	//
	// https://blog.ipfs.io/34-go-ipfs-0.4.14#ipns-improvements
	// https://github.com/ipfs/go-ipfs/blob/master/docs/experimental-features.md#ipns-pubsub

	enablePubSubIPNS = true

	// https://github.com/ipfs/go-ipfs/blob/master/docs/experimental-features.md#quic
	enableQuic = true
)

type Config struct {
	Online      bool     `json:"online"`
	LowWater    int      `json:"low_water"`
	HighWater   int      `json:"high_water"`
	GracePeriod string   `json:"grace_period"`
	Bootstrap   []string `json:"bootstrap"`
}

func main() {
}

func doesnt_exist_or_is_empty(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return true
		}
		return false
	}
	defer f.Close()

	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true
	}
	return false
}


// "/ip4/0.0.0.0/tcp/4001" -> "/ip4/0.0.0.0/tcp/0"
// "/ip6/::/tcp/4001"      -> "/ip6/::/tcp/0"
func setRandomPort(ep string) string {
    fmt.Printf("addr %s\n", ep)
	parts := strings.Split(ep, "/")
	l := len(parts);
	for i := 0; i < l; i++ {
	    if parts[i] == "4001" {
	        parts[i] = "0"
	    }
	}
	return strings.Join(parts, "/")
}

func openOrCreateRepo(repoRoot string, c Config) (repo.Repo, error) {
	if doesnt_exist_or_is_empty(repoRoot) {
		conf, err := config.Init(os.Stdout, nBitsForKeypair)
		//fmt.Printf("%v", conf);

		if err != nil {
			return nil, err
		}

		// Don't use hardcoded swarm ports (usually 4001), otherwise
		// we wouldn't be able to run multiple IPFS instances on the
		// same PC.
		for i, addr := range conf.Addresses.Swarm {
			conf.Addresses.Swarm[i] = setRandomPort(addr)
		}

        // TODO:IPFS check --enable-gc
        // TODO:IPFS check if QUIC is already in conf.Addresses
		//if (enableQuic) {
		//	conf.Experimental.QUIC = true
		//	conf.Addresses.Swarm = append(conf.Addresses.Swarm, "/ip4/0.0.0.0/udp/0/quic")
		//}
		conf.Swarm.ConnMgr.LowWater = c.LowWater
		conf.Swarm.ConnMgr.HighWater = c.HighWater
		conf.Swarm.ConnMgr.GracePeriod = c.GracePeriod

        //
        // Bootstrap peers
        //
        {
            if len (c.Bootstrap) == 0 {
                return nil, fmt.Errorf("Config.Bootstrap cannot be empty")
            }

            ps, err :=  config.ParseBootstrapPeers(c.Bootstrap)
            if err != nil {
                return nil, fmt.Errorf("Failed to parse bootstrap peers: %s", err)
            }

            conf.Bootstrap = config.BootstrapPeerStrings(ps)
        }

		// TODO:IPFS /dnsaddr/
		// TODO:IPFS change to default ports
		// TODO:IPFS api server & web ui?
		// TODO:IPFS why pins are not shown in web UI?
		// TODO:IPFS ensure we're not on a public network, what is indirect pin?
		// TODO:IPFS & setenforce
		// TODO:IPFS PASS SWARM KEY
		if err := fsrepo.Init(repoRoot, conf); err != nil {
            return nil, err
        }

        // Manually add swarm key
        repoPath := filepath.Clean(repoRoot)
        spath := filepath.Join(repoPath, "swarm.key")

        f, err := os.Create(spath)
        if err != nil {
            return nil, err
        }
        defer f.Close()

        _, err = f.WriteString("/key/swarm/psk/1.0.0/\n/base16/\n1191aea7c9f99f679f477411d9d44f1ea0fdf5b42d995966b14a9000432f8c4a")
        if err != nil {
            return nil, err
        }
	}

	return fsrepo.Open(repoRoot)
}

func printSwarmAddrs(node *core.IpfsNode) {
	if !node.IsOnline {
		fmt.Println("Swarm not listening, running in offline mode.")
		return
	}
	var addrs []string
	for _, addr := range node.PeerHost.Addrs() {
		addrs = append(addrs, addr.String())
	}
	sort.Sort(sort.StringSlice(addrs))

	for _, addr := range addrs {
		fmt.Printf("Swarm listening on %s\n", addr)
	}
}

type Node struct {
	node *core.IpfsNode
	api coreiface.CoreAPI
	ctx context.Context
	cancel context.CancelFunc

	next_cancel_signal_id C.uint64_t
	cancel_signals map[C.uint64_t]func()
}

// Both object tables, and assorted ID trackers,
// are only ever accessed from the C thread.

var g_next_node_id uint64 = 0
var g_nodes = make(map[uint64]*Node)


//export go_asio_ipfs_allocate
func go_asio_ipfs_allocate() uint64 {
	var n Node

	n.ctx, n.cancel = context.WithCancel(context.Background())

	n.next_cancel_signal_id = 0
	n.cancel_signals = make(map[C.uint64_t]func())

	ret := g_next_node_id
	g_nodes[g_next_node_id] = &n
	g_next_node_id += 1

	return ret
}

//export go_asio_ipfs_free
func go_asio_ipfs_free(handle uint64) {
	g_nodes[handle].cancel()
	delete(g_nodes, handle)
}


//export go_asio_ipfs_cancellation_allocate
func go_asio_ipfs_cancellation_allocate(handle uint64) C.uint64_t {
	n, ok := g_nodes[handle]
	if !ok { return C.uint64_t(1<<64 - 1) } // max uint64

	ret := n.next_cancel_signal_id
	n.next_cancel_signal_id += 1
	return ret
}

//export go_asio_ipfs_cancellation_free
func go_asio_ipfs_cancellation_free(handle uint64, cancel_signal C.uint64_t) {
	n, ok := g_nodes[handle]
	if !ok { return }

	delete(n.cancel_signals, cancel_signal)
}

func withCancel(n *Node, cancel_signal C.uint64_t) (context.Context) {
	ctx, cancel := context.WithCancel(n.ctx)
	n.cancel_signals[cancel_signal] = cancel
	return ctx
}

//export go_asio_ipfs_cancel
func go_asio_ipfs_cancel(handle uint64, cancel_signal C.uint64_t) {
	n, ok := g_nodes[handle]
	if !ok { return }

	cancel, ok := n.cancel_signals[cancel_signal]
	if !ok { return }

	cancel()
}

func loadPlugins(plugins []plugin.Plugin) bool {
	for _, pl := range plugins {
		if pl, ok := pl.(plugin.PluginDatastore); ok {
			err := fsrepo.AddDatastoreConfigHandler(pl.DatastoreTypeName(), pl.DatastoreConfigParser());
			if err != nil {
				return false
			}
		} else {
			panic("Attempted to load an unsupported plugin type")
		}
	}

	return true
}

func start_node(cfg_json string, n *Node, repoRoot string) C.int {
	var c Config
	err := json.Unmarshal([]byte(cfg_json), &c)

	if err != nil {
		fmt.Println("Failed to parse config ", err);
		return C.IPFS_FAILED_TO_CREATE_REPO
	}

	err = mprome.Inject()

	if err != nil {
		fmt.Println("err");
		return C.IPFS_FAILED_TO_CREATE_REPO // FIXME
	}

    // TODO:IPFS check how plugins are started in https://github.com/ipfs/go-ipfs/blob/029d82c4ea9d73f485b30e5f5100c86502308375/cmd/ipfs/daemon.go
	pr1 := loadPlugins(flatfs.Plugins);
	pr2 := loadPlugins(levelds.Plugins);

	if !pr1 || !pr2 {
		fmt.Println("Failed to load plugins")
		return C.IPFS_FAILED_TO_CREATE_REPO
	}

	r, err := openOrCreateRepo(repoRoot, c);

	if err != nil {
		fmt.Println("err", err);
		return C.IPFS_FAILED_TO_CREATE_REPO
	}

	cfg, err := fsrepo.ConfigAt(repoRoot);

    start := time.Now()
	n.node, err = core.NewNode(n.ctx, &core.BuildCfg{
       Online: c.Online,
       Permanent: true,
       Repo:   r,
       ExtraOpts: map[string]bool{
           "ipnsps": enablePubSubIPNS,
       },
   })

   if err != nil {
       fmt.Println("err", err);
       return C.IPFS_FAILED_TO_START_NODE
   }

	elapsed := time.Since(start)
    fmt.Println("IPFS core.NewNode startup time:", elapsed)

    // TODO:IPFS do we need this?
	n.node.IsDaemon = true

	printSwarmAddrs(n.node)

	prometheus.MustRegister(&corehttp.IpfsNodeCollector{Node: n.node})

	api, err := coreapi.NewCoreAPI(n.node)
	if err != nil {
		fmt.Println("err", err);
		return C.IPFS_FAILED_TO_CREATE_REPO
	}

	// Print peers
	peers, err := api.Swarm().Peers(n.node.Context())
	if err != nil {
        fmt.Println("failed to read swarm peers:", err)
        return C.IPFS_FAILED_TO_START_NODE
    }

    // TODO:IPFS print this every several minutes
    if len(peers) == 0 {
        fmt.Println("WARNING: IPFS has no peers")
    } else {
        for _, peer := range peers {
            fmt.Printf("IPFS Peer %v %v\n", peer.ID().Pretty(), peer.Address().String())
        }
    }

	go func() {
	    cctx := oldcmds.Context {
	        ConfigRoot: repoRoot,
	        ReqLog: &oldcmds.ReqLog{},
	        ConstructNode: func() (*core.IpfsNode, error) {
                return n.node, nil
            },
            // TODO:IPFS https://github.com/ipfs/go-ipfs/blob/ef866a1400b3b2861e5e8b6cc9edc8633b890a0a/cmd/ipfs/main.go
            // LoadConfig
            // Plugins:
	    }

        opts := []corehttp.ServeOption{
            corehttp.CommandsOption(cctx),
            corehttp.MetricsScrapingOption("/debug/metrics/prometheus"),
            corehttp.LogOption(),
        }
	    // TODO:IPFS do we need this?
		apiAddr := cfg.Addresses.API[0]
		apiMAddr, err := ma.NewMultiaddr(apiAddr)

		if err != nil {
		    // IPFS:TODO fail / check serveHTTPApi in daemon.go
        	fmt.Printf("serveHTTPApi: invalid API address: %q (err: %s)", apiAddr, err)
        }

		if err = n.node.Repo.SetAPIAddr(apiMAddr); err != nil {
        	fmt.Printf("serveHTTPApi: SetAPIAddr() failed: %s\n", err)
        }

		err = corehttp.ListenAndServe(n.node, apiAddr, opts... )
		if err != nil {
			fmt.Printf("Warning: failed to start API listener on %s\n", apiAddr);
		}
	}()

	n.api = api
	return C.IPFS_SUCCESS
}

//export go_asio_ipfs_start_blocking
func go_asio_ipfs_start_blocking(handle uint64, c_cfg *C.char, c_repoPath *C.char) C.int {
	var n = g_nodes[handle]

	repoRoot := C.GoString(c_repoPath)
	cfg := C.GoString(c_cfg)

	return start_node(cfg, n, repoRoot);
}

//export go_asio_ipfs_start_async
func go_asio_ipfs_start_async(handle uint64, c_cfg *C.char, c_repoPath *C.char, fn unsafe.Pointer, fn_arg unsafe.Pointer) {
	var n = g_nodes[handle]

	repoRoot := C.GoString(c_repoPath)
	cfg := C.GoString(c_cfg)

	go func() {
		err := start_node(cfg, n, repoRoot);

		C.execute_void_cb(fn, err, fn_arg)
	}()
}


// IMPORTANT: The returned value needs to be explicitly `free`d.
//export go_asio_ipfs_node_id
func go_asio_ipfs_node_id(handle uint64) *C.char {
    //TODO:IPFS
    /*  uncommenting code below causes
        key_deprecated.go:68:22: undefined: "github.com/libp2p/go-libp2p-core/crypto".StretchedKeys
        key_deprecated.go:72:9: undefined: "github.com/libp2p/go-libp2p-core/crypto".KeyStretcher
        it is not necessary for now

	var n = g_nodes[handle]

	pid, err := peer.IDFromPrivateKey(n.node.PrivateKey)

	if err != nil {
		return nil
	}

	cstr := C.CString(pid.Pretty())
	return cstr*/
	cstr := C.CString("TODO")
	return cstr
}

//export go_asio_ipfs_resolve
func go_asio_ipfs_resolve(handle uint64, cancel_signal C.uint64_t, c_ipns_id *C.char, fn unsafe.Pointer, fn_arg unsafe.Pointer) {
	// Doesn't compile after v0.4 -> v0.10
	// Commented since is not necessary for now
	/*var n = g_nodes[handle]

	ipns_id := C.GoString(c_ipns_id)

	cancel_ctx := withCancel(n, cancel_signal)

	go func() {
		if debug {
			fmt.Println("go_asio_ipfs_resolve start");
			defer fmt.Println("go_asio_ipfs_resolve end");
		}

		n := n.node
		p := path.Path("/ipns/" + ipns_id)

		node, err := core.Resolve(cancel_ctx, n.Namesys, n.Resolver, p)

		if err != nil {
			C.execute_data_cb(fn, C.IPFS_RESOLVE_FAILED, nil, C.size_t(0), fn_arg)
			return
		}

		data := []byte(node.Cid().String())
		cdata := C.CBytes(data)
		defer C.free(cdata)

		C.execute_data_cb(fn, C.IPFS_SUCCESS, cdata, C.size_t(len(data)), fn_arg)
	}()*/
}

func publish(ctx context.Context, duration time.Duration, n *core.IpfsNode, cid string) error {
	path, err := path.ParseCidToPath(cid)

	if err != nil {
		fmt.Println("go_asio_ipfs_publish failed to parse cid \"", cid, "\"");
		return err
	}

	k := n.PrivateKey

	eol := time.Now().Add(duration)
	err  = n.Namesys.PublishWithEOL(ctx, k, path, eol)

	if err != nil {
		fmt.Println("go_asio_ipfs_publish failed");
		return err
	}

	return nil
}

//export go_asio_ipfs_publish
func go_asio_ipfs_publish(handle uint64, cancel_signal C.uint64_t, cid *C.char, seconds C.int64_t, fn unsafe.Pointer, fn_arg unsafe.Pointer) {
	var n = g_nodes[handle]

	id := C.GoString(cid)

	cancel_ctx := withCancel(n, cancel_signal)

	go func() {
		if debug {
			fmt.Println("go_asio_ipfs_publish start");
			defer fmt.Println("go_asio_ipfs_publish end");
		}

		// https://stackoverflow.com/questions/17573190/how-to-multiply-duration-by-integer
		err := publish(cancel_ctx, time.Duration(seconds) * time.Second, n.node, id);

		if err != nil {
			C.execute_void_cb(fn, C.IPFS_PUBLISH_FAILED, fn_arg)
			return
		}

		C.execute_void_cb(fn, C.IPFS_SUCCESS, fn_arg)
	}()
}

//export go_asio_ipfs_add
func go_asio_ipfs_add(handle uint64, data unsafe.Pointer, size C.size_t, only_hash bool, fn unsafe.Pointer, fn_arg unsafe.Pointer) {
	var n = g_nodes[handle]

	msg := C.GoBytes(data, C.int(size))

	go func() {
		if debug {
			fmt.Println("go_asio_ipfs_add start");
			defer fmt.Println("go_asio_ipfs_add end");
		}

		p, err := n.api.Unixfs().Add(n.node.Context(), files.NewBytesFile(msg), options.Unixfs.HashOnly(only_hash))

		if err != nil {
			fmt.Println("Error: failed to insert content ", err)
			C.execute_data_cb(fn, C.IPFS_ADD_FAILED, nil, C.size_t(0), fn_arg)
			return;
		}

		cid := p.Root()

		if err != nil {
			fmt.Println("Error: failed to parse IPFS path ", err)
			C.execute_data_cb(fn, C.IPFS_ADD_FAILED, nil, C.size_t(0), fn_arg)
			return;
		}

		cidstr := cid.String()
		cdata := C.CBytes([]byte(cidstr))
		defer C.free(cdata)

		C.execute_data_cb(fn, C.IPFS_SUCCESS, cdata, C.size_t(len(cidstr)), fn_arg)
	}()
}

//export go_asio_ipfs_cat
func go_asio_ipfs_cat(handle uint64, cancel_signal C.uint64_t, c_cid *C.char, fn unsafe.Pointer, fn_arg unsafe.Pointer) {
	var n = g_nodes[handle]

	cid := C.GoString(c_cid)

	cancel_ctx := withCancel(n, cancel_signal)

	go func() {
		if debug {
			fmt.Println("go_asio_ipfs_cat start");
			defer fmt.Println("go_asio_ipfs_cat end");
		}

		path := corepath.New(cid)
		f, err := n.api.Unixfs().Get(cancel_ctx, path)

		if err != nil {
			fmt.Printf("go_asio_ipfs_cat failed to Cat %q\n", err);
			C.execute_data_cb(fn, C.IPFS_CAT_FAILED, nil, C.size_t(0), fn_arg)
			return
		}

		var file files.File

		switch f := f.(type) {
		case files.File:
			file = f
		case files.Directory:
			fmt.Printf("go_asio_ipfs_cat path corresponds to a directory\n");
			C.execute_data_cb(fn, C.IPFS_CAT_FAILED, nil, C.size_t(0), fn_arg)
			return
		default:
			fmt.Printf("go_asio_ipfs_cat unsupported type\n");
			C.execute_data_cb(fn, C.IPFS_CAT_FAILED, nil, C.size_t(0), fn_arg)
			return
		}

		var r io.Reader = file
		bytes, err := ioutil.ReadAll(r)

		if err != nil {
			fmt.Println("go_asio_ipfs_cat failed to read");
			C.execute_data_cb(fn, C.IPFS_READ_FAILED, nil, C.size_t(0), fn_arg)
			return
		}

		cdata := C.CBytes(bytes)
		defer C.free(cdata)

		C.execute_data_cb(fn, C.IPFS_SUCCESS, cdata, C.size_t(len(bytes)), fn_arg)
	}()
}

//export go_asio_ipfs_pin
func go_asio_ipfs_pin(handle uint64, cancel_signal C.uint64_t, c_cid *C.char, fn unsafe.Pointer, fn_arg unsafe.Pointer) {
	var n = g_nodes[handle]

	cid := C.GoString(c_cid)

	cancel_ctx := withCancel(n, cancel_signal)

	go func() {
		if debug {
			fmt.Println("go_asio_ipfs_pin start");
			defer fmt.Println("go_asio_ipfs_pin end");
		}

		path := corepath.New(cid)
		err := n.api.Pin().Add(cancel_ctx, path)

		if err != nil {
			fmt.Printf("go_asio_ipfs_pin failed to pin %q %q\n", cid, err)
			C.execute_void_cb(fn, C.IPFS_PIN_FAILED, fn_arg)
			return
		}

		C.execute_void_cb(fn, C.IPFS_SUCCESS, fn_arg)
	}()
}

//export go_asio_ipfs_unpin
func go_asio_ipfs_unpin(handle uint64, cancel_signal C.uint64_t, c_cid *C.char, fn unsafe.Pointer, fn_arg unsafe.Pointer) {
	var n = g_nodes[handle]

	cid := C.GoString(c_cid)

	cancel_ctx := withCancel(n, cancel_signal)

	go func() {
		if debug {
			fmt.Println("go_asio_ipfs_unpin start");
			defer fmt.Println("go_asio_ipfs_unpin end");
		}

		path := corepath.New(cid)
		err := n.api.Pin().Rm(cancel_ctx, path)

		if err != nil {
			fmt.Printf("go_asio_ipfs_unpin failed to unpin %q %q\n", cid, err);
			C.execute_void_cb(fn, C.IPFS_UNPIN_FAILED, fn_arg)
			return
		}

		C.execute_void_cb(fn, C.IPFS_SUCCESS, fn_arg)
	}()
}

//export go_asio_ipfs_gc
func go_asio_ipfs_gc(handle uint64, cancel_signal C.uint64_t, fn unsafe.Pointer, fn_arg unsafe.Pointer) {
    var n = g_nodes[handle]
    cancel_ctx := withCancel(n, cancel_signal)

    go func() {
        gcOutChan := corerepo.GarbageCollectAsync(n.node, cancel_ctx)
        err := corerepo.CollectResult(cancel_ctx, gcOutChan, func(k cid.Cid) {})

        if err != nil {
            fmt.Printf("go_asio_ipfs_gc failed %q\n", err);
            C.execute_void_cb(fn, C.IPFS_GC_FAILED, fn_arg)
            return
        }

        C.execute_void_cb(fn, C.IPFS_SUCCESS, fn_arg)
    }()
}