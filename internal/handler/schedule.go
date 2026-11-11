package handler

import (
	"StudioAttendance_backend/internal/middleware"
	"StudioAttendance_backend/internal/repository"
	"StudioAttendance_backend/internal/response"
	"StudioAttendance_backend/internal/util"
	"net/http"
	"path/filepath"


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

func RegisterScheduleRoutes(rg *gin.RouterGroup) {
	rg.POST("/upload", UploadScheduleImage)
	rg.GET("/parse", ParseScheduleImage)
}
