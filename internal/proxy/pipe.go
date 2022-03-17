package proxy

import (
	"io"
	"net"

	"golang.org/x/sync/errgroup"
)

func tcpPipe(c1, c2 net.Conn) error {
	var eg errgroup.Group

	eg.Go(func() error {
		defer closeHalfConn(c1, c2)
		io.Copy(c1, c2)
		return nil
	})

	eg.Go(func() error {
		defer closeHalfConn(c2, c1)
		io.Copy(c2, c1)
		return nil
	})

	return eg.Wait()
}

type (
	closeWriter interface {
		CloseWrite() error
	}
	closeReader interface {
		CloseRead() error
	}
)

var _ interface {
	closeWriter
	closeReader
} = (*net.TCPConn)(nil)

func closeHalfConn(dst, src net.Conn) {
	if v, ok := dst.(closeWriter); ok {
		v.CloseWrite()
	}
	if v, ok := src.(closeReader); ok {
		v.CloseRead()
	}
}
