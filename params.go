package httpsrv

import (
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
	uuid "github.com/satori/go.uuid"
)

// ReqValue is a value pulled from the request
type ReqValue string

func (r ReqValue) String() string {
	return string(r)
}

// Bool returns r as a bool
func (r ReqValue) Bool() bool {
	v, _ := strconv.ParseBool(string(r))
	return v
}

// Int returns r as an int
func (r ReqValue) Int() int {
	v, _ := strconv.Atoi(string(r))
	return v
}

// Float64 returns r as a float64
func (r ReqValue) Float64() float64 {
	v, _ := strconv.ParseFloat(string(r), 64)
	return v
}

// UUID returns r as a UUID
func (r ReqValue) UUID() uuid.UUID {
	return uuid.FromStringOrNil(string(r))
}

// ParamValue returns a ReqValue from the url path
func ParamValue(r *http.Request, key string) ReqValue {
	return ReqValue(httprouter.ParamsFromContext(r.Context()).ByName(key))
}

// QueryValue returns a ReqValue from the query string
func QueryValue(r *http.Request, key string) ReqValue {
	return ReqValue(r.URL.Query().Get(key))
}
