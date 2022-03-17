package proxy

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/basemachina/bridge/internal/testlogr"
)

func MustParseRequestURI(rawURL string) *url.URL {
	u, err := url.ParseRequestURI(rawURL)
	if err != nil {
		panic(err)
	}
	return u
}

func TestProxy_validateAndGetTarget(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		target  *url.URL
		req     *http.Request
		wantErr bool
	}{
		{
			name:    "invalid POST method",
			target:  MustParseRequestURI("tcp://127.0.0.1:80"),
			req:     httptest.NewRequest("POST", "/", nil),
			wantErr: true,
		},
		{
			name:   "invalid Sec-Websocket-Key",
			target: MustParseRequestURI("tcp://127.0.0.1:80"),
			req: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				req.Header.Set(secWebSocketKey, "") // empty
				return req
			}(),
			wantErr: true,
		},
		{
			name:   "invalid Scheme",
			target: MustParseRequestURI("http://127.0.0.1:80"),
			req: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				req.Header.Set(secWebSocketKey, "hello") // empty
				return req
			}(),
			wantErr: true,
		},
		{
			name:   "valid",
			target: MustParseRequestURI("tcp://127.0.0.1:80"),
			req: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				req.Header.Set(secWebSocketKey, "hello") // empty
				return req
			}(),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if err := validateAndGetTarget(tt.req, tt.target); (err != nil) != tt.wantErr {
				if tt.wantErr && !errors.Is(err, ErrBadRequest) {
					t.Errorf("unexpected http status code: %v", err)
				}
				t.Errorf("Proxy.validateAndGetTarget() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTCPProxy_proxy(t *testing.T) {
	echoListener := newEchoListener()
	t.Cleanup(func() { echoListener.Close() })

	h := NewProxy(testlogr.Logger)
	testServer := httptest.NewServer(h)
	t.Cleanup(testServer.Close) // Listener も close してくれる

	t.Run("check echo over HTTP", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequest(http.MethodGet, testServer.URL, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set(TargetURLHeaderKey, "tcp://"+echoListener.Addr().String())
		nonce := generateNonce()
		req.Header.Set(secWebSocketKey, nonce)

		addr := strings.TrimPrefix(testServer.URL, "http://")
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()

		if err := req.Write(conn); err != nil {
			t.Fatal(err)
		}

		resp, err := http.ReadResponse(bufio.NewReader(conn), req)
		if err != nil {
			t.Fatal(err)
		}

		// check resp.Body is NoBody
		// We can call resp.Body.Close if NoBody
		if resp.Body != http.NoBody {
			t.Fatalf("unexpected response: %+v", resp.Body)
		}

		if resp.StatusCode != http.StatusSwitchingProtocols {
			t.Fatalf("want 101, but got %d", resp.StatusCode)
		}

		connection := strings.ToLower(resp.Header.Get("Connection"))
		if connection != "upgrade" {
			t.Fatalf("want 'upgrade', but got %q", connection)
		}
		upgrade := resp.Header.Get("Upgrade")
		if upgrade != "websocket" {
			t.Fatalf("want 'websocket', but got %q", upgrade)
		}
		expectedAccept := getNonceAccept(nonce)
		if got := resp.Header.Get(secWebSocketAcceptKey); got != expectedAccept {
			t.Fatalf("want %q, but got %q", expectedAccept, got)
		}

		want := "hello, world"
		testEcho(t, conn, want)
	})

	t.Run("check echo over HTTP using own dialer", func(t *testing.T) {
		t.Parallel()

		u, err := url.Parse(testServer.URL)
		if err != nil {
			t.Fatal(err)
		}

		dialer := &Dialer{
			BridgeURL: u,
			Tls:       false,
			BaseDialContext: (&net.Dialer{
				Timeout: 3 * time.Second,
			}).DialContext,
		}

		cases := []struct {
			name string
			run  func(t *testing.T, conn net.Conn)
		}{
			{
				name: "check echo",
				run: func(t *testing.T, conn net.Conn) {
					want := "hello, world bridge client"
					testEcho(t, conn, want)
				},
			},
			{
				name: "check EOF",
				run:  testQuit,
			},
		}
		for _, tc := range cases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				ctx := context.Background()
				ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
				defer cancel()

				conn, err := dialer.DialContext(ctx, echoListener.Addr().String())
				if err != nil {
					t.Fatal(err)
				}
				defer conn.Close()
				tc.run(t, conn)
			})
		}
	})

	t.Run("check echo over HTTP using own dialer (tls)", func(t *testing.T) {
		t.Parallel()

		// create a new listner for tls
		h := NewProxy(testlogr.Logger)
		tlsTestServer := httptest.NewTLSServer(h)
		defer tlsTestServer.Close()

		u, err := url.Parse(tlsTestServer.URL)
		if err != nil {
			t.Fatal(err)
		}

		dialer := &Dialer{
			BridgeURL: u,
			Tls:       true,
			BaseDialContext: (&net.Dialer{
				Timeout: 3 * time.Second,
			}).DialContext,
		}

		cases := []struct {
			name string
			run  func(t *testing.T, conn net.Conn)
		}{
			{
				name: "check echo",
				run: func(t *testing.T, conn net.Conn) {
					want := "hello, world bridge tls client"
					testEcho(t, conn, want)
				},
			},
			{
				name: "check EOF",
				run:  testQuit,
			},
		}
		for _, tc := range cases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				ctx := context.Background()
				ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
				defer cancel()

				conn, err := dialer.DialContext(ctx, echoListener.Addr().String())
				if err != nil {
					t.Fatal(err)
				}
				defer conn.Close()
				tc.run(t, conn)
			})
		}
	})
}

