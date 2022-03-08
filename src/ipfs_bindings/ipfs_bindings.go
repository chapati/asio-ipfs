package main

import (
	"log"
	"context"
	"unsafe"
	"time"
	"io"
	"io/ioutil"
	corepath "github.com/ipfs/interface-go-ipfs-core/path"
    "github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/interface-go-ipfs-core/options"

	path "github.com/ipfs/go-path"
	files "github.com/ipfs/go-ipfs-files"
)

// #cgo CFLAGS: -DIN_GO=1 -ggdb -I ${SRCDIR}/../../include
// #cgo android LDFLAGS: -Wl,--unresolved-symbols=ignore-all
//#include <stdlib.h>
//#include <stddef.h>
//#include <stdint.h>
//#include <ipfs_error_codes.h>
//
//// Don't export these functions into C or we'll get "unused function" warnings
//// (Or errors saying functions are defined more than once if the're not static).
//
//#if IN_GO
//static void execute_void_cb(void* func, int32_t err, void* arg)
//{
//    ((void(*)(int, void*)) func)(err, arg);
//}
//static void execute_data_cb(void* func, int32_t err, void* data, size_t size, void* arg)
//{
//    ((void(*)(uint32_t, char*, size_t, void*)) func)(err, data, size, arg);
//}
//static void execute_state_cb(void* func, void* arg, const char* err, uint32_t peercnt)
//{
//    ((void(*)(void*, const char*, uint32_t)) func)(arg, err, peercnt);
//}
//#endif // if IN_GO
import "C"

func main() {
}

func executeStateCB(n* Node, err error, peercnt uint32) {
    var cstr *C.char

    if err != nil {
        cstr := C.CString(err.Error())
        defer C.free(unsafe.Pointer(cstr))
    }

    C.execute_state_cb(n.state_cb, n.state_cb_arg, cstr, C.uint32_t(peercnt))
    return
}

func executeVoidCB(fn unsafe.Pointer, code C.int32_t, fn_arg unsafe.Pointer) {
    C.execute_void_cb(fn, code, fn_arg)
}

//export go_asio_memfree
func go_asio_memfree(what unsafe.Pointer) {
	C.free(what)
}

//export go_asio_ipfs_resolve
func go_asio_ipfs_resolve(cancel_signal C.uint64_t, c_ipns_id *C.char, fn unsafe.Pointer, fn_arg unsafe.Pointer) {
    // TODO:IPNS implement
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
		log.Println("go_asio_ipfs_publish failed to parse cid \"", cid, "\"");
		return err
	}

	k := n.PrivateKey
	eol := time.Now().Add(duration)
	err  = n.Namesys.PublishWithEOL(ctx, k, path, eol)

	if err != nil {
		log.Println("go_asio_ipfs_publish PublishWithEOL failed: ", err);
		return err
	}

	return nil
}

//export go_asio_ipfs_publish
func go_asio_ipfs_publish(cancel_signal C.uint64_t, cid *C.char, seconds C.int64_t, fn unsafe.Pointer, fn_arg unsafe.Pointer) {
	var n = g_node
    if n == nil {
        go func () {
            C.execute_void_cb(fn, C.IPFS_NO_NODE, fn_arg)
        } ()
        return
    }

	id := C.GoString(cid)
	cancel_ctx := withCancel(n, cancel_signal)

	go func() {
		err := publish(cancel_ctx, time.Duration(seconds) * time.Second, n.node, id);
		if err != nil {
			C.execute_void_cb(fn, C.IPFS_PUBLISH_FAILED, fn_arg)
			return
		}
		C.execute_void_cb(fn, C.IPFS_SUCCESS, fn_arg)
	}()
}

//export go_asio_ipfs_calc_cid
func go_asio_ipfs_calc_cid(data unsafe.Pointer, size C.size_t, fn unsafe.Pointer, fn_arg unsafe.Pointer) {
	var n = g_node
	if n == nil {
	    go func () {
	        C.execute_data_cb(fn, C.IPFS_NO_NODE, nil, C.size_t(0), fn_arg)
	    } ()
	    return
	}

	msg := C.GoBytes(data, C.int(size))
	go func() {
        p, err := n.api.Unixfs().Add(n.node.Context(), files.NewBytesFile(msg), options.Unixfs.HashOnly(true))
        if err != nil {
            log.Println("Error: failed to calculate cid ", err)
            C.execute_data_cb(fn, C.IPFS_CALC_CID_FAILED, nil, C.size_t(0), fn_arg)
            return;
        }

		cid := p.Root()
		if err != nil {
			log.Println("Error: failed to parse IPFS path ", err)
			C.execute_data_cb(fn, C.IPFS_CALC_CID_FAILED, nil, C.size_t(0), fn_arg)
			return;
		}

		cidstr := cid.String()
		cdata := C.CBytes([]byte(cidstr))
		defer C.free(cdata)

		C.execute_data_cb(fn, C.IPFS_SUCCESS, cdata, C.size_t(len(cidstr)), fn_arg)
	}()
}

