package services

import (
	"context"
	"github.com/google/uuid"
	"github.com/phsoftwares/storelink-backend/internal/models"
	"github.com/phsoftwares/storelink-backend/internal/repositories"
	"net/mail"
	"strings"
)

type ICustomerIntegrationService interface {
	UpsertCustomer(context.Context, models.Identity, models.CustomerSnapshot, string) (models.CustomerSnapshot, error)
	UpsertCustomers(context.Context, models.Identity, []models.CustomerSnapshot) (models.CatalogBatchResult, error)
}

var _ ICustomerIntegrationService = (*Service)(nil)

func (s *Service) UpsertCustomer(ctx context.Context, identity models.Identity, snapshot models.CustomerSnapshot, pathCode string) (models.CustomerSnapshot, error) {
	var result models.CustomerSnapshot
	err := s.Repo.Atomic(ctx, identity.CompanyID, func(r *repositories.TenantRepository) error {
		if err := requireStoreIntegration(r, identity, "customer"); err != nil {
			return err
		}
		if err := validateCustomerSnapshot(snapshot); err != nil {
			return err
		}
		if !strings.EqualFold(snapshot.LocalCode, pathCode) {
			return models.Error(400, "localCode deve corresponder ao código indicado na URL")
		}
		if err := lockCustomerSnapshots(r, identity, []models.CustomerSnapshot{snapshot}); err != nil {
			return err
		}
		id, err := upsertCustomer(r, identity, snapshot)
		if err != nil {
			return err
		}
		snapshot.ID = id.String()
		result = snapshot
		return nil
	})
	return result, err
}

func (s *Service) UpsertCustomers(ctx context.Context, identity models.Identity, snapshots []models.CustomerSnapshot) (models.CatalogBatchResult, error) {
	result := models.CatalogBatchResult{Items: make([]models.CatalogWriteResult, 0, len(snapshots))}
	if err := validateBatchSize(len(snapshots)); err != nil {
		return result, err
	}
	err := s.Repo.Atomic(ctx, identity.CompanyID, func(r *repositories.TenantRepository) error {
		if err := requireStoreIntegration(r, identity, "customer"); err != nil {
			return err
		}
		seen := make(map[string]struct{}, len(snapshots))
		for _, snapshot := range snapshots {
			if err := validateCustomerSnapshot(snapshot); err != nil {
				return err
			}
			if err := uniqueLocalCode(seen, snapshot.LocalCode); err != nil {
				return err
			}
		}
		if err := lockCustomerSnapshots(r, identity, snapshots); err != nil {
			return err
		}
		for _, snapshot := range snapshots {
			id, err := upsertCustomer(r, identity, snapshot)
			if err != nil {
				return err
			}
			result.Items = append(result.Items, models.CatalogWriteResult{ID: id, LocalCode: snapshot.LocalCode})
		}
		result.ProcessedCount = len(result.Items)
		return nil
	})
	return result, err
}

func validateCustomerSnapshot(snapshot models.CustomerSnapshot) error {
	snapshot.LocalCode = strings.TrimSpace(snapshot.LocalCode)
	snapshot.Name = strings.TrimSpace(snapshot.Name)
	snapshot.CreditStatus = strings.ToUpper(strings.TrimSpace(snapshot.CreditStatus))
	if snapshot.LocalCode == "" || len(snapshot.LocalCode) > 100 || snapshot.Name == "" || len(snapshot.Name) > 255 || len(snapshot.TaxIdentifier) > 20 || len(snapshot.Phone) > 50 || len(snapshot.Mobile) > 50 || len(snapshot.Email) > 254 {
		return models.Error(400, "Cliente contém campos obrigatórios vazios ou fora do limite")
	}
	if snapshot.CreditStatus != "STANDARD" && snapshot.CreditStatus != "UNDER_REVIEW" && snapshot.CreditStatus != "UNCLASSIFIED" {
		return models.Error(400, "creditStatus deve ser STANDARD, UNDER_REVIEW ou UNCLASSIFIED")
	}
	if snapshot.Email != "" {
		if _, err := mail.ParseAddress(snapshot.Email); err != nil {
			return models.Error(400, "email inválido")
		}
	}
	document := onlyDigits(snapshot.TaxIdentifier)
	if strings.TrimSpace(snapshot.TaxIdentifier) != "" && len(document) != 11 && len(document) != 14 {
		return models.Error(400, "taxIdentifier deve conter CPF ou CNPJ válido")
	}
	return validateAddress(snapshot.Address)
}

