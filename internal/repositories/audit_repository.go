package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/phsoftwares/storelink-backend/internal/models"
)

type IAuditRepository interface {
	ListAudit(context.Context, uuid.UUID, Filter) (models.Page, error)
}

func (r *Repository) ListAudit(ctx context.Context, companyID uuid.UUID, filter Filter) (models.Page, error) {
	items := []models.Audit{}
	return r.Scoped(ctx, companyID).List(&items, filter)
}
