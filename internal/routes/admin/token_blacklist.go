package admin

import (
	"sync"
	"time"
)

// TokenBlacklist stores invalidated tokens in memory
type TokenBlacklist struct {
	tokens map[string]time.Time // token -> expiration time
	mutex  sync.RWMutex
}

// NewTokenBlacklist creates a new token blacklist
func NewTokenBlacklist() *TokenBlacklist {
	return &TokenBlacklist{
		tokens: make(map[string]time.Time),
	}
}

// AddToken adds a token to the blacklist with an expiration time
func (tb *TokenBlacklist) AddToken(token string, expiration time.Time) {
	tb.mutex.Lock()
	defer tb.mutex.Unlock()
	tb.tokens[token] = expiration
}

// IsTokenBlacklisted checks if a token is in the blacklist and hasn't expired
func (tb *TokenBlacklist) IsTokenBlacklisted(token string) bool {
	tb.mutex.RLock()
	defer tb.mutex.RUnlock()

	if exp, exists := tb.tokens[token]; exists {
		// Check if token has expired
		if time.Now().After(exp) {
			return false
		}
		return true
	}
	return false
}

// CleanupExpiredTokens removes expired tokens from the blacklist
func (tb *TokenBlacklist) CleanupExpiredTokens() {
	tb.mutex.Lock()
	defer tb.mutex.Unlock()

	now := time.Now()
	for token, exp := range tb.tokens {
		if now.After(exp) {
			delete(tb.tokens, token)
		}
	}
}
