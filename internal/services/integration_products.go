package services

import (
	"context"
	"strings"

	"github.com/phsoftwares/storelink-backend/internal/models"
	"github.com/phsoftwares/storelink-backend/internal/repositories"
)

type IProductIntegrationService interface {
	UpsertProduct(context.Context, models.Identity, models.ProductSnapshot, string) (models.ProductSnapshot, error)
	UpsertProducts(context.Context, models.Identity, []models.ProductSnapshot) (models.CatalogBatchResult, error)
}

var _ IProductIntegrationService = (*Service)(nil)

func (s *Service) UpsertProduct(ctx context.Context, identity models.Identity, snapshot models.ProductSnapshot, pathCode string) (models.ProductSnapshot, error) {
	var result models.ProductSnapshot
	err := s.Repo.Atomic(ctx, identity.CompanyID, func(r *repositories.TenantRepository) error {
		if err := requireStoreIntegration(r, identity, "product"); err != nil {
			return err
		}
		if err := normalizeProductBatchSnapshot(&snapshot); err != nil {
			return err
		}
		if err := validateProductPath(snapshot, pathCode); err != nil {
			return err
		}
		if err := lockProductSnapshots(r, identity, []models.ProductSnapshot{snapshot}); err != nil {
			return err
		}
		items, err := upsertProductSnapshotsBatchDirect(r, identity, []models.ProductSnapshot{snapshot})
		if err != nil {
			return err
		}
		if len(items) != 1 {
			return models.Error(500, "Produto salvo sem identidade canonica")
		}
		result = snapshot
		result.ProductID = items[0].ID.String()
		result.LocalCode = items[0].LocalCode
		result.SKU = items[0].SKU
		return nil
	})
	return result, err
}

func (s *Service) UpsertProducts(ctx context.Context, identity models.Identity, snapshots []models.ProductSnapshot) (models.CatalogBatchResult, error) {
	result := models.CatalogBatchResult{Items: make([]models.CatalogWriteResult, 0, len(snapshots))}
	if err := validateBatchSize(len(snapshots)); err != nil {
		return result, err
	}
	err := s.Repo.Atomic(ctx, identity.CompanyID, func(r *repositories.TenantRepository) error {
		if err := requireStoreIntegration(r, identity, "product"); err != nil {
			return err
		}
		seen := make(map[string]struct{}, len(snapshots))
		for index := range snapshots {
			if err := normalizeProductBatchSnapshot(&snapshots[index]); err != nil {
				return err
			}
			if err := validateProductPath(snapshots[index], snapshots[index].LocalCode); err != nil {
				return err
			}
			if err := uniqueLocalCode(seen, snapshots[index].LocalCode); err != nil {
				return err
			}
		}
		if err := lockProductSnapshots(r, identity, snapshots); err != nil {
			return err
		}
		items, err := upsertProductSnapshotsBatchDirect(r, identity, snapshots)
		if err != nil {
			return err
		}
		result.Items = items
		result.ProcessedCount = len(result.Items)
		return nil
	})
	return result, err
}

func validateProductPath(snapshot models.ProductSnapshot, pathCode string) error {
	snapshot.LocalCode = strings.TrimSpace(snapshot.LocalCode)
	if pathCode == "" || len(pathCode) > 100 || !strings.EqualFold(snapshot.LocalCode, strings.TrimSpace(pathCode)) {
		return models.Error(400, "localCode deve corresponder ao código indicado na URL")
	}
	return nil
}
