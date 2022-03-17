package proxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
)

type Dialer struct {
	BridgeURL       *url.URL
	Tls             bool
	BaseDialContext DialContextFunc
}

func attachRequestHeaders(req *http.Request, addr string) (nonce string) {
	// from bridge server to tcp server
	// bridge <--> tcp server
	req.Header.Set(TargetURLHeaderKey, "tcp://"+addr)

	// To connect to google cloud run as bi-directional streaming,
	// we have to use websocket upgrade header.
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")

	nonce = generateNonce()
	req.Header.Set(secWebSocketKey, nonce)
	return
}

func (d *Dialer) DialContext(ctx context.Context, addr string) (conn net.Conn, err error) {
	// Create a request message to connect bridge server.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.BridgeURL.String(), nil)
	if err != nil {
		return nil, err
	}

	nonce := attachRequestHeaders(req, addr)

	// to bridge HTTP server
	// api <--> bridge
	conn, err = d.BaseDialContext(ctx, "tcp", d.BridgeURL.Host)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			conn.Close() // prevent a leak
			conn = nil
			return
		}
	}()

	// swap plain connection with tls connection if tls is enabled
	if d.Tls {
		tlsConn := tls.Client(conn, &tls.Config{
			ServerName:         d.BridgeURL.Hostname(),
			InsecureSkipVerify: true,
		})
		// tls handshake
		if err = tlsConn.HandshakeContext(ctx); err != nil {
			return
		}
		conn = tlsConn
	}

	if err = req.Write(conn); err != nil {
		return
	}

	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, req)
	if err != nil {
		return
	}

	conn = &bufConn{
		rawConn: conn,
		reader:  br,
	}

	if resp.StatusCode != http.StatusSwitchingProtocols {
		err = fmt.Errorf("unexpected status: %s", resp.Status)
		return
	}

	expectedAccept := getNonceAccept(nonce)
	if resp.Header.Get(secWebSocketAcceptKey) != expectedAccept {
		err = errors.New("unexpected challenge response")
		return
	}

	return
}
