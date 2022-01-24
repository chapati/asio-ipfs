package main

import(
    "log"
    "fmt"
    "net"
    "sync"
    "net/http"
    "github.com/ipfs/go-ipfs/core"
    "github.com/ipfs/go-ipfs/core/corehttp"
    ma "github.com/multiformats/go-multiaddr"
    manet "github.com/multiformats/go-multiaddr/net"
)

func defaultMux(path string) corehttp.ServeOption {
	return func(node *core.IpfsNode, _ net.Listener, mux *http.ServeMux) (*http.ServeMux, error) {
		mux.Handle(path, http.DefaultServeMux)
		return mux, nil
	}
}

func maybeServeHTTPApi(n *Node, acfg *AsioConfig) (<-chan error, error) {
	if acfg.APIPort == 0 {
	    log.Println("IPFS API is disabled")
		return nil, nil
	}

	cfg, err := n.cctx.GetConfig()
    if err != nil {
        return nil, fmt.Errorf("maybeServeHTTPApi: GetConfig() failed: %s", err)
    }

    apiAddrs := cfg.Addresses.API
    var listeners []manet.Listener

    for _, addr := range apiAddrs {
        apiMaddr, err := ma.NewMultiaddr(addr)
        if err != nil {
            return nil, fmt.Errorf("maybeServeHTTPApi: invalid API address: %q (err: %s)", addr, err)
        }
        apiLis, err := manet.Listen(apiMaddr)
        if err != nil {
            return nil, fmt.Errorf("serveHTTPApi: manet.Listen(%s) failed: %s", apiMaddr, err)
        }
        listeners = append(listeners, apiLis)
    }

    for _, listener := range listeners {
        log.Printf("IPFS API server listening on %s\n", listener.Multiaddr())
        switch listener.Addr().Network() {
        case "tcp", "tcp4", "tcp6":
            log.Printf("IPFS WebUI is on http://%s/webui\n", listener.Addr())
        }
    }

    // by default, we don't let you load arbitrary ipfs objects through the api,
    // only the webui objects are allowed.
    gatewayOpt := corehttp.GatewayOption(false, corehttp.WebUIPaths...)
    var opts = []corehttp.ServeOption{
        corehttp.MetricsCollectionOption("api"),
        corehttp.MetricsOpenCensusCollectionOption(),
        corehttp.CheckVersionOption(),
        corehttp.CommandsOption(*n.cctx),
        corehttp.WebUIOption,
        gatewayOpt,
        corehttp.VersionOption(),
        defaultMux("/debug/vars"),
        defaultMux("/debug/pprof/"),
        corehttp.MutexFractionOption("/debug/pprof-mutex/"),
        corehttp.MetricsScrapingOption("/debug/metrics/prometheus"),
        corehttp.LogOption(),
    }

    if len(cfg.Gateway.RootRedirect) > 0 {
        opts = append(opts, corehttp.RedirectOption("", cfg.Gateway.RootRedirect))
    }

    if err := n.node.Repo.SetAPIAddr(listeners[0].Multiaddr()); err != nil {
        return nil, fmt.Errorf("serveHTTPApi: SetAPIAddr() failed: %s", err)
    }

    errc := make(chan error)
    var wg sync.WaitGroup
    for _, apiLis := range listeners {
        wg.Add(1)
        go func(lis manet.Listener) {
            defer wg.Done()
            errc <- corehttp.Serve(n.node, manet.NetListener(lis), opts...)
        }(apiLis)
    }

    go func() {
        wg.Wait()
        close(errc)
    }()

    return errc, nil
}

func maybeServeHTTPGateway(n *Node, acfg *AsioConfig) (<-chan error, error) {
    if acfg.GatewayPort == 0 {
        log.Println("IPFS Gateway is disabled")
        return nil, nil
    }

	cfg, err := n.cctx.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("maybeServeHTTPGateway: GetConfig() failed: %s", err)
	}

	gatewayAddrs := cfg.Addresses.Gateway
	var listeners []manet.Listener

	for _, addr := range gatewayAddrs {
		gatewayMaddr, err := ma.NewMultiaddr(addr)
		if err != nil {
			return nil, fmt.Errorf("maybeServeHTTPGateway: invalid gateway address: %q (err: %s)", addr, err)
		}
		gwLis, err := manet.Listen(gatewayMaddr)
		if err != nil {
			return nil, fmt.Errorf("maybeServeHTTPGateway: manet.Listen(%s) failed: %s", gatewayMaddr, err)
		}
		listeners = append(listeners, gwLis)
	}

	gwType := "readonly"
	writable := false
	if cfg.Gateway.Writable {
		gwType = "writable"
		writable = true
	}

	for _, listener := range listeners {
		log.Printf("IPFS Gateway (%s) server listening on %s\n", gwType, listener.Multiaddr())
	}

	cmdctx := *n.cctx
	cmdctx.Gateway = true

	var opts = []corehttp.ServeOption{
		corehttp.MetricsCollectionOption("gateway"),
		corehttp.HostnameOption(),
		corehttp.GatewayOption(writable, "/ipfs", "/ipns"),
		corehttp.VersionOption(),
		corehttp.CheckVersionOption(),
		corehttp.CommandsROOption(cmdctx),
	}

	if cfg.Experimental.P2pHttpProxy {
		opts = append(opts, corehttp.P2PProxyOption())
	}

	if len(cfg.Gateway.RootRedirect) > 0 {
		opts = append(opts, corehttp.RedirectOption("", cfg.Gateway.RootRedirect))
	}

	node, err := n.cctx.ConstructNode()
	if err != nil {
		return nil, fmt.Errorf("maybeServeHTTPGateway: ConstructNode() failed: %s", err)
	}

	errc := make(chan error)
	var wg sync.WaitGroup
	for _, lis := range listeners {
		wg.Add(1)
		go func(lis manet.Listener) {
			defer wg.Done()
			errc <- corehttp.Serve(node, manet.NetListener(lis), opts...)
		}(lis)
	}

	go func() {
		wg.Wait()
		close(errc)
	}()

	return errc, nil
}
