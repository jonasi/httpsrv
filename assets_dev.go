// +build dev

package httpsrv

import "net/http"

func (c SPAConf) fs() (http.FileSystem, error) {
	return http.Dir(c.DevAssetsPath), nil
}
