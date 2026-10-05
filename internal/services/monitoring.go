package services

import (
	"context"

	"github.com/phsoftwares/storelink-backend/internal/models"
	"github.com/phsoftwares/storelink-backend/internal/repositories"
)

type IMonitoringService interface {
	Dashboard(context.Context, models.Identity, repositories.Filter) (models.Dashboard, error)
}

func (s *Service) Dashboard(ctx context.Context, identity models.Identity, filter repositories.Filter) (models.Dashboard, error) {
	return s.Repo.Scoped(ctx, identity.CompanyID).Dashboard(filter, s.Config.OfflineAfter)
}
