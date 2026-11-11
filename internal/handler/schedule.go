package handler

import (
	"StudioAttendance_backend/internal/middleware"
	"StudioAttendance_backend/internal/model"
	"StudioAttendance_backend/internal/repository"
	"StudioAttendance_backend/internal/response"
	"StudioAttendance_backend/internal/util"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
)

func UploadScheduleImage(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		response.FailErr(c, http.StatusBadRequest, "接受文件失败", err)
		return
	}
	safeName := filepath.Base(file.Filename)
	f, err := file.Open()
	if err != nil {
		response.FailErr(c, http.StatusBadRequest, "文件打开失败", err)
		return
	}
	defer f.Close()
	url, err := util.Upload(c.Request.Context(), "schedules", safeName, f)
	if err != nil {
		response.FailErr(c, http.StatusBadRequest, "上传图片失败", err)
		return
	}
	userID, err := middleware.CurrentUserID(c)
	if err != nil {
		response.FailErr(c, http.StatusUnauthorized, "登陆状态失效,请重新登录", err)
		return
	}
	if err = repository.CreateScheduleImage(url, userID); err != nil {
		response.FailErr(c, http.StatusInternalServerError, "保存课表图片失败", err)
		return
	}
	response.Ok(c, url)

}

func ParseScheduleImage(c *gin.Context) {
	userID , err:= middleware.CurrentUserID(c)
	if err != nil{
		response.FailErr(c,http.StatusBadRequest,"未找到该用户id",err)
		return
	}
	ImageURL,err := repository.FindLatestScheduleImage(userID)
	if err != nil{
		response.Fail(c,http.StatusBadRequest,err.Error())
		return
	}

	courses,err := util.ParseScheduleImage(ImageURL)
	if err != nil{
		response.Fail(c,http.StatusBadRequest,err.Error())
		return
	}
	response.Ok(c,courses)
}

func ListSchedules(c *gin.Context){
	userIDStr := c.Query("id")
	userID,err := strconv.Atoi(userIDStr)
	if err != nil{
		panic(err)
	}
	schedules,err := repository.ListSchedules(&userID)
	if err != nil{
		response.Fail(c,http.StatusBadRequest,err.Error())
		return
	}
	response.Ok(c,schedules)
}

func CreateSchedules(c *gin.Context){
	var schedules []model.Schedule
	if err :=repository.CreateSchedules(schedules);err != nil{
		response.Fail(c,http.StatusBadRequest,err.Error())
		return
	}
	response.OkMsg(c,"保存课表成功")
}

func UpdateSchedule(c *gin.Context){
	var schedule model.Schedule
	c.ShouldBindJSON(&schedule)
	if err := repository.UpdateSchedules(schedule);err != nil{
		response.FailErr(c,http.StatusBadRequest,"更改数据失败",err)
		return
	}
	response.OkMsg(c,"更改成功")
}

func RegisterScheduleRoutes(rg *gin.RouterGroup) {
	rg.POST("/upload", UploadScheduleImage)
	rg.GET("/parse", ParseScheduleImage)
	rg.GET("/",ListSchedules)
	rg.POST("/",CreateSchedules)
	rg.PUT("/",UpdateSchedule)
}
