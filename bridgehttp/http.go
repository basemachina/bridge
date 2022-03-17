package bridgehttp

import "net/http"

// Middleware is middleware for http.Handler
type Middleware func(http.Handler) http.Handler

// UseMiddlewares uses some middlewares when handled specified h (http.Handler)
func UseMiddlewares(h http.Handler, middlewares ...Middleware) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}