func validateAddress(address models.Address) error {
	if len(address.Street) > 255 || len(address.Number) > 50 || len(address.Complement) > 255 || len(address.District) > 100 || len(address.City) > 100 || len(address.State) > 2 || len(address.PostalCode) > 20 {
		return models.Error(400, "address contém campos fora do limite")
	}
	return nil
}

func upsertCustomer(r *repositories.TenantRepository, identity models.Identity, snapshot models.CustomerSnapshot) (uuid.UUID, error) {
	localCode := strings.TrimSpace(snapshot.LocalCode)
	var selected uuid.UUID
	if mapping, found, err := r.CustomerMapping(identity.StoreID, localCode); err != nil {
		return uuid.Nil, err
	} else if found {
		selected = mapping.CustomerID
	}
	if snapshot.ID != "" {
		id, err := parseCanonicalID(snapshot.ID)
		if err != nil {
			return uuid.Nil, err
		}
		if selected != uuid.Nil && selected != id {
			return uuid.Nil, models.Error(409, "CUSTOMER_IDENTITY_CONFLICT: id diverge do mapeamento da loja")
		}
		selected = id
	}
	if document := onlyDigits(snapshot.TaxIdentifier); document != "" {
		customer, found, err := r.CustomerByDocument(document)
		if err != nil {
			return uuid.Nil, err
		}
		if found {
			if selected != uuid.Nil && selected != customer.ID {
				return uuid.Nil, models.Error(409, "CUSTOMER_IDENTITY_CONFLICT: documento pertence a outro cliente")
			}
			selected = customer.ID
		}
	}
	if selected == uuid.Nil {
		selected = uuid.New()
	}
	customer := models.Customer{Tenant: models.NewTenant(r.CompanyID()), Name: strings.TrimSpace(snapshot.Name), Document: onlyDigits(snapshot.TaxIdentifier), Phone: strings.TrimSpace(snapshot.Phone), Mobile: strings.TrimSpace(snapshot.Mobile), Email: strings.TrimSpace(snapshot.Email), Address: snapshot.Address, CreditStatus: strings.ToUpper(strings.TrimSpace(snapshot.CreditStatus))}
	customer.ID = selected
	var existing models.Customer
	if err := r.Find(&existing, selected, false); err == nil {
		customer.CreatedAt = existing.CreatedAt
		if err := r.Save(&customer, selected); err != nil {
			return uuid.Nil, err
		}
	} else if repositories.IsNotFound(err) {
		if err := r.Create(&customer); err != nil {
			return uuid.Nil, err
		}
	} else {
		return uuid.Nil, err
	}
	mapping, found, err := r.CustomerMapping(identity.StoreID, localCode)
	if err != nil {
		return uuid.Nil, err
	}
	if found {
		if mapping.CustomerID != selected {
			return uuid.Nil, models.Error(409, "CUSTOMER_LOCAL_CODE_CONFLICT: localCode pertence a outro cliente")
		}
		if mapping.LocalCode != localCode {
			mapping.LocalCode = localCode
			if err := r.Save(&mapping, mapping.ID); err != nil {
				return uuid.Nil, err
			}
		}
	} else {
		mapping = models.CustomerStoreMapping{Tenant: models.NewTenant(r.CompanyID()), StoreID: identity.StoreID, CustomerID: selected, LocalCode: localCode}
		if err := r.Create(&mapping); err != nil {
			return uuid.Nil, err
		}
	}
	return selected, nil
}
