package handler

import (
	"StudioAttendance_backend/internal/config"
	"StudioAttendance_backend/internal/middleware"
	"StudioAttendance_backend/internal/model"
	"StudioAttendance_backend/internal/repository"
	"StudioAttendance_backend/internal/response"
	"StudioAttendance_backend/internal/util"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var errPermission = errors.New("没有权限操作其他用户的课表")

func resolveTargetUser(c *gin.Context, raw string) (int, error) {
	userID, err := middleware.CurrentUserID(c)
	if err != nil || raw == "" {
		return userID, err
	}
	target, err := strconv.Atoi(raw)
	if err != nil || target <= 0 {
		return 0, errors.New("用户 ID 不合法")
	}
	role, err := middleware.CurrentUserRole(c)
	if err != nil {
		return 0, err
	}
	if target != userID && role != int(model.RoleAdmin) {
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

func failSchedule(c *gin.Context, msg string, err error) {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		response.Fail(c, http.StatusNotFound, "请求的用户、课表或配置不存在")
	case errors.Is(err, repository.ErrScheduleUploadLimit), errors.Is(err, repository.ErrScheduleImageChanged):
		response.Fail(c, http.StatusConflict, err.Error())
	default:
		response.FailErr(c, http.StatusInternalServerError, msg, err)
	}
}

func UploadScheduleImage(c *gin.Context) {
	const maxSize = 5 << 20
	userID, _ := middleware.CurrentUserID(c)
	amount, err := repository.FindImagesAmountByUserId(userID)
	if err != nil {
		failSchedule(c, "查询上传次数失败", err)
		return
	}
	if amount >= model.MaxScheduleUploads {
		failSchedule(c, "", repository.ErrScheduleUploadLimit)
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSize+(64<<10))
	file, err := c.FormFile("file")
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			response.Fail(c, http.StatusRequestEntityTooLarge, "课表图片不能超过 5 MB")
			return
		}
		response.Fail(c, http.StatusBadRequest, "请选择课表图片")
		return
	}
	if file.Size > maxSize {
		response.Fail(c, http.StatusRequestEntityTooLarge, "课表图片不能超过 5 MB")
		return
	}
	f, err := file.Open()
	if err != nil {
		failSchedule(c, "读取图片失败", err)
		return
	}
	defer f.Close()
	var header [512]byte
	n, err := io.ReadFull(f, header[:])
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		response.Fail(c, http.StatusBadRequest, "图片为空或无法读取")
		return
	}
	extensions := map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp"}
	extension := extensions[http.DetectContentType(header[:n])]
	if extension == "" {
		response.Fail(c, http.StatusUnsupportedMediaType, "请上传 JPG、PNG 或 WEBP 图片")
		return
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		failSchedule(c, "读取图片失败", err)
		return
	}
	url, key, err := util.Upload(c.Request.Context(), "schedules", "schedule"+extension, f)
	if err != nil {
		failSchedule(c, "上传图片失败", err)
		return
	}
	if err := repository.CreateScheduleImage(url, userID); err != nil {
		if errors.Is(err, repository.ErrScheduleUploadLimit) || errors.Is(err, gorm.ErrRecordNotFound) {
			ctx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 5*time.Second)
			defer cancel()
			if cleanupErr := util.DeleteUpload(ctx, key); cleanupErr != nil {
				log.Printf("清理未保存的课表图片失败: %v", cleanupErr)
			}
		}
		failSchedule(c, "保存课表图片失败", err)
		return
	}
	response.Ok(c, url)
}

func FindCountByUserId(c *gin.Context) {
	userID, _ := middleware.CurrentUserID(c)
	count, err := repository.FindImagesAmountByUserId(userID)
	if err != nil {
		failSchedule(c, "查询上传次数失败", err)
		return
	}
	response.Ok(c, count)
}

