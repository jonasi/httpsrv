package httpsrv

import (
	"encoding/json"
	"net/http"
	"text/template"

	"github.com/jonasi/ctxlog"
	"github.com/rakyll/statik/fs"
)

// HandleAssets registers an http.Handler to handle static assets
func HandleAssets(s *Server, prefix string, fs http.FileSystem) {
	if prefix[0] != '/' {
		prefix = "/" + prefix
	}
	if prefix[len(prefix)-1] != '/' {
		prefix = prefix + "/"
	}

	s.Handle(&Route{Method: "GET", Path: prefix + "*splat", Handler: http.StripPrefix(prefix, http.FileServer(fs))})
}

// TemplateHandler returns an http.Handler that renders the provided template
func TemplateHandler(t *template.Template, data interface{}) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := t.Execute(w, data); err != nil {
			ctxlog.Errorf(r.Context(), "Template render error: %s", err)
		}
	})
}

// SPAConf is a helper for defining routes and
// asset handling for single page apps
type SPAConf struct {
	IndexTemplate         string
	IndexTemplateEntryVar string
	IndexPaths            []string
	DevAssetsPath         string
	AssetFile             string
	AssetPrefix           string
}

// Init initializes all the routes and confs for SPAConf
func (c SPAConf) Init(s *Server) error {
	index, err := template.ParseFiles(c.IndexTemplate)
	if err != nil {
		return err
	}

	assets, err := c.fs()
	if err != nil {
		return err
	}
	b, err := fs.ReadFile(assets, c.AssetFile)
	if err != nil {
		return err
	}
	var js map[string]map[string]string
	if err := json.Unmarshal(b, &js); err != nil {
		return err
	}

	indexHandler := TemplateHandler(index, map[string]interface{}{
		"index_file": js["main"]["js"],
	})

	for _, p := range c.IndexPaths {
		s.Handle(&Route{Method: "GET", Path: p, Handler: indexHandler})
	}

	HandleAssets(s, c.AssetPrefix, assets)
	return nil
}
