package bridgehttp

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUseMiddlewares(t *testing.T) {
	var buf bytes.Buffer
	baseHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf.WriteString("handler")
	})
	alpha := []string{"A", "B", "C"}
	middlewares := make([]Middleware, 0, len(alpha))
	for _, v := range alpha {
		v := v
		middlewares = append(middlewares, func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				buf.WriteString(v)
				next.ServeHTTP(w, r)
				buf.WriteString(v)
			})
		})
	}

	h := UseMiddlewares(baseHandler, middlewares...)
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	want := "ABChandlerCBA"
	got := buf.String()
	if want != got {
		t.Errorf("want %q, but got %q", want, got)
	}
}
