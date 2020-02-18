package httpsrv

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/jonasi/ctxlog"
	"github.com/julienschmidt/httprouter"
)

const (
	stateEmpty int = iota
	stateStarted
	stateStopped
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

	return s
}

// Server is the http server
type Server struct {
	addr       []string
	server     *http.Server
	middleware []Middleware
	router     *httprouter.Router
	routes     routes
	notFound   http.Handler
	mu         sync.Mutex
	state      int
	serveCh    []chan struct{}
	serveErrs  []error
	shutCh     chan struct{}
	shutErr    error
}

// AddListenAddr adds a new address to listen to when the server starts
func (s *Server) AddListenAddr(addr string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != stateEmpty {
		panic("Attempting to add a listen addr after the server has started")
	}

	s.addr = append(s.addr, addr)
}

// Handle registers the provided route with the router
func (s *Server) Handle(rts ...*Route) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != stateEmpty {
		panic("Attempting to register routes after the server has started")
	}

	s.routes = append(s.routes, rts...)
}

// HandleNotFound registers a handler to run when a method is not found
func (s *Server) HandleNotFound(h http.Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != stateEmpty {
		panic("Attempting to register routes after the server has started")
	}

	s.notFound = h
}

// AddMiddleware adds global middleware to the server
func (s *Server) AddMiddleware(mw ...Middleware) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != stateEmpty {
		panic("Attempting to register global middleware after the server has started")
	}
	s.middleware = append(s.middleware, mw...)
}

// Start starts the server
func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state != stateEmpty {
		return errors.New("Attempting to start a server that has already been started")
	}

	s.state = stateStarted

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

	if s.notFound != nil {
		s.router.NotFound = s.notFound
	}
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

	s.serveCh = make([]chan struct{}, len(ls))
	s.serveErrs = make([]error, len(ls))
	s.shutCh = make(chan struct{})

	for i, l := range ls {
		s.serveCh[i] = make(chan struct{})
		go func(l net.Listener, i int) {
			ctxlog.Infof(ctx, "Server listening at %s", l.Addr())
			err := s.server.Serve(l)

			// normal shutdown
			if err == http.ErrServerClosed {
				err = nil
			}
			s.serveErrs[i] = err
			close(s.serveCh[i])
		}(l, i)
	}

	return nil
}

// Stop stops the server
func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != stateStarted {
		return errors.New("Attempting to stop a server that is not running")
	}

	s.state = stateStopped

	s.shutErr = s.server.Shutdown(ctx)
	close(s.shutCh)
	return s.shutErr
}

// Wait returns when the server has been shut down
func (s *Server) Wait() error {
	s.mu.Lock()
	switch s.state {
	case stateEmpty:
		s.mu.Unlock()
		return errors.New("Attempting to wait on a server that is not running")
	case stateStarted, stateStopped:
	}

	l := len(s.serveCh)
	s.mu.Unlock()

	for i := 0; i < l; i++ {
		<-s.serveCh[i]
		if err := s.serveErrs[i]; err != nil {
			return err
		}
	}

	<-s.shutCh
	return s.shutErr
}
