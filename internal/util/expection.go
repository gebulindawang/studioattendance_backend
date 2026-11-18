package util

import (
	"StudioAttendance_backend/internal/response"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

func Ensure(err error) error {
	log.Fatalln(err)
	return err
}

func BindingModelException(err error, c *gin.Context) error {
	response.FailErr(c, http.StatusBadRequest, "数据传递失败", err)
	return err
}
