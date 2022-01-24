package main

import (
    "log"
    "unsafe"
    corerepo "github.com/ipfs/go-ipfs/core/corerepo"
    cid "github.com/ipfs/go-cid"
)

//#include <stdlib.h>
//#include <stddef.h>
//#include <stdint.h>
//#include <ipfs_error_codes.h>
import "C"

//export go_asio_ipfs_gc
func go_asio_ipfs_gc(cancel_signal C.uint64_t, fn unsafe.Pointer, fn_arg unsafe.Pointer) {
    var n = g_node
    if n == nil {
        go func () {
            executeVoidCB(fn, C.IPFS_NO_NODE, fn_arg)
        } ()
        return
    }

    cancel_ctx := withCancel(n, cancel_signal)
    go func() {
        gcOutChan := corerepo.GarbageCollectAsync(n.node, cancel_ctx)
        err := corerepo.CollectResult(cancel_ctx, gcOutChan, func(k cid.Cid) {})

        if err != nil {
            log.Printf("go_asio_ipfs_gc failed %q\n", err);
            executeVoidCB(fn, C.IPFS_GC_FAILED, fn_arg)
            return
        }

        executeVoidCB(fn, C.IPFS_SUCCESS, fn_arg)
    }()
}

func maybeRunGC(n *Node, acfg *AsioConfig) (<-chan error, error) {
	if !acfg.RunGC {
	    log.Println("IPFS gc is not enabled and will be not started")
		return nil, nil
	}

	errc := make(chan error)
	go func() {
	    log.Println("IPFS GC launched")
		errc <- corerepo.PeriodicGC(n.ctx, n.node)
		close(errc)
		log.Println("IPFS GC thread is closed")
	}()

	return errc, nil
}
