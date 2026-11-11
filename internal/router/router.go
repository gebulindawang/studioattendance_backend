package router

import (
	"StudioAttendance_backend/internal/handler"
	"StudioAttendance_backend/internal/middleware"

	"github.com/gin-gonic/gin"
)

func Setup() *gin.Engine {
	r := gin.Default()
	api := r.Group("/api")
	user := api.Group("/user")
	schedule := api.Group("/schedule", middleware.JWTAuth())
	checkInRecord := api.Group("/record")

	handler.RegisterUserRoutes(user, middleware.JWTAuth())
	handler.RegisterScheduleRoutes(schedule)
	handler.RegisterCheckInRecordRoutes(checkInRecord)
	return r
}
