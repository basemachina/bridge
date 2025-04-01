package auth

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/basemachina/bridge/bridgehttp"
	"github.com/basemachina/bridge/internal/ctxtime"
	"github.com/go-logr/logr"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jws"
	"github.com/lestrrat-go/jwx/v3/jwt"
)

const (
	issuerKey                     = "basemachina.com"
	XBridgeAuthorizationHeaderKey = "X-Bridge-Authorization"
)

func parseBearer(headerKey string, header http.Header) (string, error) {
	if h := header.Get(headerKey); len(h) > 7 && strings.EqualFold(h[0:7], "BEARER ") {
		return h[7:], nil
	}
	return "", errors.New("bearer token not found")
}

// User is included in payload of the jwt which is coming from basemachina API.
type User struct {
	Tenant Tenant `json:"tenant"`
}

var _ TenantIDGetter = User{}

func (u User) GetTenantID() string { return u.Tenant.ID }

// Tenant is included in payload of the jwt which is coming from basemachina API.
type Tenant struct {
	ID string `json:"id"`
}

// TenantIDGetter is getter of tenant ID.
type TenantIDGetter interface {
	GetTenantID() string
}

// MiddlewareConfig is a config for Middleware function.
type MiddlewareConfig struct {
	TenantID string
	Logger   logr.Logger
	PublicKeyGetter

	// RegisterUserObject is an optional
	RegisterUserObject TenantIDGetter
}

// PublicKeyGetter is getter of public jwk.
type PublicKeyGetter interface {
	GetPublicKey() jwk.Set
}

// Middleware is a middleware to handle authn and authz
func Middleware(c *MiddlewareConfig) bridgehttp.Middleware {
	if c.RegisterUserObject != nil {
		jwt.RegisterCustomField(`user`, c.RegisterUserObject)
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			bearer, err := parseBearer(XBridgeAuthorizationHeaderKey, r.Header)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			now := ctxtime.Now(r.Context())

			// check also expired or not
			t, err := jwt.ParseString(bearer,
				jwt.WithValidate(true),
				jwt.WithIssuer(issuerKey),
				jwt.WithKeySet(c.PublicKeyGetter.GetPublicKey(), jws.WithInferAlgorithmFromKey(true)),
				jwt.WithClock(jwt.ClockFunc(func() time.Time {
					return now
				})),
			)
			if err != nil {
				c.Logger.Error(err, "jwt unauthorized error")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			tenantID, ok := getTenantID(t)
			if !ok {
				c.Logger.Info("user not found in claim")
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			if c.TenantID != "" && tenantID != c.TenantID {
				c.Logger.Info("mismatch tenant ID", tenantID, c.TenantID)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			// must not forward to proxy
			r.Header.Del(XBridgeAuthorizationHeaderKey)

			next.ServeHTTP(w, r)
		})
	}
}

func getTenantID(t jwt.Token) (string, bool) {
	var user TenantIDGetter
	if err := t.Get("user", &user); err != nil {
		return "", false
	}
	return user.GetTenantID(), true
}
