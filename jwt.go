package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

var (
	ErrAuth   = errors.New("auth error")
	jwtSecret = []byte("jwt-secret-key")
)

type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

func GenerateJWT(username string) (string, error) {
	expirationTime := time.Now().Add(24 * time.Hour)

	claims := &Claims{
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   username,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func ValidateJWT(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}

func ExtractJWTFromRequest(c echo.Context) string {
	authHeader := c.Request().Header.Get("Authorization")
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		return authHeader[7:]
	}

	cookie, err := c.Cookie("token")
	if err == nil {
		return cookie.Value
	}

	return ""
}

func Authorize(c echo.Context, db *gorm.DB) error {
	tokenString := ExtractJWTFromRequest(c)
	if tokenString == "" {
		return ErrAuth
	}

	claims, err := ValidateJWT(tokenString)
	if err != nil {
		return ErrAuth
	}

	var user User
	if err := db.Where("username = ?", claims.Username).First(&user).Error; err != nil {
		return ErrAuth
	}

	c.Set("username", claims.Username)

	return nil
}

func GetUsername(c echo.Context) string {
	username, ok := c.Get("username").(string)
	if !ok {
		return ""
	}
	return username
}
