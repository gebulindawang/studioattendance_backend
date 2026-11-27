package handler

import (
	"StudioAttendance_backend/internal/config"
	"StudioAttendance_backend/internal/middleware"
	"StudioAttendance_backend/internal/model"
	"StudioAttendance_backend/internal/repository"
	"StudioAttendance_backend/internal/response"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

func ListRecord(c *gin.Context) {
	now := time.Now().In(config.AttendanceLocation)
	result, err := repository.ListCheckinRecord(now)
	if err != nil {
		attendanceFailure(c, "查询考勤记录失败", err)
		return
	}
	response.Ok(c, gin.H{"date": now.Format(time.DateOnly), "records": result})
}

func weekAttendance(c *gin.Context) {
	userID, _ := middleware.CurrentUserID(c)
	now := time.Now().In(config.AttendanceLocation)
	result, err := repository.GetWeekAttendance(userID, now)
	if err != nil {
		attendanceFailure(c, "查询本周签到失败", err)
		return
	}
	response.Ok(c, result)
}

func checkIn(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, strconv.IntSize)
	if err != nil || id == 0 {
		response.Fail(c, http.StatusBadRequest, "签到记录编号不合法")
		return
	}
	userID, _ := middleware.CurrentUserID(c)
	record, err := repository.CheckIn(userID, uint(id), time.Now().In(config.AttendanceLocation))
	switch {
	case err == nil:
		response.Ok(c, record)
	case errors.Is(err, repository.ErrCheckinUnavailable):
		response.Fail(c, http.StatusNotFound, err.Error())
	case errors.Is(err, repository.ErrCheckinTooEarly), errors.Is(err, repository.ErrCheckinClosed),
		errors.Is(err, repository.ErrCheckinCourseConflict), errors.Is(err, repository.ErrCheckinContinuation),
		errors.Is(err, repository.ErrCheckinPeriodSigned):
		response.Fail(c, http.StatusConflict, err.Error())
	default:
		attendanceFailure(c, "签到失败", err)
	}
}

func generateWeekAttendance(c *gin.Context) {
	var input struct {
		UserIDs []int `json:"user_ids"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || len(input.UserIDs) == 0 {
		response.Fail(c, http.StatusBadRequest, "请选择需要生成签到表的成员")
		return
	}
	for _, id := range input.UserIDs {
		if id <= 0 {
			response.Fail(c, http.StatusBadRequest, "成员编号不合法")
			return
		}
	}
	now := time.Now().In(config.AttendanceLocation)
	result, err := repository.GenerateWeekAttendance(c.Request.Context(), input.UserIDs, now)
	if err != nil {
		if errors.Is(err, repository.ErrAttendanceConfig) {
			response.Fail(c, http.StatusConflict, err.Error())
			return
		}
		attendanceFailure(c, "生成本周签到表失败", err)
		return
	}
	response.Ok(c, result)
}

func attendanceFailure(c *gin.Context, msg string, err error) {
	if errors.Is(err, repository.ErrAttendanceSchema) {
		response.Fail(c, http.StatusServiceUnavailable, "签到表结构尚未升级，请执行后端的 docs/sql/init.sql")
		return
	}
	response.FailErr(c, http.StatusInternalServerError, msg, err)
}

func RegisterCheckInRecordRoutes(rg *gin.RouterGroup) {
	admin := middleware.RequireRole(model.RoleAdmin)
	member := middleware.RequireRole(model.RoleMember)
	rg.GET("/", admin, ListRecord)
	rg.GET("/week", member, weekAttendance)
	rg.POST("/:id/checkin", member, checkIn)
	rg.PUT("/record", admin, generateWeekAttendance)
}
