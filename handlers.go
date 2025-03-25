package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
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
	return nil
}

func deleteUser(c echo.Context) error {
	return nil
}

func getUserHandler(c echo.Context) error {
	return nil
}

func getUserSongsHandler(c echo.Context) error {
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
	r := c.Request()
	w := c.Response()
	log.Print(r.URL)
	if r.URL.Path != "/" {
		http.Error(w, "404 not found", http.StatusNotFound)
		return nil
	}
	if r.Method != "GET" {
		http.Error(w, "Method not allowed please send a GET request", http.StatusMethodNotAllowed)
		return nil
	}
	http.ServeFile(w, r, "index.html")
	return nil
}
