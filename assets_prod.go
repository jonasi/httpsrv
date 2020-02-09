// +build !dev

package httpsrv

import (
	"net/http"

	"github.com/rakyll/statik/fs"
)

func (c SPAConf) fs() (http.FileSystem, error) {
	return fs.New()
}
