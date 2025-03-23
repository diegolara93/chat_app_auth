package main

import (
	"errors"
	"log/slog"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		slog.Error("Error loading .env file")
	}

	secretKey := os.Getenv("JWT_SECRET")
	if secretKey != "" {
		jwtSecret = []byte(secretKey)
	} else {
		slog.Warn("JWT_SECRET not found in environment, using default (INSECURE)")
	}

	dsn := os.Getenv("DB_URL")
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		slog.Error("failed to connect database", "error", err)
		return
	}

	db.AutoMigrate(&User{})

	// hub := newHub()
	// go hub.run()

	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// e.GET("/", serveHome)
	// e.GET("/ws", func(c echo.Context) error {
	// 	serveWs(hub, c)
	// 	return nil
	// })
	// e.GET("/signin", serverSignIn)
	e.POST("/register", func(c echo.Context) error { return registerHandler(c, db) })
	e.POST("/login", func(c echo.Context) error { return loginHandler(c, db) })

	// Protected routes
	e.POST("/logout", func(c echo.Context) error { return logoutHandler(c, db) })
	e.POST("/protected", func(c echo.Context) error { return protectedHandler(c, db) })

	// protectedGroup := e.Group("/api")
	// protectedGroup.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
	//     return func(c echo.Context) error {
	//         if err := Authorize(c, db); err != nil {
	//             return c.JSON(http.StatusUnauthorized, map[string]string{
	//                 "error": "Unauthorized",
	//             })
	//         }
	//         return next(c)
	//     }
	// })
	// protectedGroup.GET("/profile", profileHandler)

	if err := e.Start(":8080"); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("failed to start server", "error", err)
	}
}
