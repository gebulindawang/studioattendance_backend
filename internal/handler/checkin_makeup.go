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
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const maxMakeupText = 500

func memberAttendance(c *gin.Context) {
	now := time.Now().In(config.AttendanceLocation)
	month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	if raw := c.Query("month"); raw != "" {
		var err error
		month, err = time.ParseInLocation("2006-01", raw, now.Location())
		if err != nil || month.Year() < 1000 || month.Year() >= 9999 {
			response.Fail(c, http.StatusBadRequest, "月份格式应为 YYYY-MM")
			return
		}
	}
	userID, _ := middleware.CurrentUserID(c)
	result, err := repository.GetMemberAttendance(c.Request.Context(), userID, month, now)
	if err != nil {
		attendanceFailure(c, "读取个人考勤失败", err)
		return
	}
	response.Ok(c, result)
}

func submitMakeup(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, strconv.IntSize)
	if err != nil || id == 0 {
		response.Fail(c, http.StatusBadRequest, "签到记录编号不合法")
		return
	}
	var input struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Fail(c, http.StatusBadRequest, "补签申请格式错误")
		return
	}
	reason := strings.TrimSpace(input.Reason)
	if reason == "" || utf8.RuneCountInString(reason) > maxMakeupText {
		response.Fail(c, http.StatusBadRequest, "请填写补签原因，最多 500 字")
		return
	}
	userID, _ := middleware.CurrentUserID(c)
	request, err := repository.SubmitCheckinMakeup(c.Request.Context(), userID, uint(id), reason, time.Now().In(config.AttendanceLocation))
	if err != nil {
		makeupFailure(c, "提交补签申请失败", err)
		return
	}
	response.Ok(c, request)
}

func listMakeups(c *gin.Context) {
	requests, err := repository.ListCheckinMakeups(c.Request.Context())
	if err != nil {
		attendanceFailure(c, "读取补签申请失败", err)
		return
	}
	response.Ok(c, requests)
}

func reviewMakeup(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, strconv.IntSize)
	if err != nil || id == 0 {
		response.Fail(c, http.StatusBadRequest, "补签申请编号不合法")
		return
	}
	var input struct {
		Decision string `json:"decision"`
		Note     string `json:"note"`
	}
	if err := c.ShouldBindJSON(&input); err != nil ||
		(input.Decision != model.MakeupApproved && input.Decision != model.MakeupRejected) {
		response.Fail(c, http.StatusBadRequest, "请选择通过或驳回")
		return
	}
	note := strings.TrimSpace(input.Note)
	if utf8.RuneCountInString(note) > maxMakeupText || (input.Decision == model.MakeupRejected && note == "") {
		response.Fail(c, http.StatusBadRequest, "审批意见最多 500 字，驳回时须填写原因")
		return
	}
	reviewerID, _ := middleware.CurrentUserID(c)
	if err := repository.ReviewCheckinMakeup(c.Request.Context(), reviewerID, uint(id), input.Decision, note, time.Now().In(config.AttendanceLocation)); err != nil {
		makeupFailure(c, "审批补签申请失败", err)
		return
	}
	response.OkMsg(c, "补签申请已处理")
}

func makeupFailure(c *gin.Context, message string, err error) {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		response.Fail(c, http.StatusNotFound, "签到记录或补签申请不存在")
	case errors.Is(err, repository.ErrMakeupUnavailable), errors.Is(err, repository.ErrMakeupPending),
		errors.Is(err, repository.ErrMakeupReviewed), errors.Is(err, repository.ErrCheckinContinuation),
		errors.Is(err, repository.ErrCheckinPeriodSigned), errors.Is(err, repository.ErrCheckinCourseConflict):
		response.Fail(c, http.StatusConflict, err.Error())
	default:
		attendanceFailure(c, message, err)
	}
}
