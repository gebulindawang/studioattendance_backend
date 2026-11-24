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
	result, err := repository.ListCheckinRecord()
	if err != nil {
		response.FailErr(c, http.StatusInternalServerError, "查询考勤记录失败", err)
		return
	}
	response.Ok(c, gin.H{"date": time.Now().In(config.AttendanceLocation).Format(time.DateOnly), "records": result})
}

func todayAttendance(c *gin.Context) {
	userID, _ := middleware.CurrentUserID(c)
	result, err := repository.GetTodayAttendance(userID, time.Now().In(config.AttendanceLocation))
	if err != nil {
		response.FailErr(c, http.StatusInternalServerError, "查询今日签到失败", err)
		return
	}
	response.Ok(c, result)
}

func checkIn(c *gin.Context) {
	section, err := strconv.Atoi(c.Param("section"))
	if err != nil || section <= 0 {
		response.Fail(c, http.StatusBadRequest, "节次编号不合法")
		return
	}
	userID, _ := middleware.CurrentUserID(c)
	record, err := repository.CheckIn(userID, section, time.Now().In(config.AttendanceLocation))
	switch {
	case err == nil:
		response.Ok(c, record)
	case errors.Is(err, repository.ErrCheckinUnavailable), errors.Is(err, repository.ErrCheckinExempted),
		errors.Is(err, repository.ErrCheckinTooEarly), errors.Is(err, repository.ErrCheckinClosed),
		errors.Is(err, repository.ErrCheckinDuplicate):
		response.Fail(c, http.StatusConflict, err.Error())
	default:
		response.FailErr(c, http.StatusInternalServerError, "签到失败", err)
	}
}

func settleAttendance(c *gin.Context) {
	if err := repository.SettleAttendance(c.Request.Context(), time.Now().In(config.AttendanceLocation)); err != nil {
		response.FailErr(c, http.StatusInternalServerError, "结算考勤失败", err)
		return
	}
	response.OkMsg(c, "今日已结束的节次已结算")
}

func RegisterCheckInRecordRoutes(rg *gin.RouterGroup) {
	admin := middleware.RequireRole(model.RoleAdmin)
	member := middleware.RequireRole(model.RoleMember)
	rg.GET("/", admin, ListRecord)
	rg.GET("/today", member, todayAttendance)
	rg.POST("/checkin/:section", member, checkIn)
	rg.PUT("/record", admin, settleAttendance)
}
