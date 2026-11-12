package handler

import (
	"StudioAttendance_backend/internal/repository"
	"StudioAttendance_backend/internal/response"
	"net/http"
	"strconv"

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

// 签到关键接口
func checkIn(c *gin.Context){
	userId , err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Fail(c,http.StatusBadRequest,"用户 id 格式错误")
		return
	}
	section , err := strconv.Atoi(c.Param("section"))
	if err != nil {
		response.Fail(c,http.StatusBadRequest,"课节编号格式错误")
		return
	}
	week , err := strconv.Atoi(c.Param("week"))
	if err != nil {
		response.Fail(c,http.StatusBadRequest,"周次格式错误")
		return
	}
	if err := repository.CheckIn(userId,section,week); err != nil {
		response.Fail(c,http.StatusBadRequest,err.Error())
		return
	}
	response.Ok(c,nil)
}

func RegisterCheckInRecordRoutes(rg *gin.RouterGroup){
	rg.GET("/",ListRecord)
	rg.GET("/checkIn/:id/:section/:week",checkIn)
}