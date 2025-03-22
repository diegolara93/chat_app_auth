package main

import (
	"errors"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

var AuthError = errors.New("auth error")

func Authorize(c echo.Context, db *gorm.DB) error {
	r := c.Request()
	username := r.FormValue("username")
	if db.Where("username = ?", username).First(&User{}).Error != nil {
		return AuthError
	}
	// gets the session token from the cookie
	st, err := r.Cookie("session_token")
	if err != nil || st.Value == "" || db.Where("session_token = ?", st.Value).First(&User{}).Error != nil {
		return AuthError
	}
	// get the csrf token from the headers
	ct := r.Header.Get("X-CSRF-Token")
	if ct == "" || db.Where("csrf_token = ?", ct).First(&User{}).Error != nil {
		return AuthError
	}
	return nil
}
