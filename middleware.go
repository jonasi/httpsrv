package httpsrv

import (
	"io"
	"net/http"

	"github.com/jonasi/httpsrv/h2c"
	apachelog "github.com/lestrrat-go/apache-logformat"
	"golang.org/x/net/http2"
)

// Middleware takes an http route and spits out a new handler
type Middleware interface {
	ID() string
	Handler(method, path string, h http.Handler) http.Handler
}

type mw struct {
	id string
	mk func(string, string, http.Handler) http.Handler
}

func (m *mw) ID() string                                               { return m.id }
func (m *mw) Handler(method, path string, h http.Handler) http.Handler { return m.mk(method, path, h) }

// MiddlewareFunc is a helper for defining middleware
func MiddlewareFunc(id string, fn func(method, path string, h http.Handler) http.Handler) Middleware {
	return &mw{id: id, mk: fn}
}

// SkipMiddleware returns a MiddlewareFilter to skip specific middleware
func SkipMiddleware(mws ...Middleware) func(Middleware) bool {
	return func(m Middleware) bool {
		for _, m2 := range mws {
			if m.ID() == m2.ID() {
				return false
			}
		}

		return true
	}
}

// AccessLogger logs to w when requests come in the apache log format
func AccessLogger(w io.Writer) Middleware {
	return MiddlewareFunc("access_logger", func(method, path string, h http.Handler) http.Handler {
		return apachelog.CombinedLog.Wrap(h, w)
	})
}

// H2C wraps a handler and provides support for upgrading to h2c
var H2C Middleware = MiddlewareFunc("h2c", func(method, path string, h http.Handler) http.Handler {
	s := &http2.Server{}
	h = h2c.NewHandler(h, s)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(w, r)
	})
})
