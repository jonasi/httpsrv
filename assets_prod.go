// +build !dev

package httpsrv

import (
	"net/http"

	"github.com/rakyll/statik/fs"
)

func (c SPAConf) fs() (http.FileSystem, error) {
	return fs.New()
}

func (c SPAConf) indexHandler(assets http.FileSystem) (http.Handler, error) {
	return c.mkIndexHandler(assets)
}
