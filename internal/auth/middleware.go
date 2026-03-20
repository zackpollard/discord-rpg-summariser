package auth

import (
	"context"
	"net/http"
)

type contextKey string

const userContextKey contextKey = "auth_user"

// RequireAuth returns middleware that checks for a valid session cookie.
// If the session is missing or invalid, it returns 401.
func RequireAuth(sm *SessionManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session, err := sm.Decode(r)
			if err != nil || session == nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), userContextKey, session)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserFromContext extracts the authenticated user from the request context.
// Returns nil if not authenticated.
func UserFromContext(ctx context.Context) *SessionData {
	v, _ := ctx.Value(userContextKey).(*SessionData)
	return v
}
