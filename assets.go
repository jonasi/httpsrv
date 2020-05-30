package httpsrv

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strings"

	"github.com/jonasi/ctxlog"
)

func fixPrefix(prefix string) string {
	if prefix[0] != '/' {
		prefix = "/" + prefix
	}
	if prefix[len(prefix)-1] != '/' {
		prefix = prefix + "/"
	}

	return prefix
}

// AssetsRoute returns a Route to handle static assets
func AssetsRoute(prefix string, fs http.FileSystem, mws ...Middleware) *Route {
	prefix = fixPrefix(prefix)

	return &Route{
		Method:     "GET",
		Path:       prefix + "*splat",
		Middleware: mws,
		Handler:    http.StripPrefix(prefix, http.FileServer(fs)),
	}
}

// TemplateHandler returns an http.Handler that renders the provided template
func TemplateHandler(t *template.Template, fn func(*http.Request) interface{}) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data := fn(r)
		if err := t.Execute(w, data); err != nil {
			ctxlog.Errorf(r.Context(), "Template render error: %s", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	})
}

// SPAConf is a helper for defining routes and
// asset handling for single page apps
type SPAConf struct {
	Root              string
	IndexTemplate     *template.Template
	IndexTemplateData func(*http.Request, map[string]map[string]interface{}) interface{}
	IndexFilter       func(*http.Request) bool
	IndexMiddleware   []Middleware
	Assets            http.FileSystem
	AssetFile         string
	AssetPrefix       string
	AssetMiddleware   []Middleware
}

// Init initializes all the routes and confs for SPAConf
func (c SPAConf) Init(s *Server) error {
	indexHandler, err := c.indexHandler(c.Assets)
	if err != nil {
		return err
	}

	// use (abuse?) the not found mechanism to load the client
	// only do it for GET requests that pass the provided IndexFilter method
	oldNF := s.NotFoundHandler()
	s.HandleNotFound(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c.IndexFilter != nil && r.Method == http.MethodGet && c.IndexFilter(r) {
			var (
				h                  = oldNF
				isGet              = r.Method == http.MethodGet
				matchesRoot        = c.Root == "" || strings.HasPrefix(r.URL.Path, c.Root)
				matchesIndexFilter = c.IndexFilter == nil || c.IndexFilter(r)
			)

			if isGet && matchesRoot && matchesIndexFilter {
				h = indexHandler
			}

			h.ServeHTTP(w, r)
		}
	}), c.IndexMiddleware...)

	pref := fixPrefix(c.AssetPrefix)
	if r := s.Lookup("GET", pref+"*splat"); r == nil {
		r := AssetsRoute(pref, c.Assets, c.AssetMiddleware...)
		s.Handle(r)
	}

	return nil
}

func (c SPAConf) mkIndexHandler(assets http.FileSystem) (http.Handler, error) {
	b, err := ReadFile(assets, c.AssetFile)
	if err != nil {
		return nil, err
	}
	var js map[string]map[string]interface{}
	if err := json.Unmarshal(b, &js); err != nil {
		return nil, err
	}

	return TemplateHandler(c.IndexTemplate, func(r *http.Request) interface{} {
		return c.IndexTemplateData(r, js)
	}), nil
}
