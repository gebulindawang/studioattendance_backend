package handler

import (
	"StudioAttendance_backend/internal/middleware"
	"StudioAttendance_backend/internal/model"
	"StudioAttendance_backend/internal/repository"
	"StudioAttendance_backend/internal/response"
	"StudioAttendance_backend/internal/util"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func Register(c *gin.Context) {
	var input struct {
		credentials
		Nickname string `json:"nickname"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Fail(c, http.StatusBadRequest, "注册数据格式错误")
		return
	}
	username, nickname := strings.TrimSpace(input.Username), strings.TrimSpace(input.Nickname)
	if username == "" || nickname == "" || utf8.RuneCountInString(username) > 64 ||
		utf8.RuneCountInString(nickname) > 64 || utf8.RuneCountInString(input.Password) < 6 || len(input.Password) > 72 {
		response.Fail(c, http.StatusBadRequest, "请检查用户名、昵称及密码长度（密码至少 6 字符且最多 72 字节）")
		return
	}
	if err := repository.Register(model.User{Username: username, Password: input.Password, Nickname: nickname}); err != nil {
		var dbErr *mysql.MySQLError
		if errors.As(err, &dbErr) && dbErr.Number == 1062 {
			response.Fail(c, http.StatusConflict, "用户名已存在")
			return
		}
		response.FailErr(c, http.StatusInternalServerError, "注册失败，请重试", err)
		return
	}
	response.OkMsg(c, "注册成功")
}

func Login(c *gin.Context) {
	var user credentials
	if err := c.ShouldBindJSON(&user); err != nil {
		response.FailErr(c, http.StatusBadRequest, "数据绑定失败", err)
		return
	}

	account, err := repository.FindUserByUsername(strings.TrimSpace(user.Username))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		response.Fail(c, http.StatusUnauthorized, "用户名或密码错误")
		return
	}
	if err != nil {
		response.FailErr(c, http.StatusInternalServerError, "登录失败，请重试", err)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(account.Password), []byte(user.Password)); err != nil {
		response.Fail(c, http.StatusUnauthorized, "用户名或密码错误")
		return
	}
	if account.Status != model.StatusActive {
		response.Fail(c, http.StatusForbidden, "账号已禁用，请联系管理员")
		return
	}

	token, err := util.GenToken(account.ID, account.Username, int(account.Role))

	if err != nil {
		response.FailErr(c, http.StatusInternalServerError, "签发token失败", err)
		return
	}

	response.Ok(c, token)
}

func UserInfo(c *gin.Context) {
	userID, err := middleware.CurrentUserID(c)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, "登陆状态失效,请重新登录")
		return
	}
	u, err := repository.FindUserByID(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, http.StatusNotFound, "该用户不存在")
			return
		}
		response.FailErr(c, http.StatusInternalServerError, "查询用户失败", err)
		return
	}
	response.Ok(c, u)
}

func ListUsers(c *gin.Context) {
	u, err := repository.ListUsers()
	if err != nil {
		response.FailErr(c, http.StatusBadRequest, "查询失败", err)
		return
	}
	response.Ok(c, u)
}

func UpdateMemberStatus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	var input struct {
		Status *int8 `json:"status"`
	}
	if err != nil || id <= 0 || c.ShouldBindJSON(&input) != nil || input.Status == nil ||
		(*input.Status != model.StatusActive && *input.Status != model.StatusDisabled) {
		response.Fail(c, http.StatusBadRequest, "成员 ID 或状态不合法")
		return
	}
	adminID, _ := middleware.CurrentUserID(c)
	err = repository.UpdateMemberStatus(id, *input.Status, adminID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		response.Fail(c, http.StatusNotFound, "成员不存在")
		return
	}
	if err != nil {
		response.FailErr(c, http.StatusInternalServerError, "更新成员状态失败", err)
		return
	}
	response.OkMsg(c, "成员状态已更新")
}

func RegisterUserRoutes(rg *gin.RouterGroup, auth gin.HandlerFunc) {
	rg.POST("/register", Register)
	rg.POST("/login", Login)
	rg.GET("/me", auth, UserInfo)
	admin := middleware.RequireRole(model.RoleAdmin)
	rg.GET("/", auth, admin, ListUsers)
	rg.PATCH("/:id/status", auth, admin, UpdateMemberStatus)
}
