package repositories

import (
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/phsoftwares/storelink-backend/internal/models"
	"gorm.io/gorm"
)

type IEmployeeRepository interface {
	EmployeeMapping(uuid.UUID, string) (models.EmployeeStoreMapping, bool, error)
	EmployeeByDocument(string) (models.Employee, bool, error)
}

var _ IEmployeeRepository = (*TenantRepository)(nil)

func (r *TenantRepository) EmployeeMapping(storeID uuid.UUID, localCode string) (models.EmployeeStoreMapping, bool, error) {
	var value models.EmployeeStoreMapping
	err := r.db.Where("id_empresa = ? AND id_loja = ? AND UPPER(BTRIM(codigo_local)) = ?", r.company, storeID, strings.ToUpper(strings.TrimSpace(localCode))).First(&value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return value, false, nil
	}
	return value, err == nil, err
}

func (r *TenantRepository) EmployeeByDocument(document string) (models.Employee, bool, error) {
	var value models.Employee
	err := r.db.Where("id_empresa = ? AND regexp_replace(documento, '[^0-9]', '', 'g') = ?", r.company, document).First(&value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return value, false, nil
	}
	return value, err == nil, err
}
