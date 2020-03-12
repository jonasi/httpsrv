package httpsrv

import (
	"context"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync/atomic"

	"github.com/jonasi/ctxlog"
	"github.com/jonasi/svc"
	"github.com/julienschmidt/httprouter"
)

// New returns an initialized Server
func New(addr string) *Server {
	mux := httprouter.New()
	mux.HandleMethodNotAllowed = false

	s := &Server{
		router: mux,
		addr:   []string{addr},
		server: &http.Server{
			Handler: mux,
		},
	}

	s.Service = svc.WrapBlocking(s.start, s.stop)

	return s
}

// Server is the http server
type Server struct {
	svc.Service
	addr       []string
	server     *http.Server
	middleware []Middleware
	router     *httprouter.Router
	routes     routes
	notFound   http.Handler
	started    int32
}

// AddListenAddr adds a new address to listen to when the server starts
func (s *Server) AddListenAddr(addr string) {
	if atomic.LoadInt32(&s.started) == 1 {
		panic("Attempting to add a listen addr after the server has started")
	}

	s.addr = append(s.addr, addr)
}

// Handle registers the provided route with the router
func (s *Server) Handle(rts ...*Route) {
	if atomic.LoadInt32(&s.started) == 1 {
		panic("Attempting to register routes after the server has started")
	}

	s.routes = append(s.routes, rts...)
}

// HandleNotFound registers a handler to run when a method is not found
func (s *Server) HandleNotFound(h http.Handler) {
	if atomic.LoadInt32(&s.started) == 1 {
		panic("Attempting to register routes after the server has started")
	}

	s.notFound = h
}

// NotFoundHandler returns the registered not found handler
func (s *Server) NotFoundHandler() http.Handler {
	if s.notFound != nil {
		return s.notFound
	}

	return http.NotFoundHandler()
}

// AddMiddleware adds global middleware to the server
func (s *Server) AddMiddleware(mw ...Middleware) {
	if atomic.LoadInt32(&s.started) == 1 {
		panic("Attempting to register global middleware after the server has started")
	}

	s.middleware = append(s.middleware, mw...)
}

// Start starts the server
func (s *Server) start(ctx context.Context) error {
	atomic.StoreInt32(&s.started, 1)
	s.server.BaseContext = func(_ net.Listener) context.Context {
		return ctx
	}

	ctxlog.Info(ctx, "Starting server")

	s.initRoutes(ctx)
	return s.initListeners(ctx)
}

var allMethods = []string{http.MethodPut, http.MethodGet, http.MethodPost, http.MethodHead, http.MethodTrace, http.MethodPatch, http.MethodDelete, http.MethodOptions, http.MethodConnect}

func (s *Server) initRoutes(ctx context.Context) {
	sort.Sort(s.routes)
	for _, r := range s.routes {
		h := r.Handler
		for i := len(r.Middleware) - 1; i >= 0; i-- {
			h = r.Middleware[i].Handler(r.Method, r.Path, h)
		}
		for i := len(s.middleware) - 1; i >= 0; i-- {
			h = s.middleware[i].Handler(r.Method, r.Path, h)
		}

		if r.Method == "*" {
			ctxlog.Infof(ctx, "Handling all methods for path: %s", r.Path)
			for _, method := range allMethods {
				s.router.Handler(method, r.Path, h)
			}
		} else {
			ctxlog.Infof(ctx, "Handling route: %-9s %s", r.Method, r.Path)
			s.router.Handler(r.Method, r.Path, h)
		}
	}

	nf := s.NotFoundHandler()
	for i := len(s.middleware) - 1; i >= 0; i-- {
		nf = s.middleware[i].Handler("", "", nf)
	}

	s.router.NotFound = nf
}

func (s *Server) initListeners(ctx context.Context) error {
	ls := []net.Listener{}

	for _, addr := range s.addr {
		var (
			l   net.Listener
			err error
		)

		switch {
		case strings.HasPrefix(addr, "unix://"):
			l, err = net.Listen("unix", addr[7:])
		default:
			l, err = net.Listen("tcp", addr)
		}
		if err != nil {
			for _, l := range ls {
				if err := l.Close(); err != nil {
					ctxlog.Errorf(ctx, "Attempting to cleanup listeners, but encountered error for listener %s: %s", l, err)
				}
			}
			return err
		}

		ls = append(ls, l)
	}

	var (
		serveCh   = make([]chan struct{}, len(ls))
		serveErrs = make([]error, len(ls))
		shutCh    = make(chan struct{})
	)

	for i, l := range ls {
		serveCh[i] = make(chan struct{})
		go func(l net.Listener, i int) {
			ctxlog.Infof(ctx, "Server listening at %s", l.Addr())
			err := s.server.Serve(l)

			// normal shutdown
			if err == http.ErrServerClosed {
				err = nil
			}
			serveErrs[i] = err
			close(serveCh[i])
		}(l, i)
	}

	for i := 0; i < len(ls); i++ {
		<-serveCh[i]
		if err := serveErrs[i]; err != nil {
			return err
		}
	}

	<-shutCh
	return nil
}

// Stop stops the server
func (s *Server) stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}
