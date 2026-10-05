package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/phsoftwares/storelink-backend/internal/models"
)

type ISyncJobRepository interface {
	ListJobs(context.Context, uuid.UUID, Filter) (models.Page, error)
	ListJobBatches(context.Context, uuid.UUID, uuid.UUID, Filter) (models.Page, error)
	ListJobErrors(context.Context, uuid.UUID, uuid.UUID, Filter) (models.Page, error)
}

func (r *Repository) ListJobs(ctx context.Context, companyID uuid.UUID, filter Filter) (models.Page, error) {
	items := []models.SyncJob{}
	page, err := r.Scoped(ctx, companyID).List(&items, filter)
	for index := range items {
		items[index].SetProgress()
	}
	page.Items = items
	return page, err
}

func (r *Repository) ListJobBatches(ctx context.Context, companyID, jobID uuid.UUID, filter Filter) (models.Page, error) {
	items := []models.SyncBatch{}
	filter.Where = map[string]interface{}{"id_sync_job": jobID}
	return r.Scoped(ctx, companyID).List(&items, filter)
}

func (r *Repository) ListJobErrors(ctx context.Context, companyID, jobID uuid.UUID, filter Filter) (models.Page, error) {
	items := []models.SyncItem{}
	filter.Where = map[string]interface{}{"id_sync_job": jobID}
	return r.Scoped(ctx, companyID).List(&items, filter)
}
