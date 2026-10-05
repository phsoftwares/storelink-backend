package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/phsoftwares/storelink-backend/internal/models"
)

type IGroupRepository interface {
	ListGroups(context.Context, uuid.UUID, Filter) (models.Page, error)
}

func (r *Repository) ListGroups(ctx context.Context, companyID uuid.UUID, filter Filter) (models.Page, error) {
	items := []models.Group{}
	return r.Scoped(ctx, companyID).List(&items, filter)
}
