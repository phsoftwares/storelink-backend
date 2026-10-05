package services

import (
	"context"

	"github.com/phsoftwares/storelink-backend/internal/models"
	"github.com/phsoftwares/storelink-backend/internal/repositories"
)

type IAuditService interface {
	ListAudit(context.Context, models.Identity, repositories.Filter) (models.Page, error)
}

func (s *Service) ListAudit(ctx context.Context, i models.Identity, filter repositories.Filter) (models.Page, error) {
	return s.Repo.ListAudit(ctx, i.CompanyID, filter)
}
