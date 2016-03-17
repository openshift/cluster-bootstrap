package bootkube

import (
	"io"
	"net"
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

	src.Close()
	dst.Close()
}

func copyBytes(dst io.Writer, src io.Reader, wg *sync.WaitGroup) {
	defer wg.Done()
	if _, err := io.Copy(dst, src); err != nil {
		glog.Errorf("Tunnel i/o error: %v", err)
	}
}
