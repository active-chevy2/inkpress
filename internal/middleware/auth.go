package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"inkpress/internal/config"
	"inkpress/internal/models"
)

type contextKey string

const UserKey contextKey = "user"
const csrfCookieName = "inkpress_csrf"

type Store interface {
	GetUserBySession(token string) (*models.User, error)
	CreateSession(token string, userID int, expires time.Time) error
	DeleteSession(token string) error
}

type AuthMiddleware struct {
	cfg   *config.Config
	store Store
}

func NewAuthMiddleware(cfg *config.Config, store Store) *AuthMiddleware {
	return &AuthMiddleware{cfg: cfg, store: store}
}

func generateToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (a *AuthMiddleware) LoginUser(w http.ResponseWriter, userID int) error {
	token := generateToken()
	expires := time.Now().Add(7 * 24 * time.Hour)
	if err := a.store.CreateSession(token, userID, expires); err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "inkpress_session",
		Value:    token,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   7 * 24 * 60 * 60,
	})
	return nil
}

func (a *AuthMiddleware) LogoutUser(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("inkpress_session")
	if err == nil && cookie.Value != "" {
		_ = a.store.DeleteSession(cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "inkpress_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func (a *AuthMiddleware) CurrentUser(r *http.Request) *models.User {
	cookie, err := r.Cookie("inkpress_session")
	if err != nil || cookie.Value == "" {
		return nil
	}
	user, err := a.store.GetUserBySession(cookie.Value)
	if err != nil {
		return nil
	}
	return user
}

func (a *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := a.CurrentUser(r)
		if user == nil {
			http.Redirect(w, r, "/admin/login", http.StatusFound)
			return
		}
		r = setUserContext(r, user)
		next.ServeHTTP(w, r)
	})
}

func (a *AuthMiddleware) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := a.CurrentUser(r)
		if user == nil {
			http.Redirect(w, r, "/admin/login", http.StatusFound)
			return
		}
		if !user.IsAdmin() {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		r = setUserContext(r, user)
		next.ServeHTTP(w, r)
	})
}

func (a *AuthMiddleware) OptionalAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := a.CurrentUser(r)
		if user != nil {
			r = setUserContext(r, user)
		}
		next.ServeHTTP(w, r)
	})
}

// GenerateCSRFToken sets a CSRF cookie and returns the token value.
func (a *AuthMiddleware) GenerateCSRFToken(w http.ResponseWriter, r *http.Request) string {
	cookie, err := r.Cookie(csrfCookieName)
	var token string
	if err == nil && cookie.Value != "" {
		token = cookie.Value
	} else {
		token = generateToken()
	}
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400, // 1 day
	})
	return token
}

// ValidateCSRFToken compares the cookie token with the form value.
func (a *AuthMiddleware) ValidateCSRFToken(r *http.Request) bool {
	cookie, err := r.Cookie(csrfCookieName)
	if err != nil {
		return false
	}
	formToken := r.FormValue("csrf_token")
	if formToken == "" {
		return false
	}
	return cookie.Value == formToken
}

func setUserContext(r *http.Request, user *models.User) *http.Request {
	return r.WithContext(contextWithUser(r.Context(), user))
}

func GetUser(r *http.Request) *models.User {
	if v := r.Context().Value(UserKey); v != nil {
		return v.(*models.User)
	}
	return nil
}
