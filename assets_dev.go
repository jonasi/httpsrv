// +build dev

package httpsrv

import (
	"net/http"

	"github.com/jonasi/ctxlog"
)

func (c SPAConf) indexHandler(assets http.FileSystem) (http.Handler, error) {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h, err := c.mkIndexHandler(assets)
		if err != nil {
			ctxlog.Errorf(r.Context(), "Error making index handler: %s", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		h.ServeHTTP(w, r)
	}), nil
}
