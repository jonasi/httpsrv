package httpsrv

import (
	"context"
	"net"
	"net/http"
	"sort"

	"github.com/jonasi/ctxlog"
	"github.com/julienschmidt/httprouter"
)

// New returns an initialized Server
func New(addr string) *Server {
	mux := httprouter.New()
	s := &Server{
		router: mux,
		server: &http.Server{
			Addr:    addr,
			Handler: mux,
		},
		ch: make(chan error),
	}

	return s
}

// Server is the http server
type Server struct {
	server     *http.Server
	middleware []Middleware
	router     *httprouter.Router
	routes     routes
	ch         chan error
}

// Handle registers the provided handler with the router
func (s *Server) Handle(method, path string, h http.Handler, mw ...Middleware) {
	s.routes = append(s.routes, route{method: method, path: path, handler: h, mw: mw})
}

// AddMiddleware adds global middleware to the server
func (s *Server) AddMiddleware(mw ...Middleware) {
	s.middleware = append(s.middleware, mw...)
}

// Start starts the server
func (s *Server) Start(ctx context.Context) error {
	s.server.BaseContext = func(_ net.Listener) context.Context {
		return ctx
	}

	l, err := net.Listen("tcp", s.server.Addr)
	if err != nil {
		return err
	}

	log := ctxlog.KV(ctx, "addr", l.Addr())
	log.Info("Starting server")

	sort.Sort(s.routes)
	for _, r := range s.routes {
		log.Infof("Handling route: %s %s", r.method, r.path)
		h := r.handler

		for i := len(r.mw) - 1; i >= 0; i-- {
			h = r.mw[i].Handler(r.method, r.path, h)
		}

		for i := len(s.middleware) - 1; i >= 0; i-- {
			h = s.middleware[i].Handler(r.method, r.path, h)
		}

		s.router.Handler(r.method, r.path, h)
	}

	go func() {
		s.ch <- s.server.Serve(l)
	}()

	return nil
}

// Stop stops the server
func (s *Server) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// Wait returns when the server has been shut down
func (s *Server) Wait() error {
	return <-s.ch
}

type route struct {
	method  string
	path    string
	handler http.Handler
	mw      []Middleware
}

type routes []route

func (r routes) Len() int      { return len(r) }
func (r routes) Swap(i, j int) { r[i], r[j] = r[j], r[i] }
func (r routes) Less(i, j int) bool {
	if r[i].path < r[j].path {
		return true
	}

	return r[i].method == r[j].method
}
