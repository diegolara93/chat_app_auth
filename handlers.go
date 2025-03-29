package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/markbates/goth/gothic"
	"gorm.io/gorm"
)

func loginHandler(c echo.Context, db *gorm.DB) error {
	if c.Request().Method != http.MethodPost {
		return errors.New("not a POST request")
	}

	email := c.FormValue("email")
	password := c.FormValue("password")

	var user User
	if err := db.Where("email = ?", email).First(&user).Error; err != nil {
		return errors.New("email not found")
	}

	if !checkPasswordHash(password, user.HashedPassword) {
		return errors.New("invalid password")
	}

	token, err := GenerateJWT(user.Username)
	if err != nil {
		return errors.New("failed to generate token")
	}

	cookie := new(http.Cookie)
	cookie.Name = "token"
	cookie.Value = token
	cookie.Expires = time.Now().Add(24 * time.Hour)
	cookie.HttpOnly = true
	cookie.Path = "/"
	// for prod
	// cookie.Secure = true
	// cookie.SameSite = http.SameSiteStrictMode
	c.SetCookie(cookie)

	return c.JSON(http.StatusOK, map[string]string{
		"token":    token,
		"username": user.Username,
	})
}

func logoutHandler(c echo.Context, db *gorm.DB) error {
	if err := Authorize(c, db); err != nil {
		return err
	}

	// Clear the token cookie
	cookie := new(http.Cookie)
	cookie.Name = "token"
	cookie.Value = ""
	cookie.Expires = time.Now().Add(-1 * time.Hour)
	cookie.HttpOnly = true
	cookie.Path = "/"
	c.SetCookie(cookie)

	username := GetUsername(c)
	fmt.Printf("User %s logged out\n", username)

	return c.JSON(http.StatusOK, map[string]string{
		"message": "Successfully logged out",
	})
}

// Updated protected handler
func protectedHandler(c echo.Context, db *gorm.DB) error {
	if c.Request().Method != http.MethodPost {
		return errors.New("not a POST request")
	}

	if err := Authorize(c, db); err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "Unauthorized",
		})
	}

	username := GetUsername(c)
	fmt.Printf("JWT validation successful for user %s\n", username)

	return c.JSON(http.StatusOK, map[string]string{
		"message":  "Access granted to protected resource",
		"username": username,
	})
}

func registerHandler(c echo.Context, db *gorm.DB) error {
	if c.Request().Method != http.MethodPost {
		return errors.New("not a POST request")
	}

	username := c.FormValue("username")
	email := c.FormValue("email")
	password := c.FormValue("password")

	if len(username) < 3 || email == "" || len(password) < 8 {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid information",
		})
	}

	var user User

	exists := db.Where("email = ?", email).First(&user)
	if exists.Error == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Email already exists",
		})
	}

	exists = db.Where("username = ?", username).First(&user)
	if exists.Error == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Username already exists",
		})
	}

	hashedPassword, err := hashPassword(password)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to hash password",
		})
	}

	newUser := &User{
		Username:       username,
		Email:          email,
		HashedPassword: hashedPassword,
	}

	if err := db.Create(newUser).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to create user",
		})
	}

	fmt.Println("User created:", username)
	return c.JSON(http.StatusCreated, map[string]string{
		"message": "User created successfully",
	})
}

func incrementMessagesSent(c echo.Context) error {
	// check where the user sent the message and then increment by 1
	// for total messages, and by 1 for the coin they sent the message about
	// also check the # of messages to see if they got an achievement
	return nil
}

func deleteUser(c echo.Context) error {
	return nil
}

func getUserHandler(c echo.Context) error {
	return nil
}

func getUserCoins(c echo.Context, db *gorm.DB) error {
	// returns the users "favorited" coins TO THE MOON! HODL!
	return nil
}

func userMostActiveCoins(e echo.Context, db *gorm.DB) error {
	// returns coins the user has sent the most messages about
	return nil
}

