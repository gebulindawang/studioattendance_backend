package util

import (
	"StudioAttendance_backend/internal/config"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const tokenIssuer = "studioattendance"

func signingKey() ([]byte, error) {
	if len(config.JWTSecret) < 32 {
		return nil, errors.New("JWT_SECRET 至少需要 32 字节")
	}
	return []byte(config.JWTSecret), nil
}

type MyClaims struct {
	UserID   int
	Username string
	Role     int
	jwt.RegisteredClaims
}

func GenToken(userID int, username string, role int) (string, error) {
	key, err := signingKey()
	if err != nil {
		return "", err
	}
	now := time.Now()
	claims := MyClaims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(2 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    tokenIssuer,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(key)
}

func ParseToken(tokenStr string) (*MyClaims, error) {
	key, err := signingKey()
	if err != nil {
		return nil, err
	}
	claims := &MyClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(*jwt.Token) (any, error) {
		return key, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(tokenIssuer), jwt.WithExpirationRequired())
	if err != nil {
		return nil, err
	}
	if !token.Valid || claims.UserID <= 0 {
		return nil, errors.New("token 无效")
	}
	return claims, nil
}
