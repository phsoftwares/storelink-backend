package auth

import (
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/phsoftwares/storelink-backend/internal/models"
	"time"
)

type Claims struct {
	Kind      string    `json:"kind"`
	CompanyID uuid.UUID `json:"id_empresa,omitempty"`
	StoreID   uuid.UUID `json:"store_id,omitempty"`
	jwt.RegisteredClaims
}
type Tokens struct{ key []byte }

func New(secret string) *Tokens { return &Tokens{key: []byte(secret)} }
func (t *Tokens) Issue(i models.Identity) (string, time.Time, error) {
	now := time.Now().UTC()
	expires := now.Add(12 * time.Hour)
	c := Claims{Kind: "user", RegisteredClaims: jwt.RegisteredClaims{Subject: i.UserID.String(), Issuer: "storelink", Audience: jwt.ClaimStrings{"storelink-api"}, IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(expires)}}
	s, e := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(t.key)
	return s, expires, e
}
func (t *Tokens) Parse(raw string) (*Claims, error) {
	c := new(Claims)
	tok, e := jwt.ParseWithClaims(raw, c, func(token *jwt.Token) (interface{}, error) { return t.key, nil }, jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer("storelink"), jwt.WithAudience("storelink-api"), jwt.WithExpirationRequired())
	if e != nil || tok == nil || !tok.Valid {
		return nil, fmt.Errorf("token inválido")
	}
	id, e := uuid.Parse(c.Subject)
	if e != nil || id == uuid.Nil || c.Kind != "user" {
		return nil, fmt.Errorf("identidade inválida")
	}
	return c, nil
}