func getUserAchievements(c echo.Context, db *gorm.DB) error {
	// returns the users achievements
	return nil
}

// these are the handlers for oauth
func oAuthCallbackHandler(c echo.Context) error {
	req := c.Request()
	res := c.Response().Writer
	user, err := gothic.CompleteUserAuth(res, req)
	if err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}
	return c.JSON(http.StatusOK, user)
}

func oAuthLogoutHandler(c echo.Context) error {
	gothic.Logout(c.Response(), c.Request())
	return c.Redirect(http.StatusTemporaryRedirect, "/")
}

func oAuthProviderHandler(c echo.Context) error {
	provider := c.Param("provider")
	if provider == "" {
		return c.String(http.StatusBadRequest, "Provider not specified")
	}

	q := c.Request().URL.Query()
	q.Add("provider", c.Param("provider"))
	c.Request().URL.RawQuery = q.Encode()

	req := c.Request()
	res := c.Response().Writer
	if gothUser, err := gothic.CompleteUserAuth(res, req); err == nil {
		return c.JSON(http.StatusOK, gothUser)
	}
	gothic.BeginAuthHandler(res, req)
	return nil
}

func profileHandler(c echo.Context) error {
	username := GetUsername(c)
	return c.JSON(http.StatusOK, map[string]string{
		"username": username,
	})
}

// temporary until I add the frontend
func serverSignIn(c echo.Context) error {
	r := c.Request()
	w := c.Response()
	log.Print(r.URL)
	if r.URL.Path != "/signin" {
		http.Error(w, "404 not found", http.StatusNotFound)
		return nil
	}
	if r.Method != "GET" {
		http.Error(w, "Method not allowed please send a GET request", http.StatusMethodNotAllowed)
		return nil
	}
	http.ServeFile(w, r, "signin.html")
	return nil
}

func serveOathSignUp(c echo.Context) error {
	r := c.Request()
	w := c.Response()
	log.Print(r.URL)
	if r.URL.Path != "/oauthsignup" {
		http.Error(w, "404 not found", http.StatusNotFound)
		return nil
	}
	if r.Method != "GET" {
		http.Error(w, "Method not allowed please send a GET request", http.StatusMethodNotAllowed)
		return nil
	}
	http.ServeFile(w, r, "oauth.html")
	return nil
}

func serveHome(c echo.Context) error {
	return c.File("templates/home.html")
}

// handlers for chat room related stuff

func generateRoomID() string {
	bytes := make([]byte, 4)
	if _, err := rand.Read(bytes); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(bytes)
}

func createRoomHandler(c echo.Context, db *gorm.DB) error {

	if err := Authorize(c, db); err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "You must be logged in to create a room",
		})
	}

	username := GetUsername(c)

	var user User
	if err := db.Where("username = ?", username).First(&user).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "User not found",
		})
	}

	roomName := c.FormValue("name")
	if roomName == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Room name is required",
		})
	}

	maxParticipants := 10
	if maxStr := c.FormValue("max_participants"); maxStr != "" {
		if max, err := strconv.Atoi(maxStr); err == nil {
			if max < 1 {
				maxParticipants = 1
			} else if max > 10 {
				maxParticipants = 10
			} else {
				maxParticipants = max
			}
		}
	}

	password := c.FormValue("password")
	hasPassword := password != ""

	hashedPassword := ""
	if hasPassword {
		var err error
		hashedPassword, err = hashPassword(password)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "Failed to secure room password",
			})
		}
	}

	roomID := generateRoomID()

	room := &ChatRoom{
		Name:            roomName,
		OwnerID:         user.ID,
		Password:        hashedPassword,
		HasPassword:     hasPassword,
		MaxParticipants: maxParticipants,
		RoomID:          roomID,
	}

	if err := db.Create(room).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to create room",
		})
	}

	participant := &RoomParticipant{
		RoomID:     room.ID,
		UserID:     user.ID,
		JoinedAt:   time.Now(),
		IsActive:   true,
		LastActive: time.Now(),
	}

	if err := db.Create(participant).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to add creator to room",
		})
	}

	return c.JSON(http.StatusCreated, map[string]interface{}{
		"message":  "Room created successfully",
		"room_id":  roomID,
		"name":     roomName,
		"password": hasPassword,
	})
}

