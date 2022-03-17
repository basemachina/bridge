package ctxtime

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNow(t *testing.T) {
	want := time.Date(2020, 1, 21, 0, 0, 0, 0, time.UTC)
	ctx := context.Background()
	ctx = WithTime(ctx, want)
	got := Now(ctx)
	if !want.Equal(got) {
		t.Fatalf("got %v but want %v", got, want)
	}
}

func TestMiddleware(t *testing.T) {
	t.Cleanup(func() { now = time.Now })
	want := time.Date(2020, 1, 21, 0, 0, 0, 0, time.UTC)
	now = func() time.Time {
		return want
	}
	h := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got := Now(r.Context())
		if !want.Equal(got) {
			t.Fatalf("got %v but want %v", got, want)
		}
	})
	r := httptest.NewRequest("GET", "/", nil)
	Middleware()(h).ServeHTTP(nil, r)

	// Use time which is propagated by incoming context.
	want2 := want.Add(time.Second)
	ctx := WithTime(context.Background(), want2)
	r2 := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
	h2 := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got := Now(r.Context())
		if !want2.Equal(got) {
			t.Fatalf("got %v but want %v", got, want)
		}
	})
	Middleware()(h2).ServeHTTP(nil, r2)
}