func ParseScheduleImage(c *gin.Context) {
	userID, err := resolveTargetUser(c, c.Query("user_id"))
	if err != nil {
		failTargetUser(c, err)
		return
	}
	image, err := repository.FindLatestScheduleImage(userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		response.Fail(c, http.StatusNotFound, "该用户还没有上传课表图片")
		return
	}
	if err != nil {
		failSchedule(c, "查询课表图片失败", err)
		return
	}
	if id := c.Query("image_id"); id != "" && id != strconv.Itoa(image.ID) {
		failSchedule(c, "", repository.ErrScheduleImageChanged)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 80*time.Second)
	defer cancel()
	courses, err := util.ParseScheduleImage(ctx, image.ImagesUrl)
	if err != nil {
		response.FailErr(c, http.StatusBadGateway, "识别失败，请稍后重试", err)
		return
	}
	response.Ok(c, courses)
}

func ListSchedules(c *gin.Context) {
	userID, err := resolveTargetUser(c, c.Query("id"))
	if err != nil {
		failTargetUser(c, err)
		return
	}
	schedules, err := repository.ListSchedules(userID)
	if err != nil {
		failSchedule(c, "查询课表失败", err)
		return
	}
	response.Ok(c, schedules)
}

func ListScheduleMembers(c *gin.Context) {
	summaries, err := repository.ListScheduleMembers()
	if err != nil {
		failSchedule(c, "查询课表状态失败", err)
		return
	}
	response.Ok(c, summaries)
}

func ScheduleDetail(c *gin.Context) {
	userID, err := resolveTargetUser(c, c.Query("user_id"))
	if err != nil {
		failTargetUser(c, err)
		return
	}
	detail, err := repository.GetScheduleDetail(userID)
	if err != nil {
		failSchedule(c, "查询课表详情失败", err)
		return
	}
	response.Ok(c, detail)
}

func validateSchedules(c *gin.Context, schedules []model.Schedule) bool {
	config, err := repository.FindSemesterAndSectionTime()
	if err != nil {
		failSchedule(c, "读取学期配置失败", err)
		return false
	}
	if err := repository.ValidateSchedules(schedules, config); err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return false
	}
	for i := range schedules {
		schedules[i].CourseName = strings.TrimSpace(schedules[i].CourseName)
		schedules[i].Location = strings.TrimSpace(schedules[i].Location)
	}
	return true
}

func CreateSchedules(c *gin.Context) {
	userID, err := resolveTargetUser(c, c.Query("user_id"))
	if err != nil {
		failTargetUser(c, err)
		return
	}
	var imageID *int
	if raw := c.Query("image_id"); raw != "" {
		id, err := strconv.Atoi(raw)
		if err != nil || id < 0 {
			response.Fail(c, http.StatusBadRequest, "课表图片 ID 不合法")
			return
		}
		imageID = &id
	}
	var schedules []model.Schedule
	if err := c.ShouldBindJSON(&schedules); err != nil || len(schedules) == 0 {
		response.Fail(c, http.StatusBadRequest, "请提供课程列表")
		return
	}
	if !validateSchedules(c, schedules) {
		return
	}
	if err := repository.ReplaceUserSchedules(userID, schedules, imageID); err != nil {
		failSchedule(c, "保存课表失败", err)
		return
	}
	response.OkMsg(c, "保存课表成功")
}

func UpdateSchedule(c *gin.Context) {
	var schedule model.Schedule
	if err := c.ShouldBindJSON(&schedule); err != nil || schedule.ID <= 0 {
		response.Fail(c, http.StatusBadRequest, "课程格式或 ID 不合法")
		return
	}
	schedules := []model.Schedule{schedule}
	if !validateSchedules(c, schedules) {
		return
	}
	if err := repository.UpdateSchedule(schedules[0]); err != nil {
		failSchedule(c, "更改课程失败", err)
		return
	}
	response.OkMsg(c, "更改成功")
}

