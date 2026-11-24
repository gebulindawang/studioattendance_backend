package main

import (
	"context"
	"log"
	"time"

	"StudioAttendance_backend/internal/config"
	"StudioAttendance_backend/internal/repository"
	"StudioAttendance_backend/internal/router"
	"StudioAttendance_backend/internal/util"
)

func main() {
	if err := config.Load(); err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	if err := repository.InitDB(); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	log.Printf("数据库初始化成功")
	if err := util.InitOSS(); err != nil {
		log.Printf("初始化oss失败(图片上传不可用): %v", err)
	} else {
		log.Printf("oss初始化成功")
	}
	go runAttendanceSettlement()

	r := router.Setup()

	if err := r.Run("127.0.0.1:" + config.AppPort); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}

func runAttendanceSettlement() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		err := repository.SettleAttendance(ctx, time.Now().In(config.AttendanceLocation))
		cancel()
		if err != nil {
			log.Printf("结算今日考勤失败: %v", err)
		}
		<-ticker.C
	}
}
