package handler

import (
	"StudioAttendance_backend/internal/middleware"
	"StudioAttendance_backend/internal/model"
	"StudioAttendance_backend/internal/repository"
	"StudioAttendance_backend/internal/response"
	"StudioAttendance_backend/internal/util"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func Register(c *gin.Context) {
	var user model.User
	if err := c.ShouldBindJSON(&user); err != nil {
		panic(err)
	}
	err := repository.Register(user)
	if err != nil{
		response.Fail(c,http.StatusBadRequest,"注册失败，请重试")
		return
	}
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

	token, err := util.GenToken(select_user.ID, select_user.Username, int(select_user.Role))

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

func RegisterUserRoutes(rg *gin.RouterGroup, auth gin.HandlerFunc) {
	rg.POST("/register", Register)
	rg.POST("/login", Login)
	rg.GET("/me", auth, UserInfo)
}
