package main

import (
	"fmt"
	"github.com/SasukeBo/configer"
	"github.com/SasukeBo/log"
	"github.com/SasukeBo/pmes-data-producer/handler"
	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()
	//r.Use(cors.Default())

	// Panic Recovery
	r.Use(gin.Recovery())

	// Data transfer
	r.POST("/produce", handler.HttpRequestLogger(), handler.DeviceProduce()) // 设备上传生产数据

	log.Info("start service on [%s] mode", configer.GetEnv("env"))
	r.Run(fmt.Sprintf(":%s", configer.GetString("port")))
}
