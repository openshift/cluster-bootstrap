package bootkube

import (
	"io"
	"net"
	"strings"
	"sync"

	"github.com/golang/glog"
)

type Proxy struct {
	listenAddr string
	listenFunc func(string, string) (net.Listener, error)
	dialAddr   string
	dialFunc   func(string, string) (net.Conn, error)
}

func (p *Proxy) Run() error {
	lConn, err := p.listenFunc("tcp", p.listenAddr)
	if err != nil {
		return err
	}
	for {
		src, err := lConn.Accept()
		if err != nil {
			glog.Errorf("Error accepting on %s: %v", p.listenAddr, err)
			continue
		}

		dst, err := p.dialFunc("tcp", p.dialAddr)
		if err != nil {
			glog.Errorf("Error dialing %s: %v", p.dialAddr, err)
			src.Close()
			continue
		}
		go proxy(src, dst)
	}
}

func proxy(src, dst net.Conn) {
	var wg sync.WaitGroup

	wg.Add(2)
	go copyBytes(src, dst, &wg)
	go copyBytes(dst, src, &wg)
	wg.Wait()
}

func copyBytes(dst, src net.Conn, wg *sync.WaitGroup) {
	defer wg.Done()
	if _, err := io.Copy(dst, src); err != nil {
		if !isClosedErr(err) {
			glog.Errorf("Tunnel i/o error: %v", err)
		}
	}
	dst.Close()
	src.Close()
}

func isClosedErr(err error) bool {
	return strings.HasSuffix(err.Error(), "use of closed network connection")
}
