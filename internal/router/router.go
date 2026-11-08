package router

import (
	"StudioAttendance_backend/internal/handler"

	"github.com/gin-gonic/gin"
)

func Setup() *gin.Engine {
	r := gin.Default()
	api := r.Group("/api")
	user := api.Group("/user")
	schedule := api.Group("/schedule")
	
	handler.RegisterUserRoutes(user)
	handler.RegisterScheduleRoutes(schedule)

	return r
}
