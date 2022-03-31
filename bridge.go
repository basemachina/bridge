package bridge

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/basemachina/bridge/bridgehttp"
	"github.com/basemachina/bridge/internal/auth"
	"github.com/basemachina/bridge/internal/ctxtime"
	"github.com/basemachina/bridge/internal/proxy"
	"github.com/go-logr/logr"
)

const (
	OKPath    = "/ok"
	OKMessage = "bridge is ready"
	ProxyPath = "/htproxy"
)

// Env stores configuration settings extract from enviromental variables
// The practice getting from environmental variables comes from https://12factor.net.
type Env struct {
	// Port is port to listen HTTP server. Default is 8080.
	// This value should be other than 4321. because "127.0.0.1:4321" is used in connection checking.
	Port string `envconfig:"PORT" default:"8080" description:"bridge を HTTP としてサーブするために利用します。4321 以外を指定してください。"`

	// LogLevel is INFO or DEBUG. Default is "INFO".
	LogLevel string `envconfig:"LOG_LEVEL" default:"INFO"`

	// APIURL is an url of basemachina.
	APIURL string `envconfig:"BASEMACHINA_API_URL" default:"https://api.basemachina.com"`

	// FetchInterval is interval to fetch
	FetchInterval time.Duration `envconfig:"FETCH_INTERVAL" default:"1h" description:"認可処理に利用する公開鍵を更新する間隔です。"`

	// FetchTimeout is timeout to fetch
	FetchTimeout time.Duration `envconfig:"FETCH_TIMEOUT" default:"10s" description:"認可処理に利用する公開鍵を更新するタイムアウトです。"`

	// TenantID is ID of tenant
	TenantID string `envconfig:"TENANT_ID" default:"" description:"認可処理に利用します。設定されると指定されたテナント ID 以外からのリクエストを拒否します。"`
}

// HTTPHandlerConfig is a config to setup bridge http handler.
type HTTPHandlerConfig struct {
	Logger             logr.Logger
	PublicKeyGetter    auth.PublicKeyGetter
	RegisterUserObject auth.TenantIDGetter
	TenantID           string
	Middlewares        []bridgehttp.Middleware
}

// NewHTTPHandler is a handler for handling any requests.
func NewHTTPHandler(c *HTTPHandlerConfig) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(OKPath, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Write([]byte(OKMessage))
	})
	middlewares := append(c.Middlewares,
		ctxtime.Middleware(),
		auth.Middleware(&auth.MiddlewareConfig{
			TenantID:           c.TenantID,
			Logger:             c.Logger.WithName("auth"),
			PublicKeyGetter:    c.PublicKeyGetter,
			RegisterUserObject: c.RegisterUserObject,
		}),
	)
	mux.Handle(ProxyPath, bridgehttp.UseMiddlewares(
		proxy.NewProxy(c.Logger.WithName("proxy")),
		middlewares...,
	))
	return mux
}

// ConnectionCheckPort is used in connection check from API.
const ConnectionCheckPort = "4321"

func NewHTTPServer(envPort string, handler http.Handler) (*http.Server, func(), error) {
	if envPort == ConnectionCheckPort {
		return nil, nil, fmt.Errorf("PORT env should be other than 4321")
	}
	srv := &http.Server{
		Addr:    ":" + envPort,
		Handler: handler,
	}

	ServeCheckConnectionServer()

	return srv, func() {
		ctx, cancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer cancel()
		srv.Shutdown(ctx)
	}, nil
}

// ServeCheckConnectionServer serves http server
// that is used in connection check from API.
//
// This server is no required to handle graceful
// because used only the connection check from API.
//
// Serve with goroutine.
func ServeCheckConnectionServer() {
	go func() {
		err := http.ListenAndServe(
			":"+ConnectionCheckPort,
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(OKMessage))
			}),
		)
		panic(fmt.Errorf("failed to serve a server to check connection from API: %w", err))
	}()
}