func newEchoListener() net.Listener {
	echoListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}

	go func() {
		for {
			conn, err := echoListener.Accept()
			if errors.Is(err, net.ErrClosed) {
				return
			}
			if err != nil {
				panic(err)
			}
			go func() {
				defer conn.Close()
				if err := serverEcho(conn); err != nil && err != io.EOF {
					log.Printf("server Echo error: %+v\n", err)
				}
			}()
		}
	}()
	return echoListener
}

func testEcho(t *testing.T, conn net.Conn, want string) {
	t.Helper()
	conn.SetDeadline(time.Now().Add(3 * time.Second))

	length := byte(len(want))
	writeBuf := append([]byte{length}, []byte(want)...)
	if _, err := conn.Write(writeBuf); err != nil {
		t.Fatal(err)
	}
	result := make([]byte, len(want))
	n, err := io.ReadAtLeast(conn, result, len(want))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(result[:n]); want != got {
		t.Fatalf("want %q, but got %q", want, got)
	}
}

func testQuit(t *testing.T, conn net.Conn) {
	t.Helper()
	conn.SetDeadline(time.Now().Add(3 * time.Second))

	length := byte(len(quit))
	writeBuf := append([]byte{length}, []byte(quit)...)
	if _, err := conn.Write(writeBuf); err != nil {
		t.Fatal(err)
	}

	_, err := conn.Read(make([]byte, 1))
	if err != io.EOF {
		t.Fatalf("want error %q, but got %q when server connection is closed", io.EOF, err)
	}
}

const quit = "q"

func serverEcho(conn net.Conn) error {
	sizeBuf := make([]byte, 1)
	_, err := io.ReadAtLeast(conn, sizeBuf, 1) // read text length
	if err != nil {
		return err
	}

	size := int(sizeBuf[0])

	textBuf := make([]byte, size)
	n, err := io.ReadAtLeast(conn, textBuf, size) // read text message
	if err != nil {
		return err
	}

	msg := string(textBuf[:n])
	if msg == quit {
		return conn.Close()
	}

	_, err = conn.Write(textBuf[:n])
	return err
}
