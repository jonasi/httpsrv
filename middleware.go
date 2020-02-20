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
	Handler(method, path string, h http.Handler) http.Handler
}

// MiddlewareFunc is a helper for defining middleware
type MiddlewareFunc func(method, path string, h http.Handler) http.Handler

// Handler implements Middleware
func (m MiddlewareFunc) Handler(method, path string, h http.Handler) http.Handler {
	return m(method, path, h)
}

// AccessLogger logs to w when requests come in the apache log format
func AccessLogger(w io.Writer) Middleware {
	return MiddlewareFunc(func(method, path string, h http.Handler) http.Handler {
		return apachelog.CombinedLog.Wrap(h, w)
	})
}

// H2C wraps a handler and provides support for upgrading to h2c
var H2C Middleware = MiddlewareFunc(func(method, path string, h http.Handler) http.Handler {
	s := &http2.Server{}
	h = h2c.NewHandler(h, s)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(w, r)
	})
})
