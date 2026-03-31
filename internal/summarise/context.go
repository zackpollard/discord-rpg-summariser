package summarise

import "context"

type ctxKey int

const sessionIDKey ctxKey = iota

// WithSessionID returns a context carrying the given session ID for LLM logging.
func WithSessionID(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, sessionIDKey, id)
}

// SessionIDFromContext extracts the session ID from the context, or 0 if not set.
func SessionIDFromContext(ctx context.Context) int64 {
	if v, ok := ctx.Value(sessionIDKey).(int64); ok {
		return v
	}
	return 0
}
