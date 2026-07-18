// Package session implements opaque, signed session cookies backed by the
// sessions table. The signing secret is generated once by main.go and
// persisted in the settings table (not env-configured).
package session

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"time"

	"loxbak/internal/db"
)

// CookieName is the name of the HttpOnly cookie carrying the signed
// session token.
const CookieName = "loxbak_session"

// TTL is how long a session remains valid after creation.
const TTL = 30 * 24 * time.Hour

// ErrInvalid is returned when a cookie is missing, malformed, has a bad
// signature, or refers to an expired/nonexistent session.
var ErrInvalid = errors.New("session: invalid or expired session")

// GenerateSecret returns a new random 32-byte secret, hex-encoded, suitable
// for persisting in the settings table and used to HMAC-sign session
// tokens.
func GenerateSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("session: generate secret: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// sign returns the hex-encoded HMAC-SHA256 of id using secret.
func sign(secret, id string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(id))
	return hex.EncodeToString(mac.Sum(nil))
}

// newID returns a new random, URL-safe opaque session id.
func newID() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("session: generate id: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// Create starts a new session: inserts a sessions row and returns the
// signed cookie value to set (id.signature).
func Create(sqlDB *sql.DB, secret string) (string, error) {
	id, err := newID()
	if err != nil {
		return "", err
	}

	expiresAt := time.Now().Add(TTL)
	if err := db.CreateSession(sqlDB, id, expiresAt); err != nil {
		return "", fmt.Errorf("session: create: %w", err)
	}

	return id + "." + sign(secret, id), nil
}

// SetCookie sets the signed session cookie on the response.
func SetCookie(w http.ResponseWriter, value string) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(TTL.Seconds()),
	})
}

// ClearCookie removes the session cookie from the client.
func ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// splitToken splits a cookie value of the form "id.signature".
func splitToken(value string) (id, signature string, ok bool) {
	for i := len(value) - 1; i >= 0; i-- {
		if value[i] == '.' {
			return value[:i], value[i+1:], true
		}
	}
	return "", "", false
}

// Validate reads the session cookie from r, verifies its HMAC signature
// against secret, and looks up the session in the DB, returning
// ErrInvalid if anything about it is wrong or it has expired.
func Validate(r *http.Request, sqlDB *sql.DB, secret string) (*db.Session, error) {
	cookie, err := r.Cookie(CookieName)
	if err != nil {
		return nil, ErrInvalid
	}

	id, signature, ok := splitToken(cookie.Value)
	if !ok || id == "" || signature == "" {
		return nil, ErrInvalid
	}

	expected := sign(secret, id)
	if !hmac.Equal([]byte(signature), []byte(expected)) {
		return nil, ErrInvalid
	}

	sess, err := db.GetSession(sqlDB, id)
	if err != nil {
		return nil, ErrInvalid
	}

	return sess, nil
}

// IDFromCookie extracts the session id portion from a raw cookie value
// without verifying its signature — used by logout, where we just want to
// delete the row regardless of signature validity.
func IDFromCookie(value string) (string, bool) {
	id, _, ok := splitToken(value)
	return id, ok
}
