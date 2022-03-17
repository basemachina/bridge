package bridge

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/basemachina/bridge/internal/testlogr"
)

func TestNewHTTPHandler(t *testing.T) {
	t.Run("ok path", func(t *testing.T) {
		h := NewHTTPHandler(&HTTPHandlerConfig{
			Logger: testlogr.Logger,
		})
		req := httptest.NewRequest("GET", OKPath, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("want status code %d but got %d", http.StatusOK, rec.Code)
		}
		if got := rec.Body.String(); OKMessage != got {
			t.Fatalf("want message %q but got %q", OKMessage, got)
		}
	})
}
