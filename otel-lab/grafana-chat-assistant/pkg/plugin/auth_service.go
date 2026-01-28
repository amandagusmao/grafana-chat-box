package plugin

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

// AuthService handles authentication with the AI API
type AuthService struct {
	httpClient    *http.Client
	endpoint      string
	identificador string
	senha         string

	// Token cache
	mu          sync.RWMutex
	cachedToken string
	tokenExpiry time.Time
}

// LoginRequest represents the login request payload
type LoginRequest struct {
	Identificador string `json:"identificador"`
	Senha         string `json:"senha"`
}

// LoginResponse represents the login response
type LoginResponse struct {
	Token string `json:"token"`
}

// JWTClaims represents the JWT payload for expiration check
type JWTClaims struct {
	Exp int64 `json:"exp"`
}

// NewAuthService creates a new AuthService instance
func NewAuthService(endpoint, identificador, senha string) *AuthService {
	return &AuthService{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		endpoint:      endpoint,
		identificador: identificador,
		senha:         senha,
	}
}

// GetToken returns a valid token, refreshing if necessary
func (s *AuthService) GetToken() (string, error) {
	s.mu.RLock()
	if s.cachedToken != "" && time.Now().Before(s.tokenExpiry) {
		token := s.cachedToken
		s.mu.RUnlock()
		return token, nil
	}
	s.mu.RUnlock()

	// Need to refresh token
	return s.refreshToken()
}

// refreshToken performs login and caches the new token
func (s *AuthService) refreshToken() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock
	if s.cachedToken != "" && time.Now().Before(s.tokenExpiry) {
		return s.cachedToken, nil
	}

	log.DefaultLogger.Info("Refreshing authentication token")

	// Build login request
	loginReq := LoginRequest{
		Identificador: s.identificador,
		Senha:         s.senha,
	}

	jsonBody, err := json.Marshal(loginReq)
	if err != nil {
		log.DefaultLogger.Error("Failed to marshal login request", "error", err)
		return "", err
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/iagen-identity/v1/usuarios/login-servico", s.endpoint)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		log.DefaultLogger.Error("Failed to create login request", "error", err)
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		log.DefaultLogger.Error("Login request failed", "error", err)
		return "", err
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.DefaultLogger.Error("Failed to read login response", "error", err)
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		log.DefaultLogger.Error("Login failed", "status", resp.StatusCode, "body", string(body))
		return "", fmt.Errorf("login failed: %s", string(body))
	}

	// Parse response
	var loginResp LoginResponse
	if err := json.Unmarshal(body, &loginResp); err != nil {
		log.DefaultLogger.Error("Failed to parse login response", "error", err, "body", string(body))
		return "", err
	}

	if loginResp.Token == "" {
		return "", fmt.Errorf("empty token received from login")
	}

	// Extract expiry from JWT
	expiry, err := extractJWTExpiry(loginResp.Token)
	if err != nil {
		log.DefaultLogger.Warn("Failed to extract JWT expiry, using default 25 minutes", "error", err)
		// Default to 25 minutes (5 minutes buffer before 30 min expiry)
		expiry = time.Now().Add(25 * time.Minute)
	} else {
		// Add 1 minute buffer before actual expiry
		expiry = expiry.Add(-1 * time.Minute)
	}

	// Cache the token
	s.cachedToken = loginResp.Token
	s.tokenExpiry = expiry

	log.DefaultLogger.Info("Token refreshed successfully", "expiry", expiry)

	return s.cachedToken, nil
}

// extractJWTExpiry extracts the expiration time from a JWT token
func extractJWTExpiry(token string) (time.Time, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return time.Time{}, fmt.Errorf("invalid JWT format")
	}

	// Decode payload (second part)
	payload := parts[1]
	// Add padding if necessary
	switch len(payload) % 4 {
	case 2:
		payload += "=="
	case 3:
		payload += "="
	}

	decoded, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		// Try standard encoding
		decoded, err = base64.StdEncoding.DecodeString(payload)
		if err != nil {
			return time.Time{}, fmt.Errorf("failed to decode JWT payload: %w", err)
		}
	}

	var claims JWTClaims
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return time.Time{}, fmt.Errorf("failed to parse JWT claims: %w", err)
	}

	if claims.Exp == 0 {
		return time.Time{}, fmt.Errorf("no exp claim in JWT")
	}

	return time.Unix(claims.Exp, 0), nil
}

// InvalidateToken clears the cached token
func (s *AuthService) InvalidateToken() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cachedToken = ""
	s.tokenExpiry = time.Time{}
}