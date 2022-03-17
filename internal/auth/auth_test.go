package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/basemachina/bridge/internal/ctxtime"
	"github.com/basemachina/bridge/internal/rand"
	"github.com/basemachina/bridge/internal/testlogr"
)

func TestMiddleware(t *testing.T) {
	tenantID := rand.String()
	wantUser := User{
		Tenant: Tenant{
			ID: tenantID,
		},
	}

	ret, err := CreateJWT(wantUser)
	if err != nil {
		t.Fatal(err)
	}

	headerKey := XBridgeAuthorizationHeaderKey

	tm := ret.IssuedAt
	accessToken := ret.Token
	expireIn := ret.ExpireIn

	cases := []struct {
		name       string
		req        *http.Request
		tenantID   string
		wantStatus int
	}{
		{
			name: "valid",
			req: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				req.Header.Add(headerKey, "bearer "+string(accessToken))
				ctx := req.Context()
				now := tm.Add(time.Second) // adding for "nbf"
				ctx = ctxtime.WithTime(ctx, now)
				return req.WithContext(ctx)
			}(),
			tenantID:   tenantID,
			wantStatus: http.StatusOK,
		},
		{
			name: "invalid mismatch tenant ID",
			req: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				req.Header.Add(headerKey, "bearer "+string(accessToken))
				ctx := req.Context()
				now := tm.Add(time.Second) // adding for "nbf"
				ctx = ctxtime.WithTime(ctx, now)
				return req.WithContext(ctx)
			}(),
			tenantID:   "invalid",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "invalid usage timing (nbf)",
			req: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				req.Header.Add(headerKey, "bearer "+string(accessToken))
				ctx := req.Context()
				ctx = ctxtime.WithTime(ctx, tm.Add(-2*time.Hour)) // before time than nbf
				return req.WithContext(ctx)
			}(),
			tenantID:   tenantID,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "invalid expired",
			req: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				req.Header.Add(headerKey, "bearer "+string(accessToken))
				ctx := req.Context()
				ctx = ctxtime.WithTime(ctx, tm.Add(expireIn+time.Second))
				return req.WithContext(ctx)
			}(),
			tenantID:   tenantID,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "invalid non authorized header",
			req:        httptest.NewRequest("GET", "/", nil),
			tenantID:   tenantID,
			wantStatus: http.StatusBadRequest,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
			rec := httptest.NewRecorder()
			middleware := Middleware(&MiddlewareConfig{
				TenantID: tc.tenantID,
				Logger:   testlogr.Logger,
				PublicKeyGetter: &StaticPublicKeyGetter{
					PublicKey: ret.PublicKeySet,
				},
			})
			middleware(h).ServeHTTP(rec, tc.req)
			if tc.wantStatus != rec.Code {
				t.Fatalf("want %d, but got %d", tc.wantStatus, rec.Code)
			}
		})
	}
}
