package httpsrv

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sort"
	"sync/atomic"

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
		serveCh: make(chan struct{}),
		shutCh:  make(chan struct{}),
	}

	return s
}

// Server is the http server
type Server struct {
	server     *http.Server
	middleware []Middleware
	router     *httprouter.Router
	routes     routes
	state      int32 // 0: initial, 1: started, 2: stopped
	serveCh    chan struct{}
	serveErr   error
	shutCh     chan struct{}
	shutErr    error
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
	if !atomic.CompareAndSwapInt32(&s.state, 0, 1) {
		return errors.New("Attempting to start a server that has already been started")
	}

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
		s.serveErr = s.server.Serve(l)
		close(s.serveCh)
	}()

	return nil
}

// Stop stops the server
func (s *Server) Stop(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&s.state, 1, 2) {
		return errors.New("Attempting to stop a server that is not running")
	}

	s.shutErr = s.server.Shutdown(ctx)
	close(s.shutCh)
	return s.shutErr
}

// Wait returns when the server has been shut down
func (s *Server) Wait() error {
	<-s.serveCh
	if s.serveErr != http.ErrServerClosed {
		return s.serveErr
	}

	<-s.shutCh
	return s.shutErr
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
