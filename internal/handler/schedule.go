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
	file,err:= c.FormFile("file")
	if err != nil{
		response.FailErr(c,http.StatusBadRequest,"接受文件失败",err)
		return
	}
	safeName := filepath.Base(file.Filename)
	f,err := file.Open()
	if err != nil{
		response.FailErr(c,http.StatusBadRequest,"文件打开失败",err)
		return
	}
	defer f.Close()
	url,err := util.Upload(c.Request.Context(),"schedules",safeName,f)
	if err != nil{
		response.FailErr(c,http.StatusBadRequest,"上传图片失败",err)
		return
	}
	userID ,err := middleware.CurrentUserID(c)
	err = repository.CreateScheduleImage(url,userID)
	response.Ok(c,url)

}

func ParseScheduleImage(c *gin.Context){
	// imageURL,_ := repository.FindScheduleImageByID(c.GetInt(middleware.CtxUserID))
}

func RegisterScheduleRoutes(rg *gin.RouterGroup){
	rg.POST("/upload",UploadScheduleImage)
	rg.GET("/Parse",ParseScheduleImage)
}