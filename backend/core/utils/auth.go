package utils

import (
	"errors"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// HashPassword hashes a plain text password using bcrypt
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 10) // cost = 10
	return string(bytes), err
}

// CheckPasswordHash compares a plain text password with a hashed password
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// JWTClaims represents the claims inside the JWT token
type JWTClaims struct {
	UserID          uint   `json:"user_id"`
	TenantID        uint   `json:"tenant_id"`
	Username        string `json:"username"`
	Role            string `json:"role"`
	AllowedTenants  []uint `json:"allowed_tenants"`
	AllowedProjects []uint `json:"allowed_projects"`
	SubjectType     string `json:"subject_type,omitempty"`
	TokenUse        string `json:"token_use,omitempty"`
	AppID           string `json:"app_id,omitempty"`
	AppDBID         uint   `json:"app_db_id,omitempty"`
	jwt.RegisteredClaims
}

// GenerateTokens generates both an access token and a refresh token
func GenerateTokens(userID, tenantID uint, username, role string, allowedTenants, allowedProjects []uint, secret string, accessExpiryMin, refreshExpiryMin int) (accessToken, refreshToken string, err error) {
	// Access Token
	accessClaims := JWTClaims{
		UserID:          userID,
		TenantID:        tenantID,
		Username:        username,
		Role:            role,
		AllowedTenants:  allowedTenants,
		AllowedProjects: allowedProjects,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(accessExpiryMin) * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}
	accessT := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessToken, err = accessT.SignedString([]byte(secret))
	if err != nil {
		return "", "", err
	}

	// Refresh Token (less payload, longer expiry)
	refreshClaims := jwt.RegisteredClaims{
		Subject:   strconv.FormatUint(uint64(userID), 10),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(refreshExpiryMin) * time.Minute)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		NotBefore: jwt.NewNumericDate(time.Now()),
	}
	refreshT := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshToken, err = refreshT.SignedString([]byte(secret))
	if err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}

// GenerateAppTokens generates short-lived bearer tokens for application access.
func GenerateAppTokens(appDBID, tenantID uint, appID, username, secret string, accessExpiryMin, refreshExpiryMin int) (accessToken, refreshToken string, err error) {
	subject := strconv.FormatUint(uint64(appDBID), 10)

	accessClaims := JWTClaims{
		TenantID:    tenantID,
		Username:    username,
		Role:        "app",
		SubjectType: "app",
		TokenUse:    "access",
		AppID:       appID,
		AppDBID:     appDBID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(accessExpiryMin) * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}
	accessT := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessToken, err = accessT.SignedString([]byte(secret))
	if err != nil {
		return "", "", err
	}

	refreshClaims := JWTClaims{
		TenantID:    tenantID,
		Username:    username,
		Role:        "app",
		SubjectType: "app",
		TokenUse:    "refresh",
		AppID:       appID,
		AppDBID:     appDBID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(refreshExpiryMin) * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}
	refreshT := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshToken, err = refreshT.SignedString([]byte(secret))
	if err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}

// ParseToken parses and validates a JWT token, returning the claims
func ParseToken(tokenString, secret string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Ensure signing method is correct
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token claims")
}
