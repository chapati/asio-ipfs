package main

import(
    "C"
    "os"
    "log"
    "unsafe"
)

type CustomLog struct {
    log_cb unsafe.Pointer
}

func (l CustomLog) Write(data []byte) (n int, err error) {
    if l.log_cb == nil {
        panic("CustomLog.Write called while logcb is nil")
    }
    executeLogCB(l.log_cb, data)
    return len(data), nil
}

//export go_asio_ipfs_redirect_logs
func go_asio_ipfs_redirect_logs(log_cb unsafe.Pointer) {
    if log_cb == nil {
        log.SetFlags(log.Flags() | log.Ldate | log.Ltime)
        log.SetOutput(os.Stderr)
    }
    log.SetFlags(log.Flags() &^ (log.Ldate | log.Ltime))
    log.SetOutput(&CustomLog{
      log_cb,
    })
}
