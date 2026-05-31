package odyssey

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
)

const sessionCookieName = "odyssey_sid"

// SessionKey is the HMAC secret loaded from SESSION_SECRET env at startup.
// Must be at least 32 bytes. Set via SetSessionKey.
var sessionKey []byte

func SetSessionKey(key string) {
	sessionKey = []byte(key)
}

// IssueSession creates or returns the signed session ID cookie for this request.
// The cookie value is "<random_id>.<hmac>" — the HMAC binds the ID to our secret
// so clients cannot forge or tamper with it.
func IssueSession(w http.ResponseWriter, r *http.Request) string {
	if id := ValidateSession(r); id != "" {
		return id
	}
	id := newSessionID()
	signed := signSession(id)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    signed,
		Path:     "/",
		MaxAge:   86400 * 7, // 7 days
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		// Secure is set to false here so local dev (HTTP) works;
		// in production behind TLS set Secure: true via env flag
	})
	return id
}

// ValidateSession reads and verifies the session cookie. Returns "" if missing or invalid.
func ValidateSession(r *http.Request) string {
	c, err := r.Cookie(sessionCookieName)
	if err != nil {
		return ""
	}
	parts := strings.SplitN(c.Value, ".", 2)
	if len(parts) != 2 {
		return ""
	}
	id, sig := parts[0], parts[1]
	if !hmac.Equal([]byte(sig), []byte(computeMAC(id))) {
		return ""
	}
	return id
}

func newSessionID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func signSession(id string) string {
	return id + "." + computeMAC(id)
}

func computeMAC(id string) string {
	mac := hmac.New(sha256.New, sessionKey)
	mac.Write([]byte(id))
	return hex.EncodeToString(mac.Sum(nil))
}
