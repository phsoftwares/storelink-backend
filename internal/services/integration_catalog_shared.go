package services

import (
	"github.com/google/uuid"
	"github.com/phsoftwares/storelink-backend/internal/models"
	"github.com/phsoftwares/storelink-backend/internal/repositories"
	"regexp"
	"strings"
)

var nonDigit = regexp.MustCompile(`[^0-9]`)

func requireStoreIntegration(r *repositories.TenantRepository, identity models.Identity, entity string) error {
	var group models.Group
	if err := r.Find(&group, identity.GroupID, false); err != nil {
		return err
	}
	if !group.Ativo {
		return models.Error(409, "Grupo inativo")
	}
	return requireEntityIntegration(group, entity)
}

func validateBatchSize(count int) error {
	if count < 1 || count > 1000 {
		return models.Error(400, "O lote deve conter de 1 a 1000 registros")
	}
	return nil
}

func uniqueLocalCode(seen map[string]struct{}, code string) error {
	key := normalizeCatalogKey(code)
	if key == "" {
		return models.Error(400, "localCode é obrigatório")
	}
	if _, exists := seen[key]; exists {
		return models.Error(400, "O lote contém localCode duplicado")
	}
	seen[key] = struct{}{}
	return nil
}

func parseCanonicalID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(strings.TrimSpace(raw))
	if err != nil || id == uuid.Nil {
		return uuid.Nil, models.Error(400, "id deve ser um UUID válido")
	}
	return id, nil
}

func onlyDigits(raw string) string { return nonDigit.ReplaceAllString(raw, "") }

func storeIsMatrix(r *repositories.TenantRepository, storeID uuid.UUID) (bool, error) {
	var store models.Store
	if err := r.Find(&store, storeID, false); err != nil {
		return false, err
	}
	tipo := strings.ToUpper(strings.TrimSpace(store.Tipo))
	return tipo == "MATRIZ" || tipo == "MATRIX", nil
}

// effectiveProductBarcodePrimary keeps the first consolidated EAN stable. A
// filial may send the same product with a different local primary barcode;
// that barcode is still retained as an active alternate, but it cannot replace
// the canonical EAN. The matrix is the one source allowed to deliberately
// correct the canonical EAN. If the incoming snapshot has no primary barcode,
// an existing canonical EAN is preserved as well.
func effectiveProductBarcodePrimary(currentPrimary, incomingValue, incomingPrimaryValue string, incomingActive, incomingMarkedPrimary, sourceIsMatrix bool) bool {
	if !incomingActive {
		return false
	}
	if currentPrimary == "" {
		return incomingMarkedPrimary
	}
	if incomingValue == currentPrimary {
		if incomingPrimaryValue == "" || incomingPrimaryValue == currentPrimary {
			return true
		}
		return !sourceIsMatrix
	}
	return incomingMarkedPrimary && sourceIsMatrix
}

func hasActiveProductBarcode(snapshot models.ProductSnapshot) bool {
	for _, barcode := range snapshot.Barcodes {
		if barcode.Active != nil && *barcode.Active {
			return true
		}
	}
	return false
}

// nextCanonicalProductCode first preserves the six-digit FNTS sequence and
// falls back to a tenant-generated code when the incoming legacy code is not
// numeric. This keeps a collision deterministic without inventing a code by
// concatenating arbitrary text to a C000025 value.
func nextCanonicalProductCode(current, generated string) string {
	if next := nextLocalCodeCandidate(current); next != "" {
		return next
	}
	return strings.TrimSpace(generated)
}

func lockProductSnapshots(r *repositories.TenantRepository, identity models.Identity, _ []models.ProductSnapshot) error {
	// SKU and active barcode identity are tenant-wide, so a single bounded
	// tenant lock protects them without consuming one PostgreSQL lock per row.
	return r.AdvisoryKeys([]string{"catalog:products:tenant:" + identity.CompanyID.String()})
}

func lockCustomerSnapshots(r *repositories.TenantRepository, identity models.Identity, _ []models.CustomerSnapshot) error {
	return r.AdvisoryKeys([]string{"catalog:customers:tenant:" + identity.CompanyID.String()})
}

func lockEmployeeSnapshots(r *repositories.TenantRepository, identity models.Identity, _ []models.EmployeeSnapshot) error {
	return r.AdvisoryKeys([]string{"catalog:people:tenant:" + identity.CompanyID.String()})
}

func lockCashierOperators(r *repositories.TenantRepository, identity models.Identity, _ []models.CashierOperatorSnapshot) error {
	return r.AdvisoryKeys([]string{"catalog:people:tenant:" + identity.CompanyID.String()})
}
