package services

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/google/uuid"
	"github.com/phsoftwares/storelink-backend/internal/auth"
	"github.com/phsoftwares/storelink-backend/internal/config"
	"github.com/phsoftwares/storelink-backend/internal/models"
	"github.com/phsoftwares/storelink-backend/internal/repositories"
)

type Service struct {
	Repo   repositories.IRepository
	Tokens *auth.Tokens
	Config config.Config
}

func New(r repositories.IRepository, t *auth.Tokens, c config.Config) *Service {
	return &Service{Repo: r, Tokens: t, Config: c}
}

func audit(r *repositories.TenantRepository, i models.Identity, action string, store *uuid.UUID, details interface{}) error {
	a := models.Audit{Tenant: models.NewTenant(i.CompanyID), StoreID: store, Action: action, Details: models.ToJSON(details)}
	if i.UserID != uuid.Nil {
		a.UserID = &i.UserID
	}
	return r.Create(&a)
}
func hashJSON(v interface{}) string {
	h := sha256.Sum256(models.ToJSON(v))
	return hex.EncodeToString(h[:])
}
