package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

const sessionCookie = "hdu_ride_session"

type contextKey string

const userContextKey contextKey = "user"

func currentUser(c *gin.Context) User {
	user, _ := c.Request.Context().Value(userContextKey).(User)
	return user
}

func requireSession(db *pgxpool.Pool, cfg Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, err := c.Cookie(sessionCookie)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "login required"})
			return
		}

		var user User
		err = db.QueryRow(c.Request.Context(), `
select u.id, u.username, u.display_name, u.role, u.status, u.created_at
from sessions s join users u on u.id = s.user_id
where s.token_hash=$1 and s.expires_at > now() and u.status='active'
`, hashToken(cfg, raw)).Scan(&user.ID, &user.Username, &user.DisplayName, &user.Role, &user.Status, &user.CreatedAt)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "login required"})
			return
		}

		ctx := context.WithValue(c.Request.Context(), userContextKey, user)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func createSession(ctx context.Context, db *pgxpool.Pool, cfg Config, userID string) (string, error) {
	token, err := randomToken()
	if err != nil {
		return "", err
	}
	_, err = db.Exec(ctx, `insert into sessions (token_hash, user_id, expires_at) values ($1,$2,$3)`,
		hashToken(cfg, token), userID, time.Now().Add(cfg.SessionTTL))
	return token, err
}

func deleteSession(ctx context.Context, db *pgxpool.Pool, cfg Config, token string) {
	_, _ = db.Exec(ctx, `delete from sessions where token_hash=$1`, hashToken(cfg, token))
}

func setSessionCookie(c *gin.Context, cfg Config, token string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(cfg.SessionTTL.Seconds()),
	})
}

func clearSessionCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

func randomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func hashToken(cfg Config, raw string) string {
	sum := sha256.Sum256([]byte(cfg.SessionSecret + ":" + raw))
	return hex.EncodeToString(sum[:])
}

func checkPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func hashPassword(password string) (string, error) {
	out, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(out), err
}

func HashPassword(password string) (string, error) {
	return hashPassword(password)
}

func isAdmin(user User) bool {
	return user.Role == RoleRoot || user.Role == RoleAdmin
}

func canCreateClass(user User) bool {
	return isAdmin(user) || user.Role == RoleTeacher
}

func canAccessClass(ctx context.Context, db *pgxpool.Pool, user User, classID string) (bool, error) {
	if isAdmin(user) {
		return true, nil
	}

	var exists bool
	switch user.Role {
	case RoleTeacher:
		err := db.QueryRow(ctx, `select exists(select 1 from classes where id=$1 and created_by=$2)`, classID, user.ID).Scan(&exists)
		return exists, err
	case RoleAssistant, RoleStudent:
		err := db.QueryRow(ctx, `select exists(select 1 from class_members where class_id=$1 and user_id=$2)`, classID, user.ID).Scan(&exists)
		return exists, err
	default:
		return false, nil
	}
}

func canGradeClass(ctx context.Context, db *pgxpool.Pool, user User, classID string) (bool, error) {
	if isAdmin(user) {
		return true, nil
	}
	if user.Role == RoleTeacher {
		var exists bool
		err := db.QueryRow(ctx, `select exists(select 1 from classes where id=$1 and created_by=$2)`, classID, user.ID).Scan(&exists)
		return exists, err
	}
	if user.Role == RoleAssistant {
		var exists bool
		err := db.QueryRow(ctx, `select exists(select 1 from class_members where class_id=$1 and user_id=$2 and member_role='assistant')`, classID, user.ID).Scan(&exists)
		return exists, err
	}
	return false, nil
}

func classCourse(ctx context.Context, db *pgxpool.Pool, classID string) (string, error) {
	var courseID string
	err := db.QueryRow(ctx, `select course_id from classes where id=$1`, classID).Scan(&courseID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	return courseID, err
}
