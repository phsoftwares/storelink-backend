package services

import (
	"context"
	"math"
	"strconv"
	"strings"

	"github.com/phsoftwares/storelink-backend/internal/models"
	"github.com/phsoftwares/storelink-backend/internal/repositories"
)

type IProductPriceIntegrationService interface {
	UpsertProductPrice(context.Context, models.Identity, models.ProductPriceSnapshot, string, string, string) (models.ProductPriceSnapshot, error)
	UpsertProductPrices(context.Context, models.Identity, []models.ProductPriceSnapshot) (models.ProductPriceBatchResult, error)
}

var _ IProductPriceIntegrationService = (*Service)(nil)

const maxPriceValue = 99999999999999.9999

func (s *Service) UpsertProductPrice(ctx context.Context, identity models.Identity, snapshot models.ProductPriceSnapshot, pathCode, pathKind, pathQuantity string) (models.ProductPriceSnapshot, error) {
	quantity, err := strconv.ParseFloat(strings.TrimSpace(pathQuantity), 64)
	if err != nil || !validPriceNumber(quantity, true) {
		return snapshot, models.Error(400, "quantity na URL deve ser um número positivo com até quatro casas decimais")
	}
	if !strings.EqualFold(strings.TrimSpace(snapshot.LocalCode), strings.TrimSpace(pathCode)) ||
		!strings.EqualFold(strings.TrimSpace(snapshot.Kind), strings.TrimSpace(pathKind)) ||
		!samePriceNumber(snapshot.Quantity, quantity) {
		return snapshot, models.Error(400, "localCode, kind e quantity devem corresponder à chave da URL")
	}
	result, err := s.UpsertProductPrices(ctx, identity, []models.ProductPriceSnapshot{snapshot})
	if err != nil {
		return snapshot, err
	}
	return result.Items[0], nil
}

func (s *Service) UpsertProductPrices(ctx context.Context, identity models.Identity, snapshots []models.ProductPriceSnapshot) (models.ProductPriceBatchResult, error) {
	result := models.ProductPriceBatchResult{Items: make([]models.ProductPriceSnapshot, 0, len(snapshots))}
	if err := validateBatchSize(len(snapshots)); err != nil {
		return result, err
	}
	for index := range snapshots {
		if err := normalizeAndValidateProductPrice(&snapshots[index]); err != nil {
			return result, err
		}
	}

	err := s.Repo.Atomic(ctx, identity.CompanyID, func(r *repositories.TenantRepository) error {
		if err := requireStoreIntegration(r, identity, "prices"); err != nil {
			return err
		}
		seen := make(map[string]struct{}, len(snapshots))
		codes := make([]string, 0, len(snapshots))
		for _, snapshot := range snapshots {
			localCode := normalizeCatalogKey(snapshot.LocalCode)
			tierKey := localCode + ":" + snapshot.Kind + ":" + normalizedPriceQuantity(snapshot.Quantity)
			if _, exists := seen[tierKey]; exists {
				return models.Error(400, "O lote contém preços duplicados para o mesmo produto, tipo e quantidade")
			}
			seen[tierKey] = struct{}{}
			codes = append(codes, localCode)
		}
		// Share the product lock with catalog upserts so prices never race the
		// initial store mapping; the second lock serializes tiers in this store.
		keys := []string{"catalog:products:tenant:" + identity.CompanyID.String(), "catalog:prices:store:" + identity.StoreID.String()}
		if err := r.AdvisoryKeys(keys); err != nil {
			return err
		}
		mappings, err := r.ProductMappingsByLocalCodes(identity.StoreID, codes)
		if err != nil {
			return err
		}
		productIDs := make(map[string]models.ProductStoreMapping, len(mappings))
		for _, mapping := range mappings {
			productIDs[normalizeCatalogKey(mapping.LocalCode)] = mapping
		}
		prices := make([]models.ProductPrice, 0, len(snapshots))
		for _, snapshot := range snapshots {
			mapping, found := productIDs[normalizeCatalogKey(snapshot.LocalCode)]
			if !found {
				return models.Error(409, "PRODUCT_NOT_FOUND: publique o produto antes de integrar seus preços")
			}
			prices = append(prices, models.ProductPrice{
				Tenant: models.NewTenant(r.CompanyID()), StoreID: identity.StoreID, ProductID: mapping.ProductID,
				Kind: snapshot.Kind, Quantity: roundPriceNumber(snapshot.Quantity), Amount: *snapshot.Amount,
				Active: *snapshot.Active, ValidFrom: snapshot.ValidFrom,
			})
		}
		if err := r.UpsertProductPrices(prices); err != nil {
			return err
		}
		result.Items = append(result.Items, snapshots...)
		result.ProcessedCount = len(result.Items)
		return nil
	})
	return result, err
}

func normalizeAndValidateProductPrice(snapshot *models.ProductPriceSnapshot) error {
	snapshot.LocalCode = strings.TrimSpace(snapshot.LocalCode)
	snapshot.Kind = strings.ToUpper(strings.TrimSpace(snapshot.Kind))
	if snapshot.LocalCode == "" || len(snapshot.LocalCode) > 100 {
		return models.Error(400, "localCode é obrigatório e deve conter até 100 caracteres")
	}
	switch snapshot.Kind {
	case "COST", "SALE", "WHOLESALE":
	default:
		return models.Error(400, "kind deve ser COST, SALE ou WHOLESALE")
	}
	if !validPriceNumber(snapshot.Quantity, true) || snapshot.Amount == nil || !validPriceNumber(*snapshot.Amount, false) {
		return models.Error(400, "quantity e amount devem ser valores financeiros positivos válidos com até quatro casas decimais")
	}
	if snapshot.Active == nil {
		return models.Error(400, "active deve ser um valor booleano")
	}
	snapshot.Quantity = roundPriceNumber(snapshot.Quantity)
	amount := roundPriceNumber(*snapshot.Amount)
	snapshot.Amount = &amount
	return nil
}

func validPriceNumber(value float64, strictlyPositive bool) bool {
	if math.IsNaN(value) || math.IsInf(value, 0) || value > maxPriceValue {
		return false
	}
	if strictlyPositive && value <= 0 || !strictlyPositive && value < 0 {
		return false
	}
	return math.Abs(value-roundPriceNumber(value)) < 0.0000001
}

func roundPriceNumber(value float64) float64 {
	return math.Round(value*10000) / 10000
}

func samePriceNumber(left, right float64) bool {
	return validPriceNumber(left, true) && validPriceNumber(right, true) && roundPriceNumber(left) == roundPriceNumber(right)
}

func normalizedPriceQuantity(value float64) string {
	return strconv.FormatFloat(roundPriceNumber(value), 'f', 4, 64)
}
