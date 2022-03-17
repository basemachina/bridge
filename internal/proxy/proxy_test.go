package proxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/basemachina/bridge/internal/testlogr"
)

func TestNewReverseProxy(t *testing.T) {
	targetSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer targetSrv.Close()

	proxyHandler := NewProxy(testlogr.Logger)

	cases := []struct {
		name       string
		req        *http.Request
		wantStatus int
	}{
		{
			name: "valid proxy",
			req: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				req.Header.Set(TargetURLHeaderKey, targetSrv.URL)
				return req
			}(),
			wantStatus: http.StatusOK,
		},
		{
			name: "if not specified target URL",
			req: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				return req
			}(),
			wantStatus: http.StatusBadGateway,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := httptest.NewRecorder()
			proxyHandler.ServeHTTP(resp, tc.req)

			statusCode := resp.Result().StatusCode
			if tc.wantStatus != statusCode {
				t.Errorf(
					"status code, want %d, but got %d",
					tc.wantStatus,
					statusCode,
				)
			}
		})
	}
}

func TestNewHTTPReverseProxyClientClose(t *testing.T) {
	waitUntilCanceled := make(chan struct{})
	targetSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		<-waitUntilCanceled
		w.WriteHeader(http.StatusOK)
	}))
	defer targetSrv.Close()

	proxyHandler := NewProxy(testlogr.Logger)

	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set(TargetURLHeaderKey, "https://basemachina.com")

	resp := httptest.NewRecorder()
	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	go func() {
		time.Sleep(time.Millisecond)
		cancel()
		close(waitUntilCanceled)
	}()
	proxyHandler.ServeHTTP(resp, req.WithContext(ctx))

	want := httpStatusClientClosedRequest
	statusCode := resp.Result().StatusCode
	if want != statusCode {
		t.Errorf(
			"status code, want %d, but got %d",
			want,
			statusCode,
		)
	}
}
