// +build !dev

package httpsrv

import (
	"net/http"
)

func (c SPAConf) indexHandler(assets http.FileSystem) (http.Handler, error) {
	return c.mkIndexHandler(assets)
}
