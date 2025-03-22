package main

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	HashedPassword string
	SessionToken   string
	CSRFToken      string
	Username       string
	Email          string
}

func main() {
	err := godotenv.Load()
	if err != nil {
		slog.Error("Error loading .env file")
	}
	dsn := os.Getenv("DB_URL")

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})

	if err != nil {
		slog.Error("failed to connect database", "error", err)
		return
	}

	db.AutoMigrate(&User{})

	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	e.POST("/register", func(c echo.Context) error { return registerHandler(c, db) })
	e.POST("/login", func(c echo.Context) error { return loginHandler(c, db) })
	e.POST("/logout", func(c echo.Context) error { return logoutHandler(c, db) })
	e.POST("/protected", func(c echo.Context) error { return protectedHandler(c, db) })

	if err := e.Start(":8080"); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("failed to start server", "error", err)
	}
}

func registerHandler(c echo.Context, db *gorm.DB) error {
	if c.Request().Method != http.MethodPost {
		return errors.New("not a POST request")
	}
	username := c.FormValue("username")
	email := c.FormValue("email")
	password := c.FormValue("password")

	if len(username) < 3 || email == "" || len(password) < 8 {
		return errors.New("invalid info")
	}

	// var user User

	// if err := db.Find(&user, "username = ?", username).Error; err == nil {
	// 	return errors.New("username already exists")
	// }
	// if err := db.Find(&user, "email = ?", email).Error; err == nil {
	// 	return errors.New("email already exists")
	// }
	hashedPassword, err := hashPassword(password)
	if err != nil {
		return errors.New("failed to hash password")
	}
	newUser := &User{
		Username:       username,
		Email:          email,
		HashedPassword: hashedPassword,
	}
	if err := db.Create(newUser).Error; err != nil {

	}
	fmt.Println("user created")
	return nil
}

func loginHandler(c echo.Context, db *gorm.DB) error {
	if c.Request().Method != http.MethodPost {
		return errors.New("not a POST request")
	}
	email := c.FormValue("email")
	password := c.FormValue("password")
	var user User
	// if err := db.Find(&user, "email = ?", email).Error; err == nil {
	// 	return errors.New("email already exists")
	// }
	db.Find(&user, "email = ?", email)
	if !checkPasswordHash(password, user.HashedPassword) {
		return errors.New("invalid password")
	}

	sessionToken := generateToken(32)
	csrfToken := generateToken(32)

	http.SetCookie(c.Response(), &http.Cookie{
		Name:     "session_token",
		Value:    sessionToken,
		Expires:  time.Now().Add(24 * time.Hour),
		HttpOnly: true,
	})

	http.SetCookie(c.Response(), &http.Cookie{
		Name:     "csrf_token",
		Value:    csrfToken,
		Expires:  time.Now().Add(24 * time.Hour),
		HttpOnly: false,
	})

	db.Model(&user).Update("session_token", sessionToken)
	db.Model(&user).Update("csrf_token", csrfToken)

	return nil
}

func logoutHandler(c echo.Context, db *gorm.DB) error {
	if err := Authorize(c, db); err != nil {
		return err
	}

	// clear cookie
	http.SetCookie(c.Response(), &http.Cookie{
		Name:     "session_token",
		Value:    "",
		Expires:  time.Now().Add(-1 * time.Hour),
		HttpOnly: true,
	})
	http.SetCookie(c.Response(), &http.Cookie{
		Name:     "csrf_token",
		Value:    "",
		Expires:  time.Now().Add(-1 * time.Hour),
		HttpOnly: false,
	})
	// clear session token and csrf token from db
	username := c.FormValue("username")
	var user User
	db.Where("username = ?", username).First(&user)
	db.Model(&user).Update("session_token", "")
	db.Model(&user).Update("csrf_token", "")
	fmt.Printf("user %s logged out\n", username)
	return nil
}

func protectedHandler(c echo.Context, db *gorm.DB) error {
	if c.Request().Method != http.MethodPost {
		return errors.New("not a POST request")
	}
	if err := Authorize(c, db); err != nil {
		return err
	}

	username := c.FormValue("username")
	fmt.Printf("CSRF validation successful for user %s\n", username)

	return nil
}
