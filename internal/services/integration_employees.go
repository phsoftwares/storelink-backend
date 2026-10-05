package services

import (
	"context"
	"github.com/google/uuid"
	"github.com/phsoftwares/storelink-backend/internal/models"
	"github.com/phsoftwares/storelink-backend/internal/repositories"
	"net/mail"
	"strings"
)

type IEmployeeIntegrationService interface {
	UpsertEmployee(context.Context, models.Identity, models.EmployeeSnapshot, string) (models.EmployeeSnapshot, error)
	UpsertEmployees(context.Context, models.Identity, []models.EmployeeSnapshot) (models.CatalogBatchResult, error)
}

var _ IEmployeeIntegrationService = (*Service)(nil)

func (s *Service) UpsertEmployee(ctx context.Context, identity models.Identity, snapshot models.EmployeeSnapshot, pathCode string) (models.EmployeeSnapshot, error) {
	var result models.EmployeeSnapshot
	err := s.Repo.Atomic(ctx, identity.CompanyID, func(r *repositories.TenantRepository) error {
		if err := requireStoreIntegration(r, identity, "employee"); err != nil {
			return err
		}
		if !strings.EqualFold(snapshot.LocalCode, pathCode) {
			return models.Error(400, "localCode deve corresponder ao código indicado na URL")
		}
		if err := validateEmployeeSnapshot(snapshot); err != nil {
			return err
		}
		if err := lockEmployeeSnapshots(r, identity, []models.EmployeeSnapshot{snapshot}); err != nil {
			return err
		}
		id, err := upsertEmployee(r, identity, snapshot)
		if err != nil {
			return err
		}
		snapshot.ID = id.String()
		result = snapshot
		return nil
	})
	return result, err
}

