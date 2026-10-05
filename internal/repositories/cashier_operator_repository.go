package repositories

import (
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/phsoftwares/storelink-backend/internal/models"
	"gorm.io/gorm"
)

type ICashierOperatorRepository interface {
	CashierOperatorMapping(uuid.UUID, string) (models.CashierOperator, bool, error)
}

var _ ICashierOperatorRepository = (*TenantRepository)(nil)

func (r *TenantRepository) CashierOperatorMapping(storeID uuid.UUID, localCode string) (models.CashierOperator, bool, error) {
	var value models.CashierOperator
	err := r.db.Where("id_empresa = ? AND id_loja = ? AND UPPER(BTRIM(codigo_local)) = ?", r.company, storeID, strings.ToUpper(strings.TrimSpace(localCode))).First(&value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return value, false, nil
	}
	return value, err == nil, err
}
