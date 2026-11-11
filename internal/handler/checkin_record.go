package handler

import (
	"StudioAttendance_backend/internal/repository"
	"StudioAttendance_backend/internal/response"
	"net/http"

	"github.com/gin-gonic/gin"
)

func ListRecord(c *gin.Context) {
	result,err := repository.ListCheckinRecord()
	if err != nil {
		response.Fail(c,http.StatusBadRequest,"查询失败")
		return
	}
	response.Ok(c,result)
}


func RegisterCheckInRecordRoutes(rg *gin.RouterGroup){
	rg.GET("/")
}