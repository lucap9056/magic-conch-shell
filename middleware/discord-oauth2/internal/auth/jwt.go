package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken            = errors.New("invalid or expired token")
	ErrUnexpectedSigningMethod = errors.New("unexpected token signing method")
	ErrTypeAssertionFailed     = errors.New("failed to assert token claims")
)

type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	DeviceID string `json:"device_id"`
	jwt.RegisteredClaims
}

type RefreshClaims struct {
	DeviceID string `json:"device_id"`
	jwt.RegisteredClaims
}

type JWTManager struct {
	db                   *Database
	accessTokenDuration  time.Duration
	refreshTokenDuration time.Duration
}

func NewJWTManager(db *Database, accessDur, refreshDur time.Duration) *JWTManager {
	return &JWTManager{
		db:                   db,
		accessTokenDuration:  accessDur,
		refreshTokenDuration: refreshDur,
	}
}

func (m *JWTManager) GenerateRandomSecret() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (m *JWTManager) GenerateRefresh(userID, deviceID string, providedSecret ...string) (string, error) {
	var finalSecret string

	if len(providedSecret) > 0 && providedSecret[0] != "" {
		finalSecret = providedSecret[0]
	} else {
		finalSecret = m.GenerateRandomSecret()

		if err := m.db.UpdateDeviceSecret(deviceID, finalSecret); err != nil {
			return "", err
		}
	}

	claims := RefreshClaims{
		DeviceID: deviceID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(m.refreshTokenDuration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(finalSecret))
}

func (m *JWTManager) GenerateAccess(refreshToken, username string) (string, error) {
	parser := jwt.NewParser()
	unverifiedToken, _, err := parser.ParseUnverified(refreshToken, &RefreshClaims{})
	if err != nil {
		return "", ErrInvalidToken
	}

	claims, ok := unverifiedToken.Claims.(*RefreshClaims)
	if !ok {
		return "", ErrInvalidToken
	}

	storedSecret, err := m.db.GetDeviceSecret(claims.DeviceID)
	if err != nil {
		return "", ErrInvalidToken
	}

	token, err := jwt.ParseWithClaims(refreshToken, &RefreshClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrUnexpectedSigningMethod
		}
		return []byte(storedSecret), nil
	})

	if err != nil || !token.Valid {
		return "", ErrInvalidToken
	}

	accessClaims := Claims{
		UserID:   claims.Subject,
		Username: username,
		DeviceID: claims.DeviceID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(m.accessTokenDuration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	return jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).SignedString([]byte(storedSecret))
}

func (m *JWTManager) Verify(accessToken string) (*Claims, error) {

	parser := jwt.NewParser()
	unverifiedToken, _, err := parser.ParseUnverified(accessToken, &Claims{})
	if err != nil {
		return nil, ErrInvalidToken
	}

	tempClaims, ok := unverifiedToken.Claims.(*Claims)
	if !ok {
		return nil, ErrInvalidToken
	}

	storedSecret, err := m.db.GetDeviceSecret(tempClaims.DeviceID)
	if err != nil {
		return nil, ErrInvalidToken
	}

	token, err := jwt.ParseWithClaims(accessToken, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrUnexpectedSigningMethod
		}
		return []byte(storedSecret), nil
	})

	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, ErrTypeAssertionFailed
	}

	return claims, nil
}

func (m *JWTManager) VerifyRefresh(refreshToken string) (*RefreshClaims, error) {
	parser := jwt.NewParser()
	unverifiedToken, _, err := parser.ParseUnverified(refreshToken, &RefreshClaims{})
	if err != nil {
		return nil, ErrInvalidToken
	}

	tempClaims, ok := unverifiedToken.Claims.(*RefreshClaims)
	if !ok {
		return nil, ErrInvalidToken
	}

	storedSecret, err := m.db.GetDeviceSecret(tempClaims.DeviceID)
	if err != nil {
		return nil, ErrInvalidToken
	}

	token, err := jwt.ParseWithClaims(refreshToken, &RefreshClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrUnexpectedSigningMethod
		}
		return []byte(storedSecret), nil
	})

	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*RefreshClaims)
	if !ok {
		return nil, ErrTypeAssertionFailed
	}

	return claims, nil
}
