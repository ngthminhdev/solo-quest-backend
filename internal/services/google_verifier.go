package services

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const googleJWKSURL = "https://www.googleapis.com/oauth2/v3/certs"

var errGoogleKeyNotFound = errors.New("google token signing key not found")

type HTTPGoogleTokenVerifier struct {
	audiences map[string]struct{}
	client    *http.Client
	certsURL  string

	mu          sync.Mutex
	cachedKeys  map[string]*rsa.PublicKey
	cacheExpiry time.Time
}

func NewHTTPGoogleTokenVerifier(audiences []string) *HTTPGoogleTokenVerifier {
	allowed := make(map[string]struct{}, len(audiences))
	for _, audience := range audiences {
		if audience != "" {
			allowed[audience] = struct{}{}
		}
	}
	return &HTTPGoogleTokenVerifier{
		audiences: allowed,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		certsURL: googleJWKSURL,
	}
}

type googleJWTClaims struct {
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	jwt.RegisteredClaims
}

func (v *HTTPGoogleTokenVerifier) VerifyIDToken(ctx context.Context, idToken string) (*GoogleClaims, error) {
	if len(v.audiences) == 0 {
		return nil, ErrGoogleAuthNotConfigured
	}

	claims := &googleJWTClaims{}
	token, err := jwt.ParseWithClaims(idToken, claims, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodRS256 {
			return nil, ErrInvalidGoogleToken
		}
		kid, _ := token.Header["kid"].(string)
		if kid == "" {
			return nil, ErrInvalidGoogleToken
		}
		return v.publicKeyForKID(ctx, kid)
	}, jwt.WithExpirationRequired())
	if err != nil || token == nil || !token.Valid {
		return nil, ErrInvalidGoogleToken
	}

	if claims.Issuer != "accounts.google.com" && claims.Issuer != "https://accounts.google.com" {
		return nil, ErrInvalidGoogleToken
	}
	if claims.Subject == "" || claims.ExpiresAt == nil {
		return nil, ErrInvalidGoogleToken
	}

	audience := firstAllowedAudience(claims.Audience, v.audiences)
	if audience == "" {
		return nil, ErrInvalidGoogleToken
	}

	return &GoogleClaims{
		Subject:       claims.Subject,
		Email:         claims.Email,
		EmailVerified: claims.EmailVerified,
		Name:          claims.Name,
		Picture:       claims.Picture,
		Audience:      audience,
		Issuer:        claims.Issuer,
		Expiry:        claims.ExpiresAt.Time,
	}, nil
}

func (v *HTTPGoogleTokenVerifier) publicKeyForKID(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	now := time.Now().UTC()
	if v.cachedKeys != nil && now.Before(v.cacheExpiry) {
		if key := v.cachedKeys[kid]; key != nil {
			return key, nil
		}
	}

	keys, expiry, err := v.fetchKeys(ctx)
	if err != nil {
		return nil, err
	}
	v.cachedKeys = keys
	v.cacheExpiry = expiry

	key := keys[kid]
	if key == nil {
		return nil, errGoogleKeyNotFound
	}
	return key, nil
}

type jwksResponse struct {
	Keys []jwkKey `json:"keys"`
}

type jwkKey struct {
	KID string `json:"kid"`
	KTY string `json:"kty"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func (v *HTTPGoogleTokenVerifier) fetchKeys(ctx context.Context) (map[string]*rsa.PublicKey, time.Time, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.certsURL, nil)
	if err != nil {
		return nil, time.Time{}, err
	}

	resp, err := v.client.Do(req)
	if err != nil {
		return nil, time.Time{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, time.Time{}, ErrInvalidGoogleToken
	}

	var jwks jwksResponse
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, time.Time{}, err
	}

	keys := make(map[string]*rsa.PublicKey, len(jwks.Keys))
	for _, key := range jwks.Keys {
		if key.KID == "" || key.KTY != "RSA" {
			continue
		}
		publicKey, err := rsaPublicKeyFromJWK(key)
		if err != nil {
			continue
		}
		keys[key.KID] = publicKey
	}
	if len(keys) == 0 {
		return nil, time.Time{}, ErrInvalidGoogleToken
	}

	expiry := time.Now().UTC().Add(time.Hour)
	if maxAge := cacheMaxAge(resp.Header.Get("Cache-Control")); maxAge > 0 {
		expiry = time.Now().UTC().Add(maxAge)
	}
	return keys, expiry, nil
}

func rsaPublicKeyFromJWK(key jwkKey) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(key.N)
	if err != nil {
		return nil, err
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(key.E)
	if err != nil {
		return nil, err
	}

	n := new(big.Int).SetBytes(nBytes)
	e := new(big.Int).SetBytes(eBytes).Int64()
	return &rsa.PublicKey{N: n, E: int(e)}, nil
}

func firstAllowedAudience(audiences jwt.ClaimStrings, allowed map[string]struct{}) string {
	for _, audience := range audiences {
		if _, ok := allowed[audience]; ok {
			return audience
		}
	}
	return ""
}

func cacheMaxAge(value string) time.Duration {
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if !strings.HasPrefix(part, "max-age=") {
			continue
		}
		seconds, err := time.ParseDuration(strings.TrimPrefix(part, "max-age=") + "s")
		if err == nil {
			return seconds
		}
	}
	return 0
}
