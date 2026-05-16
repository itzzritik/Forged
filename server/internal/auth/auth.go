package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	LegacyTokenTTL = 30 * 24 * time.Hour
	AccessTokenTTL = 15 * time.Minute
	// RefreshTokenTTL bounds the lifetime of a single refresh secret. Each
	// successful rotation issues a fresh secret with a fresh window, so an
	// actively-used CLI keeps sliding forward indefinitely; this is only the
	// hard cap on inactivity.
	RefreshTokenTTL = 90 * 24 * time.Hour
	// RefreshGracePeriod is the window during which presenting a refresh
	// secret that was just rotated returns the most-recent token pair
	// instead of family-revoking. Absorbs honest retries (network hiccups,
	// near-simultaneous client requests) without weakening replay
	// detection materially.
	RefreshGracePeriod = 30 * time.Second
)

var ErrInvalidRefreshToken = errors.New("invalid refresh token")

type AccessToken struct {
	Token     string
	ExpiresAt time.Time
}

func GenerateToken(userID, email, name, secret string) (string, error) {
	token, _, err := GenerateAccessToken(userID, email, name, secret, LegacyTokenTTL)
	return token, err
}

func GenerateAccessToken(userID, email, name, secret string, ttl time.Duration) (string, time.Time, error) {
	expiresAt := time.Now().Add(ttl)
	claims := jwt.MapClaims{
		"sub":   userID,
		"email": email,
		"name":  name,
		"iat":   time.Now().Unix(),
		"exp":   expiresAt.Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, expiresAt.UTC(), nil
}

func ValidateToken(tokenString, secret string) (string, error) {
	return ValidateAccessToken(tokenString, secret)
}

func ValidateAccessToken(tokenString, secret string) (string, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return "", err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return "", fmt.Errorf("Invalid token")
	}

	sub, ok := claims["sub"].(string)
	if !ok {
		return "", fmt.Errorf("Missing subject")
	}

	return sub, nil
}

func GenerateRefreshSecret() (string, []byte, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("Generating refresh secret: %w", err)
	}
	secret := base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256([]byte(secret))
	return secret, hash[:], nil
}

func EncodeRefreshToken(sessionID, secret string) string {
	return sessionID + "." + secret
}

func DecodeRefreshToken(token string) (string, string, error) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", ErrInvalidRefreshToken
	}
	return parts[0], parts[1], nil
}

func HashRefreshSecret(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}

func CodeChallengeS256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func VerifyPKCE(verifier, challenge, method string) bool {
	if strings.TrimSpace(verifier) == "" || strings.TrimSpace(challenge) == "" {
		return false
	}
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case "S256":
		return CodeChallengeS256(verifier) == challenge
	default:
		return false
	}
}