func (s *Service) UpsertEmployees(ctx context.Context, identity models.Identity, snapshots []models.EmployeeSnapshot) (models.CatalogBatchResult, error) {
	result := models.CatalogBatchResult{Items: make([]models.CatalogWriteResult, 0, len(snapshots))}
	if err := validateBatchSize(len(snapshots)); err != nil {
		return result, err
	}
	err := s.Repo.Atomic(ctx, identity.CompanyID, func(r *repositories.TenantRepository) error {
		if err := requireStoreIntegration(r, identity, "employee"); err != nil {
			return err
		}
		seen := make(map[string]struct{}, len(snapshots))
		for _, snapshot := range snapshots {
			if err := validateEmployeeSnapshot(snapshot); err != nil {
				return err
			}
			if err := uniqueLocalCode(seen, snapshot.LocalCode); err != nil {
				return err
			}
		}
		if err := lockEmployeeSnapshots(r, identity, snapshots); err != nil {
			return err
		}
		for _, snapshot := range snapshots {
			id, err := upsertEmployee(r, identity, snapshot)
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

func validateEmployeeSnapshot(snapshot models.EmployeeSnapshot) error {
	if strings.TrimSpace(snapshot.LocalCode) == "" || len(strings.TrimSpace(snapshot.LocalCode)) > 100 || strings.TrimSpace(snapshot.Name) == "" || len(strings.TrimSpace(snapshot.Name)) > 255 || len(snapshot.Document) > 20 || len(snapshot.Email) > 254 || len(snapshot.Phone) > 50 || len(snapshot.Role) > 100 || snapshot.Active == nil {
		return models.Error(400, "Funcionário contém campos obrigatórios vazios ou fora do limite")
	}
	if snapshot.Email != "" {
		if _, err := mail.ParseAddress(snapshot.Email); err != nil {
			return models.Error(400, "email inválido")
		}
	}
	document := onlyDigits(snapshot.Document)
	if strings.TrimSpace(snapshot.Document) != "" && len(document) != 11 && len(document) != 14 {
		return models.Error(400, "document deve conter CPF ou CNPJ válido")
	}
	seen := make(map[string]struct{}, len(snapshot.CashierOperators))
	for _, operator := range snapshot.CashierOperators {
		if strings.TrimSpace(operator.EmployeeLocalCode) != "" && !strings.EqualFold(strings.TrimSpace(operator.EmployeeLocalCode), strings.TrimSpace(snapshot.LocalCode)) {
			return models.Error(400, "cashierOperators só pode referenciar o funcionário do cadastro")
		}
		if err := validateCashierOperator(operator.LocalCode, operator.CanOpenGeneralCash, operator.CanViewAllReports, operator.BlindClosing); err != nil {
			return err
		}
		if err := uniqueLocalCode(seen, operator.LocalCode); err != nil {
			return err
		}
	}
	return nil
}

func upsertEmployee(r *repositories.TenantRepository, identity models.Identity, snapshot models.EmployeeSnapshot) (uuid.UUID, error) {
	if err := validateEmployeeSnapshot(snapshot); err != nil {
		return uuid.Nil, err
	}
	localCode := strings.TrimSpace(snapshot.LocalCode)
	var selected uuid.UUID
	if mapping, found, err := r.EmployeeMapping(identity.StoreID, localCode); err != nil {
		return uuid.Nil, err
	} else if found {
		selected = mapping.EmployeeID
	}
	if snapshot.ID != "" {
		id, err := parseCanonicalID(snapshot.ID)
		if err != nil {
			return uuid.Nil, err
		}
		if selected != uuid.Nil && selected != id {
			return uuid.Nil, models.Error(409, "EMPLOYEE_IDENTITY_CONFLICT: id diverge do mapeamento da loja")
		}
		selected = id
	}
	if document := onlyDigits(snapshot.Document); document != "" {
		employee, found, err := r.EmployeeByDocument(document)
		if err != nil {
			return uuid.Nil, err
		}
		if found {
			if selected != uuid.Nil && selected != employee.ID {
				return uuid.Nil, models.Error(409, "EMPLOYEE_IDENTITY_CONFLICT: documento pertence a outro funcionário")
			}
			selected = employee.ID
		}
	}
	if selected == uuid.Nil {
		selected = uuid.New()
	}
	employee := models.Employee{Tenant: models.NewTenant(r.CompanyID()), Name: strings.TrimSpace(snapshot.Name), Document: onlyDigits(snapshot.Document), Email: strings.TrimSpace(snapshot.Email), Phone: strings.TrimSpace(snapshot.Phone), Role: strings.TrimSpace(snapshot.Role), Active: *snapshot.Active}
	employee.ID = selected
	var existing models.Employee
	if err := r.Find(&existing, selected, false); err == nil {
		employee.CreatedAt = existing.CreatedAt
		if err := r.Save(&employee, selected); err != nil {
			return uuid.Nil, err
		}
	} else if repositories.IsNotFound(err) {
		if err := r.Create(&employee); err != nil {
			return uuid.Nil, err
		}
	} else {
		return uuid.Nil, err
	}
	mapping, found, err := r.EmployeeMapping(identity.StoreID, localCode)
	if err != nil {
		return uuid.Nil, err
	}
	if found {
		if mapping.EmployeeID != selected {
			return uuid.Nil, models.Error(409, "EMPLOYEE_LOCAL_CODE_CONFLICT: localCode pertence a outro funcionário")
		}
		if mapping.LocalCode != localCode {
			mapping.LocalCode = localCode
			if err := r.Save(&mapping, mapping.ID); err != nil {
				return uuid.Nil, err
			}
		}
	} else {
		mapping = models.EmployeeStoreMapping{Tenant: models.NewTenant(r.CompanyID()), StoreID: identity.StoreID, EmployeeID: selected, LocalCode: localCode}
		if err := r.Create(&mapping); err != nil {
			return uuid.Nil, err
		}
	}
	for _, value := range snapshot.CashierOperators {
		cashier := models.CashierOperatorSnapshot{LocalCode: value.LocalCode, EmployeeLocalCode: localCode, CanOpenGeneralCash: value.CanOpenGeneralCash, CanViewAllReports: value.CanViewAllReports, BlindClosing: value.BlindClosing}
		if _, err := upsertCashierOperatorForEmployee(r, identity, cashier, selected); err != nil {
			return uuid.Nil, err
		}
	}
	return selected, nil
}
