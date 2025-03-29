package main

import (
	"errors"
	"html/template"
	"io"
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

// TemplateRenderer is a custom renderer for HTML templates
type TemplateRenderer struct {
	templates *template.Template
}

// Render renders a template document
func (t *TemplateRenderer) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

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

	// Migrate all models
	db.AutoMigrate(&User{}, &ChatRoom{}, &RoomParticipant{})

	e := echo.New()

	// Initialize templates
	t := &TemplateRenderer{
		templates: template.Must(template.ParseGlob("templates/*.html")),
	}
	e.Renderer = t

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

	// Serve static files for the new room UI
	e.Static("/static", "static")
	e.Static("/templates", "templates")

	// Chat and WebSockets
	e.GET("/", serveHome)
	e.GET("/ws", func(c echo.Context) error {
		serveWs(hub, c, db)
		return nil
	})

	// Basic HTML pages
	e.GET("/signin", serverSignIn)         // temporary
	e.GET("/oauthsignup", serveOathSignUp) // temporary

	// Auth routes
	e.POST("/register", func(c echo.Context) error { return registerHandler(c, db) })
	e.POST("/login", func(c echo.Context) error { return loginHandler(c, db) })
	e.POST("/logout", func(c echo.Context) error { return logoutHandler(c, db) })
	e.POST("/protected", func(c echo.Context) error { return protectedHandler(c, db) })

	// OAuth routes
	e.GET("/auth/:provider", func(c echo.Context) error { return oAuthProviderHandler(c) })
	e.GET("/auth/:provider/callback", func(c echo.Context) error {
		return oAuthCallbackHandler(c)
	})
	e.GET("/auth/logout", func(c echo.Context) error {
		return oAuthLogoutHandler(c)
	})

	// Chat room routes
	e.GET("/rooms", func(c echo.Context) error {
		return listRoomsHandler(c, db)
	})
	e.GET("/rooms/:roomID", func(c echo.Context) error {
		return getRoomInfoHandler(c, db)
	})
	e.POST("/rooms", func(c echo.Context) error {
		return createRoomHandler(c, db)
	})
	e.POST("/rooms/:roomID/join", func(c echo.Context) error {
		return joinRoomHandler(c, db)
	})
	e.POST("/rooms/:roomID/leave", func(c echo.Context) error {
		return leaveRoomHandler(c, db)
	})

	// Create a URL to view a specific room
	e.GET("/chat/:roomID", func(c echo.Context) error {
		// Get the room ID from the URL parameter
		roomID := c.Param("roomID")

		// Render the template with the room ID
		return c.Render(http.StatusOK, "chat_room.html", map[string]interface{}{
			"RoomID": roomID,
		})
	})

	// Create a URL for the room creation page
	e.GET("/create-room", func(c echo.Context) error {
		// Serve the room creation page HTML
		return c.File("templates/create_room.html")
	})

	// Protected API routes
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
	protectedGroup.GET("/my-rooms", func(c echo.Context) error {
		return getUserRoomsHandler(c, db)
	})

	if err := e.Start(":8080"); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("failed to start server", "error", err)
	}
}

// getUserRoomsHandler retrieves all rooms a user is a participant in
func getUserRoomsHandler(c echo.Context, db *gorm.DB) error {
	username := GetUsername(c)
	var user User
	if err := db.Where("username = ?", username).First(&user).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "User not found",
		})
	}

	var rooms []ChatRoom
	if err := db.Raw(`
		SELECT r.* FROM chat_rooms r
		JOIN room_participants p ON r.id = p.room_id
		WHERE p.user_id = ?
	`, user.ID).Scan(&rooms).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to retrieve rooms",
		})
	}

	return c.JSON(http.StatusOK, rooms)
}
