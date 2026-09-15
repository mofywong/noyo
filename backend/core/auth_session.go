package core

import (
	"errors"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"noyo/core/store"
	"noyo/core/utils"
	"time"
)

// persistTokenPair creates a family or atomically consumes its last refresh.
// Expiry is fixed at login, so rotating cannot extend the session indefinitely.
func persistTokenPair(access, refresh, oldRefresh, secret string) (string, string, error) {
	a, err := utils.ParseToken(access, secret)
	if err != nil {
		return "", "", err
	}
	r, err := utils.ParseToken(refresh, secret)
	if err != nil || r.ExpiresAt == nil {
		return "", "", errors.New("invalid refresh claims")
	}
	family := store.AuthSession{ID: uuid.NewString(), UserID: a.UserID, AppDBID: a.AppDBID, ExpiresAt: r.ExpiresAt.Time}
	if oldRefresh != "" {
		family = store.AuthSession{}
		old, parseErr := utils.ParseToken(oldRefresh, secret)
		if parseErr != nil || old.SessionID == "" || old.TokenUse != "refresh" {
			return "", "", errors.New("session expired; sign in again")
		}
		if err := store.DB.Where("id = ? AND refresh_hash = ? AND revoked = ? AND expires_at > ?", old.SessionID, hashToken(oldRefresh), false, time.Now()).First(&family).Error; err != nil {
			return "", "", errors.New("refresh token already used or session revoked")
		}
		if family.UserID != a.UserID || family.AppDBID != a.AppDBID {
			return "", "", errors.New("session identity mismatch")
		}
	}
	a.SessionID, r.SessionID = family.ID, family.ID
	a.TokenUse, r.TokenUse = "access", "refresh"
	a.ID, r.ID = uuid.NewString(), uuid.NewString()
	r.ExpiresAt = jwt.NewNumericDate(family.ExpiresAt)
	if a.ExpiresAt.Time.After(family.ExpiresAt) {
		a.ExpiresAt = r.ExpiresAt
	}
	access, err = jwt.NewWithClaims(jwt.SigningMethodHS256, a).SignedString([]byte(secret))
	if err != nil {
		return "", "", err
	}
	refresh, err = jwt.NewWithClaims(jwt.SigningMethodHS256, r).SignedString([]byte(secret))
	if err != nil {
		return "", "", err
	}
	if oldRefresh == "" {
		family.RefreshHash = hashToken(refresh)
		err = store.DB.Create(&family).Error
	} else {
		result := store.DB.Model(&store.AuthSession{}).Where("id = ? AND refresh_hash = ? AND revoked = ? AND expires_at > ?", family.ID, hashToken(oldRefresh), false, time.Now()).Update("refresh_hash", hashToken(refresh))
		err = result.Error
		if err == nil && result.RowsAffected != 1 {
			err = errors.New("refresh token already used or session revoked")
		}
	}
	if err != nil {
		return "", "", err
	}
	return access, refresh, nil
}

func activeTokenSession(claims *utils.JWTClaims) bool {
	// Previously issued access tokens expire naturally; legacy refresh is rejected.
	if claims.SessionID == "" {
		return claims.TokenUse != "refresh"
	}
	var count int64
	err := store.DB.Model(&store.AuthSession{}).Where("id = ? AND revoked = ? AND expires_at > ?", claims.SessionID, false, time.Now()).Count(&count).Error
	return err == nil && count == 1
}
