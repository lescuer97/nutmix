package utils

import (
	"net/http"

	jsonv2 "encoding/json/v2"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
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

func JSON(c *gin.Context, code int, data any) {
	c.Render(code, JSONV2{Data: data})
}

func DecodeJSONV2(c *gin.Context, out any) error {
	return jsonv2.UnmarshalRead(c.Request.Body, out, jsonv2.RejectUnknownMembers(true))
}
