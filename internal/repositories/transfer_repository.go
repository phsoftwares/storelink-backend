package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/phsoftwares/storelink-backend/internal/models"
)

type ITransferRepository interface {
	ListTransfers(context.Context, uuid.UUID, Filter) (models.Page, error)
}

func (r *Repository) ListTransfers(ctx context.Context, companyID uuid.UUID, filter Filter) (models.Page, error) {
	items := []models.Transfer{}
	return r.Scoped(ctx, companyID).List(&items, filter)
}
