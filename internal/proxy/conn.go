package proxy

import (
	"bufio"
	"io"
	"net"
	"time"
)

type bufConn struct {
	rawConn net.Conn
	reader  *bufio.Reader
}

var _ interface {
	net.Conn
	closeReader
	closeWriter
} = (*bufConn)(nil)

func (c *bufConn) CloseWrite() error {
	if v, ok := c.rawConn.(closeWriter); ok {
		return v.CloseWrite()
	}
	return nil
}

func (c *bufConn) CloseRead() error {
	if v, ok := c.rawConn.(closeReader); ok {
		return v.CloseRead()
	}
	return nil
}

func (c *bufConn) Read(b []byte) (int, error) {
	if c.reader.Buffered() > 0 {
		// bufio.Reader は byte slice を buffer として持つ。
		// 読み取られてない buffer が存在する時、buffer から Read メソッドに渡ってきた byte slice へ
		// 残っている分をコピーする。
		// この時 bufio.Reader が内部でもつラップされた Reader を使わない。
		return io.MultiReader(c.reader, c.rawConn).Read(b)
	}
	return c.rawConn.Read(b)
}
func (c *bufConn) Write(b []byte) (int, error) {
	return c.rawConn.Write(b)
}

func (c *bufConn) Close() error {
	return c.rawConn.Close()
}

func (c *bufConn) LocalAddr() net.Addr                { return c.rawConn.LocalAddr() }
func (c *bufConn) RemoteAddr() net.Addr               { return c.rawConn.RemoteAddr() }
func (c *bufConn) SetDeadline(t time.Time) error      { return c.rawConn.SetDeadline(t) }
func (c *bufConn) SetReadDeadline(t time.Time) error  { return c.rawConn.SetReadDeadline(t) }
func (c *bufConn) SetWriteDeadline(t time.Time) error { return c.rawConn.SetWriteDeadline(t) }
