package middleware

import (
	"StudioAttendance_backend/internal/response"
	"StudioAttendance_backend/internal/util"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	CtxUserID   = "user_id"
	CtxUsername = "username"
	CtxRole     = "role"
)

func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		const prefix = "Bearer "

		auth := c.GetHeader("Authorization")
		if auth == "" {
			response.Abort(c, http.StatusUnauthorized, "请先登录")
			return
		}
		if !strings.HasPrefix(auth, prefix) {
			response.Abort(c, http.StatusUnauthorized, "Authorization 格式应为 Bearer <token>")
			return
		}

		tokenStr := strings.TrimSpace(strings.TrimPrefix(auth, prefix))
		claims, err := util.ParseToken(tokenStr)
		if err != nil {
			response.Abort(c, http.StatusUnauthorized, "登录已过期,请重新登录")
			return
		}

		c.Set(CtxUserID, claims.UserID)
		c.Set(CtxUsername, claims.Username)
		c.Set(CtxRole, claims.Role)

		c.Next()
	}
}

// CurrentUserID 从上下文取出当前登录用户的 ID。
// 必须在 JWTAuth 中间件之后的 handler 里调用,否则返回错误而不是静默的 0。
func CurrentUserID(c *gin.Context) (int, error) {
	v, exists := c.Get(CtxUserID)
	if !exists {
		return 0, errors.New("用户未登录")
	}
	id, ok := v.(int)
	if !ok {
		return 0, errors.New("用户身份类型异常")
	}
	return id, nil
}
