package services

import (
	"context"
	"github.com/google/uuid"
	"github.com/phsoftwares/storelink-backend/internal/models"
	"github.com/phsoftwares/storelink-backend/internal/repositories"
	"strings"
)

type ICashierOperatorIntegrationService interface {
	UpsertCashierOperator(context.Context, models.Identity, models.CashierOperatorSnapshot, string) (models.CatalogWriteResult, error)
	UpsertCashierOperators(context.Context, models.Identity, []models.CashierOperatorSnapshot) (models.CatalogBatchResult, error)
}

var _ ICashierOperatorIntegrationService = (*Service)(nil)

func (s *Service) UpsertCashierOperator(ctx context.Context, identity models.Identity, snapshot models.CashierOperatorSnapshot, pathCode string) (models.CatalogWriteResult, error) {
	var result models.CatalogWriteResult
	err := s.Repo.Atomic(ctx, identity.CompanyID, func(r *repositories.TenantRepository) error {
		if err := requireStoreIntegration(r, identity, "cashier_operator"); err != nil {
			return err
		}
		if !strings.EqualFold(snapshot.LocalCode, pathCode) {
			return models.Error(400, "localCode deve corresponder ao código indicado na URL")
		}
		if err := validateCashierOperator(snapshot.LocalCode, snapshot.CanOpenGeneralCash, snapshot.CanViewAllReports, snapshot.BlindClosing); err != nil {
			return err
		}
		if err := lockCashierOperators(r, identity, []models.CashierOperatorSnapshot{snapshot}); err != nil {
			return err
		}
		operator, err := upsertCashierOperator(r, identity, snapshot)
		if err != nil {
			return err
		}
		result = models.CatalogWriteResult{ID: operator.ID, LocalCode: operator.LocalCode}
		return nil
	})
	return result, err
}

func (s *Service) UpsertCashierOperators(ctx context.Context, identity models.Identity, snapshots []models.CashierOperatorSnapshot) (models.CatalogBatchResult, error) {
	result := models.CatalogBatchResult{Items: make([]models.CatalogWriteResult, 0, len(snapshots))}
	if err := validateBatchSize(len(snapshots)); err != nil {
		return result, err
	}
	err := s.Repo.Atomic(ctx, identity.CompanyID, func(r *repositories.TenantRepository) error {
		if err := requireStoreIntegration(r, identity, "cashier_operator"); err != nil {
			return err
		}
		seen := make(map[string]struct{}, len(snapshots))
		for _, snapshot := range snapshots {
			if err := validateCashierOperator(snapshot.LocalCode, snapshot.CanOpenGeneralCash, snapshot.CanViewAllReports, snapshot.BlindClosing); err != nil {
				return err
			}
			if err := uniqueLocalCode(seen, snapshot.LocalCode); err != nil {
				return err
			}
		}
		if err := lockCashierOperators(r, identity, snapshots); err != nil {
			return err
		}
		for _, snapshot := range snapshots {
			operator, err := upsertCashierOperator(r, identity, snapshot)
			if err != nil {
				return err
			}
			result.Items = append(result.Items, models.CatalogWriteResult{ID: operator.ID, LocalCode: operator.LocalCode})
		}
		result.ProcessedCount = len(result.Items)
		return nil
	})
	return result, err
}

func validateCashierOperator(localCode string, canOpenGeneralCash, canViewAllReports, blindClosing *bool) error {
	if strings.TrimSpace(localCode) == "" || len(strings.TrimSpace(localCode)) > 100 || canOpenGeneralCash == nil || canViewAllReports == nil || blindClosing == nil {
		return models.Error(400, "Operador de caixa requer localCode e as três permissões booleanas")
	}
	return nil
}

func upsertCashierOperator(r *repositories.TenantRepository, identity models.Identity, snapshot models.CashierOperatorSnapshot) (models.CashierOperator, error) {
	if err := validateCashierOperator(snapshot.LocalCode, snapshot.CanOpenGeneralCash, snapshot.CanViewAllReports, snapshot.BlindClosing); err != nil {
		return models.CashierOperator{}, err
	}
	mapping, found, err := r.EmployeeMapping(identity.StoreID, snapshot.EmployeeLocalCode)
	if err != nil {
		return models.CashierOperator{}, err
	}
	if !found {
		return models.CashierOperator{}, models.Error(409, "EMPLOYEE_NOT_FOUND: publique o funcionário antes do operador de caixa")
	}
	return upsertCashierOperatorForEmployee(r, identity, snapshot, mapping.EmployeeID)
}

func upsertCashierOperatorForEmployee(r *repositories.TenantRepository, identity models.Identity, snapshot models.CashierOperatorSnapshot, employeeID uuid.UUID) (models.CashierOperator, error) {
	if err := validateCashierOperator(snapshot.LocalCode, snapshot.CanOpenGeneralCash, snapshot.CanViewAllReports, snapshot.BlindClosing); err != nil {
		return models.CashierOperator{}, err
	}
	localCode := strings.TrimSpace(snapshot.LocalCode)
	operator, found, err := r.CashierOperatorMapping(identity.StoreID, localCode)
	if err != nil {
		return operator, err
	}
	if found && operator.EmployeeID != employeeID {
		return operator, models.Error(409, "CASHIER_OPERATOR_CONFLICT: operador pertence a outro funcionário")
	}
	if found && operator.StoreID == identity.StoreID && operator.LocalCode == localCode &&
		operator.CanOpenGeneralCash == *snapshot.CanOpenGeneralCash &&
		operator.CanViewAllReports == *snapshot.CanViewAllReports &&
		operator.BlindClosing == *snapshot.BlindClosing {
		return operator, nil
	}
	operator.Tenant = models.NewTenant(r.CompanyID())
	operator.StoreID = identity.StoreID
	operator.EmployeeID = employeeID
	operator.LocalCode = localCode
	operator.CanOpenGeneralCash = *snapshot.CanOpenGeneralCash
	operator.CanViewAllReports = *snapshot.CanViewAllReports
	operator.BlindClosing = *snapshot.BlindClosing
	if found {
		if err := r.Save(&operator, operator.ID); err != nil {
			return operator, err
		}
	} else if err := r.Create(&operator); err != nil {
		return operator, err
	}
	return operator, nil
}
