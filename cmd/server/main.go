package main

import (
	"log"

	"StudioAttendance_backend/internal/config"
	"StudioAttendance_backend/internal/repository"
	"StudioAttendance_backend/internal/router"
	"StudioAttendance_backend/internal/util"
)

func main() {
	config.Load()

	if err := repository.InitDB(); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	log.Printf("数据库初始化成功")
	if err := util.InitOSS(); err != nil {
		log.Printf("初始化oss失败(图片上传不可用): %v", err)
	}
	log.Printf("oss初始化成功")

	r := router.Setup()

	if err := r.Run("127.0.0.1:8080"); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}