func listRoomsHandler(c echo.Context, db *gorm.DB) error {
	var rooms []ChatRoom

	if err := db.Select("id, name, has_password, max_participants, room_id, owner_id, created_at").
		Find(&rooms).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to fetch rooms",
		})
	}

	return c.JSON(http.StatusOK, rooms)
}

func joinRoomHandler(c echo.Context, db *gorm.DB) error {
	roomID := c.Param("roomID")
	if roomID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Room ID is required",
		})
	}

	var room ChatRoom
	if err := db.Where("room_id = ?", roomID).First(&room).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "Room not found",
		})
	}

	if room.HasPassword {
		password := c.FormValue("password")
		if !checkPasswordHash(password, room.Password) {
			return c.JSON(http.StatusUnauthorized, map[string]string{
				"error": "Invalid room password",
			})
		}
	}

	if err := Authorize(c, db); err != nil {
		if !room.HasPassword {

			return c.JSON(http.StatusOK, map[string]interface{}{
				"room_id": room.RoomID,
				"name":    room.Name,
				"guest":   true,
			})
		}

		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "You must be logged in to join this room",
		})
	}

	username := GetUsername(c)
	var user User
	if err := db.Where("username = ?", username).First(&user).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "User not found",
		})
	}

	var participantCount int64
	db.Model(&RoomParticipant{}).Where("room_id = ? AND is_active = ?", room.ID, true).Count(&participantCount)
	if int(participantCount) >= room.MaxParticipants {
		return c.JSON(http.StatusForbidden, map[string]string{
			"error": "Room is full",
		})
	}

	var participant RoomParticipant
	result := db.Where("room_id = ? AND user_id = ?", room.ID, user.ID).First(&participant)

	if result.Error != nil {

		participant = RoomParticipant{
			RoomID:     room.ID,
			UserID:     user.ID,
			JoinedAt:   time.Now(),
			IsActive:   true,
			LastActive: time.Now(),
		}
		if err := db.Create(&participant).Error; err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "Failed to join room",
			})
		}
	} else {

		db.Model(&participant).Updates(map[string]interface{}{
			"is_active":   true,
			"last_active": time.Now(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"room_id":   room.RoomID,
		"name":      room.Name,
		"joined_at": participant.JoinedAt,
	})
}

func leaveRoomHandler(c echo.Context, db *gorm.DB) error {

	roomID := c.Param("roomID")
	if roomID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Room ID is required",
		})
	}

	if err := Authorize(c, db); err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "You must be logged in to leave a room",
		})
	}

	username := GetUsername(c)
	var user User
	if err := db.Where("username = ?", username).First(&user).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "User not found",
		})
	}

	var room ChatRoom
	if err := db.Where("room_id = ?", roomID).First(&room).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "Room not found",
		})
	}

	result := db.Model(&RoomParticipant{}).
		Where("room_id = ? AND user_id = ?", room.ID, user.ID).
		Update("is_active", false)

	if result.RowsAffected == 0 {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "You are not allowed in this room",
		})
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "Successfully left the room",
	})
}

func getRoomInfoHandler(c echo.Context, db *gorm.DB) error {
	roomID := c.Param("roomID")
	if roomID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Room ID is required",
		})
	}

	var room ChatRoom
	if err := db.Where("room_id = ?", roomID).First(&room).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "Room not found",
		})
	}

	var participantCount int64
	db.Model(&RoomParticipant{}).Where("room_id = ? AND is_active = ?", room.ID, true).Count(&participantCount)

	room.Password = ""

	return c.JSON(http.StatusOK, map[string]interface{}{
		"room":            room,
		"active_users":    participantCount,
		"available_slots": room.MaxParticipants - int(participantCount),
	})
}