//export go_asio_ipfs_add
func go_asio_ipfs_add(data unsafe.Pointer, size C.size_t, pin bool, fn unsafe.Pointer, fn_arg unsafe.Pointer) {
	var n = g_node
	if n == nil {
	    go func () {
	        C.execute_data_cb(fn, C.IPFS_NO_NODE, nil, C.size_t(0), fn_arg)
	    } ()
	    return
	}

	msg := C.GoBytes(data, C.int(size))
	go func() {
        p, err := n.api.Unixfs().Add(n.node.Context(), files.NewBytesFile(msg), options.Unixfs.Pin(pin))
        if err != nil {
            log.Println("Error: failed to insert content ", err)
            C.execute_data_cb(fn, C.IPFS_ADD_FAILED, nil, C.size_t(0), fn_arg)
            return;
        }

		cid := p.Root()
		if err != nil {
			log.Println("Error: failed to parse IPFS path ", err)
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
func go_asio_ipfs_cat(cancel_signal C.uint64_t, c_cid *C.char, fn unsafe.Pointer, fn_arg unsafe.Pointer) {
	var n = g_node
    if n == nil {
        go func () {
            C.execute_data_cb(fn, C.IPFS_NO_NODE, nil, C.size_t(0), fn_arg)
        } ()
        return
    }

	cid := C.GoString(c_cid)
	cancel_ctx := withCancel(n, cancel_signal)

	go func() {
		path := corepath.New(cid)
		f, err := n.api.Unixfs().Get(cancel_ctx, path)

		if err != nil {
			log.Printf("go_asio_ipfs_cat failed to Cat %q\n", err);
			C.execute_data_cb(fn, C.IPFS_CAT_FAILED, nil, C.size_t(0), fn_arg)
			return
		}

		var file files.File

		switch f := f.(type) {
		case files.File:
			file = f
		case files.Directory:
			log.Printf("go_asio_ipfs_cat path corresponds to a directory\n");
			C.execute_data_cb(fn, C.IPFS_CAT_FAILED, nil, C.size_t(0), fn_arg)
			return
		default:
			log.Printf("go_asio_ipfs_cat unsupported type\n");
			C.execute_data_cb(fn, C.IPFS_CAT_FAILED, nil, C.size_t(0), fn_arg)
			return
		}

		var r io.Reader = file
		bytes, err := ioutil.ReadAll(r)

		if err != nil {
			log.Println("go_asio_ipfs_cat failed to read");
			C.execute_data_cb(fn, C.IPFS_READ_FAILED, nil, C.size_t(0), fn_arg)
			return
		}

		cdata := C.CBytes(bytes)
		defer C.free(cdata)

		C.execute_data_cb(fn, C.IPFS_SUCCESS, cdata, C.size_t(len(bytes)), fn_arg)
	}()
}

//export go_asio_ipfs_pin
func go_asio_ipfs_pin(cancel_signal C.uint64_t, c_cid *C.char, fn unsafe.Pointer, fn_arg unsafe.Pointer) {
	var n = g_node
    if n == nil {
        go func () {
            C.execute_void_cb(fn, C.IPFS_NO_NODE, fn_arg)
        } ()
        return
    }

	cid := C.GoString(c_cid)
	cancel_ctx := withCancel(n, cancel_signal)

	go func() {
		path := corepath.New(cid)
		err := n.api.Pin().Add(cancel_ctx, path)

		if err != nil {
			log.Printf("go_asio_ipfs_pin failed to pin %q %q\n", cid, err)
			C.execute_void_cb(fn, C.IPFS_PIN_FAILED, fn_arg)
			return
		}

		C.execute_void_cb(fn, C.IPFS_SUCCESS, fn_arg)
	}()
}

//export go_asio_ipfs_unpin
func go_asio_ipfs_unpin(cancel_signal C.uint64_t, c_cid *C.char, fn unsafe.Pointer, fn_arg unsafe.Pointer) {
	var n = g_node
    if n == nil {
        go func () {
            C.execute_void_cb(fn, C.IPFS_NO_NODE, fn_arg)
        } ()
        return
    }

	cid := C.GoString(c_cid)
	cancel_ctx := withCancel(n, cancel_signal)

	go func() {
		path := corepath.New(cid)
		err := n.api.Pin().Rm(cancel_ctx, path)

		if err != nil {
			log.Printf("go_asio_ipfs_unpin failed to unpin %q %q\n", cid, err);
			C.execute_void_cb(fn, C.IPFS_UNPIN_FAILED, fn_arg)
			return
		}

		C.execute_void_cb(fn, C.IPFS_SUCCESS, fn_arg)
	}()
}
