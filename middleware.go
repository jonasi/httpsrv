package httpsrv

import (
	"io"
	"net/http"

	apachelog "github.com/lestrrat-go/apache-logformat"
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
