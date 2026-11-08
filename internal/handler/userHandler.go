package handler

import (
	"StudioAttendance_backend/internal/middleware"
	"StudioAttendance_backend/internal/model"
	"StudioAttendance_backend/internal/repository"
	"StudioAttendance_backend/internal/response"
	"StudioAttendance_backend/internal/util"
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func Register(c *gin.Context) {
	var user model.User
	if err := c.ShouldBindJSON(&user); err != nil {
		panic(err)
	}
	repository.Register(user)
	response.OkMsg(c, "注册成功")
}

func Login(c *gin.Context) {
	var user model.User
	if err := c.ShouldBindJSON(&user); err != nil {
		response.FailErr(c, http.StatusBadRequest, "数据绑定失败", err)
		return
	}

	select_user, err := repository.FindUserByUsername(user.Username)
	if err != nil {
		response.FailErr(c, http.StatusBadRequest, "查询错误", err)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(select_user.Password), []byte(user.Password)); err != nil {
		response.Fail(c, http.StatusBadRequest, "用户名或密码错误")
		return
	}

	token, err := util.GenToken(user.ID, user.Username, int(user.Role))

	if err != nil {
		response.FailErr(c, http.StatusInternalServerError, "签发token失败", err)
		return
	}

	response.Ok(c, token)
}

func UserInfo(c *gin.Context) {
	username := c.GetString(middleware.CtxUsername)
	u, err := repository.FindUserByUsername(username)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "该用户不存在")
		return
	}
	response.Ok(c, u)
}

func RegisterUserRoutes(rg *gin.RouterGroup) {
	rg.POST("/register", Register)
	rg.POST("/login", Login)
	rg.GET("/me",UserInfo)
}
