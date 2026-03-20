package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	cookieName    = "rpg_session"
	sessionMaxAge = 7 * 24 * time.Hour
)

// SessionData holds the authenticated user information stored in the cookie.
type SessionData struct {
	UserID    string `json:"uid"`
	Username  string `json:"uname"`
	Avatar    string `json:"avatar"`
	ExpiresAt int64  `json:"exp"`
}

// Expired returns true if the session has passed its expiry time.
func (s *SessionData) Expired() bool {
	return time.Now().Unix() > s.ExpiresAt
}

// SessionManager handles encrypted cookie-based sessions.
type SessionManager struct {
	gcm    cipher.AEAD
	secure bool // set Cookie Secure flag (should be true in production)
}

// NewSessionManager creates a session manager. The secret is used to derive a
// 256-bit AES key (via SHA-256). If secret is empty, a random key is generated
// (sessions will not survive server restarts).
func NewSessionManager(secret string, secureCookie bool) (*SessionManager, error) {
	var key [32]byte
	if secret == "" {
		if _, err := io.ReadFull(rand.Reader, key[:]); err != nil {
			return nil, fmt.Errorf("generate random session key: %w", err)
		}
	} else {
		key = sha256.Sum256([]byte(secret))
	}

	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	return &SessionManager{gcm: gcm, secure: secureCookie}, nil
}

// Encode encrypts session data and sets it as a cookie on the response.
func (sm *SessionManager) Encode(w http.ResponseWriter, data *SessionData) error {
	plaintext, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	nonce := make([]byte, sm.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := sm.gcm.Seal(nonce, nonce, plaintext, nil)
	encoded := base64.URLEncoding.EncodeToString(ciphertext)

	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    encoded,
		Path:     "/",
		MaxAge:   int(sessionMaxAge.Seconds()),
		HttpOnly: true,
		Secure:   sm.secure,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

// Decode reads and decrypts the session cookie from the request.
// Returns nil, nil if no cookie is present.
func (sm *SessionManager) Decode(r *http.Request) (*SessionData, error) {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return nil, nil // no cookie
	}

	ciphertext, err := base64.URLEncoding.DecodeString(cookie.Value)
	if err != nil {
		return nil, errors.New("invalid session encoding")
	}

	nonceSize := sm.gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("session data too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := sm.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, errors.New("session decryption failed")
	}

	var data SessionData
	if err := json.Unmarshal(plaintext, &data); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}

	if data.Expired() {
		return nil, errors.New("session expired")
	}

	return &data, nil
}

// Clear removes the session cookie.
func (sm *SessionManager) Clear(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   sm.secure,
		SameSite: http.SameSiteLaxMode,
	})
}