func FindScheduleImageByID(c *gin.Context) {
	userID, err := resolveTargetUser(c, c.Param("id"))
	if err != nil {
		failTargetUser(c, err)
		return
	}
	urls, err := repository.FindScheduleImageByID(userID)
	if err != nil {
		failSchedule(c, "查询课表图片失败", err)
		return
	}
	response.Ok(c, urls)
}

func DeleteScheduleByUserId(c *gin.Context) {
	userID, err := resolveTargetUser(c, c.Query("user_id"))
	if err != nil {
		failTargetUser(c, err)
		return
	}
	if err := repository.DeleteScheduleByUserId(userID); err != nil {
		failSchedule(c, "重置课表失败", err)
		return
	}
	response.OkMsg(c, "重置课表成功")
}

func FindSemesterAndSectionTime(c *gin.Context) {
	config, err := repository.FindSemesterAndSectionTime()
	if err != nil {
		failSchedule(c, "查询学期配置失败", err)
		return
	}
	response.Ok(c, config)
}

func UpdateSemesterOrSectionTime(c *gin.Context) {
	var settings model.SemesterConfig
	if err := c.ShouldBindJSON(&settings); err != nil || (settings.Semester == nil && len(settings.SectionTime) == 0) {
		response.Fail(c, http.StatusBadRequest, "请提供学期或节次配置")
		return
	}
	if semester := settings.Semester; semester != nil {
		start, end := semester.StartDate.Format(time.DateOnly), semester.EndDate.Format(time.DateOnly)
		if semester.ID < 0 || semester.StartDate.Year() < 1000 || semester.EndDate.Year() > 9999 ||
			start > end || semester.TotalWeeks < 1 || semester.TotalWeeks > 30 {
			response.Fail(c, http.StatusBadRequest, "学期起止日期或总周数不合法")
			return
		}
		semester.StartDate, _ = time.ParseInLocation(time.DateOnly, start, config.AttendanceLocation)
		semester.EndDate, _ = time.ParseInLocation(time.DateOnly, end, config.AttendanceLocation)
	}
	seen := make(map[int]bool)
	for _, block := range settings.SectionTime {
		number, numberErr := strconv.Atoi(block.SectionNumber)
		start, startErr := time.Parse(time.TimeOnly, block.StartTime)
		end, endErr := time.Parse(time.TimeOnly, block.EndTime)
		if numberErr != nil || number < 1 || number > 24 || block.ID < 0 ||
			block.SectionNumber != strconv.Itoa(number) || seen[number] ||
			startErr != nil || endErr != nil || !end.After(start) {
			response.Fail(c, http.StatusBadRequest, fmt.Sprintf("大节 %s 的编号或起止时间不合法", block.SectionNumber))
			return
		}
		seen[number] = true
	}
	if err := repository.UpdateSemesterOrSectionTime(settings); err != nil {
		failSchedule(c, "更新学期配置失败", err)
		return
	}
	response.OkMsg(c, "配置已保存")
}

func RegisterScheduleRoutes(rg *gin.RouterGroup) {
	admin := middleware.RequireRole(model.RoleAdmin)
	member := middleware.RequireRole(model.RoleMember)
	rg.POST("/upload", member, UploadScheduleImage)
	rg.GET("/parse", admin, ParseScheduleImage)
	rg.GET("/", ListSchedules)
	rg.POST("/", admin, CreateSchedules)
	rg.PUT("/", admin, UpdateSchedule)
	rg.GET("/:id", FindScheduleImageByID)
	rg.GET("/count", FindCountByUserId)
	rg.GET("/me", ScheduleDetail)
	rg.GET("/members", admin, ListScheduleMembers)
	rg.DELETE("/resove", admin, DeleteScheduleByUserId)
	rg.GET("/semester", FindSemesterAndSectionTime)
	rg.POST("/updateConfig", admin, UpdateSemesterOrSectionTime)
}
