package handler

import (
	"StudioAttendance_backend/internal/middleware"
	"StudioAttendance_backend/internal/model"
	"StudioAttendance_backend/internal/repository"
	"StudioAttendance_backend/internal/response"
	"StudioAttendance_backend/internal/util"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// errPermission 标记"目标用户不是自己且调用者不是管理员"
var errPermission = errors.New("没有权限操作其他用户的课表")

// resolveTargetUser 解析本次操作的目标用户:
// 不带 user_id 参数时默认为当前登录用户;带 user_id 时仅管理员可指定他人。
func resolveTargetUser(c *gin.Context) (int, error) {
	userID, err := middleware.CurrentUserID(c)
	if err != nil {
		return 0, err
	}
	raw := c.Query("user_id")
	if raw == "" {
		return userID, nil
	}
	target, err := strconv.Atoi(raw)
	if err != nil || target <= 0 {
		return 0, fmt.Errorf("user_id 参数不合法")
	}
	if target == userID {
		return target, nil
	}
	role, err := middleware.CurrentUserRole(c)
	if err != nil {
		return 0, err
	}
	if role != int(model.RoleAdmin) {
		return 0, errPermission
	}
	return target, nil
}

func failTargetUser(c *gin.Context, err error) {
	if errors.Is(err, errPermission) {
		response.Fail(c, http.StatusForbidden, err.Error())
		return
	}
	response.Fail(c, http.StatusBadRequest, err.Error())
}

func UploadScheduleImage(c *gin.Context) {
	userId, _ := middleware.CurrentUserID(c)
	amount, err := repository.FindImagesAmountByUserId(userId)
	if err != nil {
		response.FailErr(c, http.StatusBadRequest, "查询失败", err)
		return
	}
	if amount >= 3 {
		response.Fail(c, http.StatusBadRequest, "上传图片已到达最大限制")
		return
	}

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

func FindCountByUserId(c *gin.Context) {
	userId, err := middleware.CurrentUserID(c)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "用户状态异常")
		return
	}

	count, err := repository.FindImagesAmountByUserId(userId)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "查询数量失败")
		return
	}
	response.Ok(c, count)
}

func ParseScheduleImage(c *gin.Context) {
	// 管理员带 ?user_id= 可解析指定成员的最新一张课表图,不带则解析自己的
	userID, err := resolveTargetUser(c)
	if err != nil {
		failTargetUser(c, err)
		return
	}
	ImageURL, err := repository.FindLatestScheduleImage(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, http.StatusBadRequest, "该用户还没有上传课表图片")
		} else {
			response.FailErr(c, http.StatusInternalServerError, "查询课表图片失败", err)
		}
		return
	}

	courses, err := util.ParseScheduleImage(ImageURL)
	if err != nil {
		response.FailErr(c, http.StatusBadRequest, "识别失败,请稍后重试", err)
		return
	}
	response.Ok(c, courses)
}

func ListSchedules(c *gin.Context) {
	userIDStr := c.Query("id")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		panic(err)
	}
	schedules, err := repository.ListSchedules(&userID)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Ok(c, schedules)
}

// CreateSchedules 用请求体的课程数组整体替换目标用户的课表(先删后插)。
// 管理员带 ?user_id= 可指定成员,不带则操作自己。body 中 teacher 等多余字段会被忽略。
func CreateSchedules(c *gin.Context) {
	userID, err := resolveTargetUser(c)
	if err != nil {
		failTargetUser(c, err)
		return
	}
	var schedules []model.Schedule
	if err := c.ShouldBindJSON(&schedules); err != nil {
		response.FailErr(c, http.StatusBadRequest, "课程数据格式错误", err)
		return
	}
	if len(schedules) == 0 {
		response.Fail(c, http.StatusBadRequest, "课程列表不能为空")
		return
	}
	if err := repository.ReplaceUserSchedules(userID, schedules); err != nil {
		response.FailErr(c, http.StatusInternalServerError, "保存课表失败", err)
		return
	}
	response.OkMsg(c, "保存课表成功")
}

func UpdateSchedule(c *gin.Context) {
	var schedule model.Schedule
	c.ShouldBindJSON(&schedule)
	if err := repository.UpdateSchedules(schedule); err != nil {
		response.FailErr(c, http.StatusBadRequest, "更改数据失败", err)
		return
	}
	response.OkMsg(c, "更改成功")
}

func FindScheduleImageByID(c *gin.Context) {
	id := c.Param("id")
	userID, err := strconv.Atoi(id)
	fmt.Println(userID)
	if err != nil {
		response.FailErr(c, http.StatusBadRequest, "用户id失败", err)
		return
	}
	result, err := repository.FindScheduleImageByID(userID)
	if err != nil {
		panic(err)
	}
	response.Ok(c, result)
	
}

func DeleteScheduleByUserId(c *gin.Context) {
	// 管理员带 ?user_id= 重置指定成员的课表;不带则重置自己的
	userId, err := resolveTargetUser(c)
	if err != nil {
		failTargetUser(c, err)
		return
	}
	if err := repository.DeleteScheduleByUserId(userId); err != nil {
		response.Fail(c, http.StatusBadRequest, "重置失败,请稍后重试")
		return
	}
	response.OkMsg(c, "重置课表成功")
}

func RegisterScheduleRoutes(rg *gin.RouterGroup) {
	rg.POST("/upload", UploadScheduleImage)
	rg.GET("/parse", ParseScheduleImage)
	rg.GET("/", ListSchedules)
	rg.POST("/", CreateSchedules)
	rg.PUT("/", UpdateSchedule)
	rg.GET("/:id", FindScheduleImageByID)
	rg.GET("/count", FindCountByUserId)
	rg.DELETE("/resove",DeleteScheduleByUserId)
}
