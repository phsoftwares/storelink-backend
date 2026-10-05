package repositories

import (
	"errors"

	"github.com/phsoftwares/storelink-backend/internal/models"
	"gorm.io/gorm"
)

func (r *TenantRepository) IntegrationExceptionByKey(key string) (models.IntegrationException, bool, error) {
	var value models.IntegrationException
	err := r.db.Where("id_empresa = ? AND chave_idempotencia = ?", r.company, key).First(&value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return value, false, nil
	}
	return value, err == nil, err
}
