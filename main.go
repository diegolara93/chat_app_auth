package main

import (
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/labstack/echo-contrib/echoprometheus"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/time/rate"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	NewAuth()
	hub := newHub()
	go hub.run()
	err := godotenv.Load()
	if err != nil {
		slog.Error("Error loading .env file")
	}

	secretKey := os.Getenv("JWT_SECRET")
	if secretKey != "" {
		jwtSecret = []byte(secretKey)
	} else {
		slog.Warn("JWT_SECRET not found in env")
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
	e.Use(middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(rate.Limit(20)))) // Limits to 20 requests per second per IP
	e.Use(echoprometheus.NewMiddleware("chat_logging"))                                 // prometheus logging

	go func() {
		metrics := echo.New()                                // this Echo will run on separate port 8081
		metrics.GET("/metrics", echoprometheus.NewHandler()) // adds route to serve gathered metrics
		if err := metrics.Start(":8081"); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	e.GET("/", serveHome)
	e.GET("/ws", func(c echo.Context) error {
		serveWs(hub, c, db)
		return nil
	})
	// handlers for regular auth
	e.GET("/signin", serverSignIn)         // temporary
	e.GET("/oauthsignup", serveOathSignUp) // temporary
	e.POST("/register", func(c echo.Context) error { return registerHandler(c, db) })
	e.POST("/login", func(c echo.Context) error { return loginHandler(c, db) })
	// handlers for oauth login
	e.GET("/auth/:provider", func(c echo.Context) error { return oAuthProviderHandler(c) })

	e.GET("/auth/:provider/callback", func(c echo.Context) error {
		return oAuthCallbackHandler(c)
	})

	e.GET("/auth/logout", func(c echo.Context) error {
		return oAuthLogoutHandler(c)
	})

	// Protected routes
	e.POST("/logout", func(c echo.Context) error { return logoutHandler(c, db) })
	e.POST("/protected", func(c echo.Context) error { return protectedHandler(c, db) })

	protectedGroup := e.Group("/api")
	protectedGroup.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if err := Authorize(c, db); err != nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error": "Unauthorized",
				})
			}
			return next(c)
		}
	})
	protectedGroup.GET("/profile", profileHandler)
	protectedGroup.GET("/user/:username", func(c echo.Context) error {
		return getUserHandler(c)
	})

	if err := e.Start(":8080"); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("failed to start server", "error", err)
	}
}
