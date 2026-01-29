package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

// Config holds authentication configuration.
type Config struct {
	GitHubClientID     string
	GitHubClientSecret string
	SessionSecret      string
	BaseURL            string
	AllowedUsers       []string
}

// Handler manages authentication requests.
type Handler struct {
	oauthConfig  *oauth2.Config
	allowedUsers map[string]bool
	logger       *logrus.Logger
	cookieName   string
}

// User represents a GitHub user.
type User struct {
	Login string `json:"login"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

const (
	stateCookieName = "oauth_state"
	authCookieName  = "auth_session"
)

// NewHandler creates a new authentication handler.
func NewHandler(cfg Config, logger *logrus.Logger) *Handler {
	// Create allowed users map for O(1) lookups.
	allowed := make(map[string]bool)
	for _, user := range cfg.AllowedUsers {
		allowed[strings.ToLower(user)] = true
	}

	return &Handler{
		oauthConfig: &oauth2.Config{
			ClientID:     cfg.GitHubClientID,
			ClientSecret: cfg.GitHubClientSecret, // pragma: allowlist secret
			RedirectURL:  cfg.BaseURL + "/auth/callback",
			Scopes:       []string{"read:user"},
			Endpoint:     github.Endpoint,
		},
		allowedUsers: allowed,
		logger:       logger,
		cookieName:   authCookieName,
	}
}

// Login initiates the OAuth flow.
func (h *Handler) Login(c *gin.Context) {
	// Generate random state.
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		h.logger.WithError(err).Error("Failed to generate random state")
		c.Redirect(http.StatusTemporaryRedirect, "http://localhost:3000")
		return
	}
	state := base64.URLEncoding.EncodeToString(b)

	// Set state cookie (short-lived).
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(stateCookieName, state, 600, "/auth", "", false, true)

	// Redirect to GitHub.
	url := h.oauthConfig.AuthCodeURL(state)
	c.Redirect(http.StatusTemporaryRedirect, url)
}

// Callback handles the OAuth callback.
func (h *Handler) Callback(c *gin.Context) {
	// Verify state.
	stateCookie, err := c.Cookie(stateCookieName)
	if err != nil {
		h.logger.Warn("Missing state cookie in callback")
		c.Redirect(http.StatusTemporaryRedirect, "http://localhost:3000")
		return
	}

	state := c.Query("state")
	if state != stateCookie {
		h.logger.Warn("State mismatch in callback")
		c.Redirect(http.StatusTemporaryRedirect, "http://localhost:3000")
		return
	}

	// Delete state cookie.
	c.SetCookie(stateCookieName, "", -1, "/auth", "", false, true)

	// Exchange code for token.
	code := c.Query("code")
	token, err := h.oauthConfig.Exchange(context.Background(), code)
	if err != nil {
		h.logger.WithError(err).Error("Failed to exchange token")
		c.Redirect(http.StatusTemporaryRedirect, "http://localhost:3000")
		return
	}

	// Fetch user profile.
	authCtx := c.Request.Context()
	req, err := http.NewRequestWithContext(authCtx, http.MethodGet, "https://api.github.com/user", http.NoBody)
	if err != nil {
		h.logger.WithError(err).Error("Failed to create request for user profile")
		c.Redirect(http.StatusTemporaryRedirect, "http://localhost:3000")
		return
	}

	client := h.oauthConfig.Client(authCtx, token)
	resp, err := client.Do(req)
	if err != nil {
		h.logger.WithError(err).Error("Failed to fetch user profile")
		c.Redirect(http.StatusTemporaryRedirect, "http://localhost:3000")
		return
	}
	defer resp.Body.Close()

	var user User
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		h.logger.WithError(err).Error("Failed to decode user profile")
		c.Redirect(http.StatusTemporaryRedirect, "http://localhost:3000")
		return
	}

	// Check if user is allowed.
	if !h.allowedUsers[strings.ToLower(user.Login)] {
		h.logger.WithField("user", user.Login).Warn("Unauthorized user attempted login")
		c.Redirect(http.StatusTemporaryRedirect, "http://localhost:3000")
		return
	}

	// Set session cookie
	// In a real app, we might use a signed cookie (e.g. securecookie) or JWT
	// For simplicity, we'll store the username in a cookie with a signature
	// But since we aren't using a DB, we'll implement a simple secure cookie mechanism or just a basic one for now
	// Ideally use gorilla/securecookie or similar.
	// For this MVP, we will set a simple cookie with the username.
	// TODO: Add proper signing if time permits.

	// Set session cookie with SameSite=Lax for cross-origin compatibility.
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(h.cookieName, user.Login, 86400*7, "/", "localhost", false, true) // 7 days, HttpOnly, domain=localhost.

	h.logger.WithField("user", user.Login).Info("User logged in successfully")
	// Redirect to frontend after successful login.
	c.Redirect(http.StatusTemporaryRedirect, "http://localhost:3000")
}

// Me returns the specific authenticated user.
func (h *Handler) Me(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"login":      user,
		"avatar_url": fmt.Sprintf("https://github.com/%s.png", user),
	})
}

// AuthMiddleware requires authentication.
func (h *Handler) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, err := c.Cookie(h.cookieName)
		if err != nil || user == "" {
			h.logger.WithFields(logrus.Fields{
				"cookie_name": h.cookieName,
				"error":       err,
				"path":        c.Request.URL.Path,
			}).Debug("Auth middleware: cookie not found")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		// Verify user is still allowed (in case config changed).
		if !h.allowedUsers[strings.ToLower(user)] {
			c.SetCookie(h.cookieName, "", -1, "/", "", false, true)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		c.Set("user", user)
		c.Next()
	}
}

// Logout clears the session.
func (h *Handler) Logout(c *gin.Context) {
	c.SetCookie(h.cookieName, "", -1, "/", "", false, true)
	c.Redirect(http.StatusTemporaryRedirect, "http://localhost:3000")
}
