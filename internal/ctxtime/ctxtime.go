package ctxtime

import (
	"context"
	"net/http"
	"time"

	"github.com/basemachina/bridge/bridgehttp"
)

type contextKey struct{}

// Now returns time.Time when requested. if time.Time does not exist in context
// this function cause panic.
//
// Use this function with Middleware function. In other words, this
// function is intended to be used within an application.
func Now(ctx context.Context) time.Time {
	return ctx.Value(contextKey{}).(time.Time)
}

// WithTime returns a copy of parent in which the value associated with time.Time is
// val.
//
// Use context Values only for request-scoped data that transits processes and
// APIs, not for passing optional parameters to functions.
func WithTime(ctx context.Context, t time.Time) context.Context {
	return context.WithValue(ctx, contextKey{}, t)
}

var now = time.Now

// Middleware to set time.Time which is the time when is called this handler.
//
// This function is intended to be used within an application.
func Middleware() bridgehttp.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			_, ok := ctx.Value(contextKey{}).(time.Time)
			if ok {
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			ctx = WithTime(ctx, now())
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
