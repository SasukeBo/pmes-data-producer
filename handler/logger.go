package handler

import (
	"bytes"
	"fmt"
	"github.com/SasukeBo/configer"
	"github.com/gin-gonic/gin"
	"gopkg.in/gookit/color.v1"
	"io/ioutil"
)

type responseWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (rw responseWriter) Write(b []byte) (int, error) {
	rw.body.Write(b)
	return rw.ResponseWriter.Write(b)
}

func HttpRequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		if false && configer.GetEnv("env") == "prod" {
			//if true || configer.GetEnv("env") == "prod" {
			c.Next()
			return
		}

		rw := &responseWriter{
			ResponseWriter: c.Writer,
			body:           bytes.NewBufferString(""),
		}
		c.Writer = rw
		body, _ := ioutil.ReadAll(c.Request.Body)
		c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(body))
		c.Next()
		fmt.Printf("\n%s\n", color.Warn.Render("[Debug Output]"))
		fmt.Printf("%s %s\n", color.Notice.Render("[Request Body]"), string(body))
		fmt.Printf("%s %s\n\n", color.Notice.Render("[Response Body]"), rw.body.String())
	}
}
