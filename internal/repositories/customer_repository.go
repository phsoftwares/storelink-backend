package repositories

import (
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/phsoftwares/storelink-backend/internal/models"
	"gorm.io/gorm"
)

type ICustomerRepository interface {
	CustomerMapping(uuid.UUID, string) (models.CustomerStoreMapping, bool, error)
	CustomerByDocument(string) (models.Customer, bool, error)
}

var _ ICustomerRepository = (*TenantRepository)(nil)

func (r *TenantRepository) CustomerMapping(storeID uuid.UUID, localCode string) (models.CustomerStoreMapping, bool, error) {
	var value models.CustomerStoreMapping
	err := r.db.Where("id_empresa = ? AND id_loja = ? AND UPPER(BTRIM(codigo_local)) = ?", r.company, storeID, strings.ToUpper(strings.TrimSpace(localCode))).First(&value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return value, false, nil
	}
	return value, err == nil, err
}

func (r *TenantRepository) CustomerByDocument(document string) (models.Customer, bool, error) {
	var value models.Customer
	err := r.db.Where("id_empresa = ? AND regexp_replace(documento, '[^0-9]', '', 'g') = ?", r.company, document).First(&value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return value, false, nil
	}
	return value, err == nil, err
}
