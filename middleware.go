package httpsrv

import (
	"io"
	"net/http"

	"github.com/jonasi/httpsrv/h2c"
	apachelog "github.com/lestrrat-go/apache-logformat"
	"golang.org/x/net/http2"
)

func NopMiddleware(method, path string, h http.Handler) http.Handler {
	return h
}

// Middleware takes an http route and spits out a new handler
type Middleware interface {
	ID() string
	Handler(method, path string, h http.Handler) http.Handler
}

type MiddlewareFilterer interface {
	Middleware
	Filter([]Middleware) []Middleware
}

type mw struct {
	id string
	mk func(string, string, http.Handler) http.Handler
}

func (m *mw) ID() string                                               { return m.id }
func (m *mw) Handler(method, path string, h http.Handler) http.Handler { return m.mk(method, path, h) }

type mwf struct {
	mw
	filter func([]Middleware) []Middleware
}

func (m *mwf) Filter(mw []Middleware) []Middleware {
	return m.filter(mw)
}

// MiddlewareFunc is a helper for defining middleware
func MiddlewareFunc(id string, fn func(method, path string, h http.Handler) http.Handler) Middleware {
	return &mw{id: id, mk: fn}
}

func MiddlewareFilterFunc(id string, fn func([]Middleware) []Middleware) Middleware {
	return &mwf{mw: mw{id: id, mk: NopMiddleware}, filter: fn}
}

// SkipMiddleware returns a MiddlewareFilter to skip specific middleware
func SkipMiddleware(smws ...Middleware) Middleware {
	id := "__skip__"
	for i, mw := range smws {
		id += mw.ID()
		if i != len(smws)-1 {
			id += "_"
		}
	}

	return MiddlewareFilterFunc(id, func(mws []Middleware) []Middleware {
		filtered := []Middleware{}

		for _, m := range mws {
			add := true
			for _, sm := range smws {
				if sm.ID() == m.ID() {
					add = false
					break
				}
			}

			if add {
				filtered = append(filtered, m)
			}
		}

		return filtered
	})
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
