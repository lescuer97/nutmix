package utils

import (
	"net/http"

	"github.com/gin-gonic/gin/render"
	jsonv2 "github.com/go-json-experiment/json"
)

type JSONV2 struct {
	Data any
}

func (r JSONV2) Render(w http.ResponseWriter) error {
	r.WriteContentType(w)
	bytes, err := jsonv2.Marshal(r.Data)
	if err != nil {
		return err
	}
	_, err = w.Write(bytes)
	return err
}

func (r JSONV2) WriteContentType(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
}

var _ render.Render = JSONV2{}
