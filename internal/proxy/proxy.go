package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/go-logr/logr"
)

const (
	// TargetURLHeaderKey is header key to specify target URL
	TargetURLHeaderKey = "X-Bridge-Target-URL"

	// https://httpstatuses.com/499
	httpStatusClientClosedRequest = 499
)

func NewProxy(logger logr.Logger) *Proxy {
	httpLogger := logger.WithName("http")
	return &Proxy{
		logger:   logger,
		tcpProxy: NewTCPProxy(logger.WithName("tcp")),
		httpProxy: &httputil.ReverseProxy{
			Director: func(*http.Request) {},
			ErrorHandler: func(w http.ResponseWriter, req *http.Request, err error) {
				// If the client is closed the connection, this proxy will respond
				// 499 HTTP status.
				//
				// This is an great article, but Japanese.
				// https://songmu.jp/riji/entry/2020-12-16-go-http499.html
				ctx := req.Context()
				select {
				case <-ctx.Done():
					w.WriteHeader(httpStatusClientClosedRequest)
					return
				default:
				}

				httpLogger.Error(err, "unhandled error")
				w.WriteHeader(http.StatusBadGateway)
			},
		},
	}
}

// Proxy represents a bridge which proxies the connection
// between basemachina API and any data sources in tenants.
type Proxy struct {
	logger    logr.Logger
	httpProxy *httputil.ReverseProxy
	tcpProxy  *TCPProxy
}

func (p *Proxy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	targetURL := req.Header.Get(TargetURLHeaderKey)
	target, err := url.ParseRequestURI(targetURL)
	if err != nil {
		p.logger.Error(err,
			"unexpected target url format",
			"target", targetURL,
		)
		rw.WriteHeader(http.StatusBadGateway)
		return
	}

	// forwards tcp over HTTP
	if req.Method == http.MethodGet &&
		// forwards to tcp over HTTP if target URL schema is "tcp://"
		target.Scheme == TCPScheme {
		p.tcpProxy.ServeWebSocket(rw, req, target)
		return
	}

	// because also forward this
	req.Header.Del(TargetURLHeaderKey)

	// swap to target URL
	outreq := req.Clone(ctx)
	outreq.URL = target
	outreq.Host = target.Host
	outreq.RequestURI = target.Path

	if _, ok := req.Header["User-Agent"]; !ok {
		// explicitly disable User-Agent so it's not set to default value
		req.Header.Set("User-Agent", "")
	}

	p.httpProxy.ServeHTTP(rw, outreq)
}
