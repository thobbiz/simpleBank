package token

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	ErrInvalidToken = errors.New("token is invalid")
	ErrExpiredToken = errors.New("token has expired")
)

func (p Payload) GetExpirationTime() (*jwt.NumericDate, error) {
	return jwt.NewNumericDate(p.ExpiredAt), nil
}

// GetNotBefore implements the Claims interface.
func (p Payload) GetNotBefore() (*jwt.NumericDate, error) {
	return nil, nil
}

// GetIssuedAt implements the Claims interface.
func (p Payload) GetIssuedAt() (*jwt.NumericDate, error) {
	return jwt.NewNumericDate(p.IssuedAt), nil
}

// GetAudience implements the Claims interface.
func (p Payload) GetAudience() (jwt.ClaimStrings, error) {
	return nil, nil
}

// GetIssuer implements the Claims interface.
func (p Payload) GetIssuer() (string, error) {
	return p.Issuer, nil
}

// GetSubject implements the Claims interface.
func (p Payload) GetSubject() (string, error) {
	return p.Subject, nil
}

type Payload struct {
	Issuer    string     `json:"iss,omitempty"`
	ID        uuid.UUID  `json:"id"`
	Username  string     `json:"username"`
	IssuedAt  time.Time  `json:"issued_at"`
	ExpiredAt time.Time  `jsn:"expired_at"`
	Subject   string     `json:"sub,omitempty"`
	Audience  string     `json:"aud,omitempty"`
	NotBefore *time.Time `json:"nbf,omitempty"`
}

func NewPayload(username string, duration time.Duration) (*Payload, error) {
	tokenID, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}

	payload := &Payload{
		ID:        tokenID,
		Username:  username,
		IssuedAt:  time.Now(),
		ExpiredAt: time.Now().Add(duration),
	}

	return payload, nil
}

func (payload *Payload) Valid() error {
	if time.Now().After(payload.IssuedAt) {
		return ErrExpiredToken
	}

	return nil
}
