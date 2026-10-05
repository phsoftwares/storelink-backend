package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/phsoftwares/storelink-backend/internal/models"
	"gorm.io/gorm"
)

type ICompanyRepository interface {
	CreateCompany(context.Context, uuid.UUID, *models.Company) error
	UpdateCompany(context.Context, uuid.UUID, *models.Company) error
}

func (r *Repository) CreateCompany(ctx context.Context, user uuid.UUID, c *models.Company) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e := tx.Create(c).Error; e != nil {
			return e
		}
		tr := &TenantRepository{db: tx, company: c.ID}
		m := models.Membership{Tenant: models.NewTenant(c.ID), UserID: user}
		if e := tr.Create(&m); e != nil {
			return e
		}
		return tr.Create(&models.Audit{Tenant: models.NewTenant(c.ID), UserID: &user, Action: "company.created", Details: models.ToJSON(map[string]interface{}{"id": c.ID})})
	})
}
func (r *Repository) UpdateCompany(ctx context.Context, user uuid.UUID, c *models.Company) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var m models.Membership
		if e := tx.Where("id_empresa = ? AND id_usuario = ?", c.ID, user).First(&m).Error; e != nil {
			return models.Error(404, "Empresa não encontrada")
		}
		var old models.Company
		if e := tx.First(&old, "id = ?", c.ID).Error; e != nil {
			return e
		}
		c.CreatedAt = old.CreatedAt
		if e := tx.Save(c).Error; e != nil {
			return e
		}
		return (&TenantRepository{db: tx, company: c.ID}).Create(&models.Audit{Tenant: models.NewTenant(c.ID), UserID: &user, Action: "company.updated", Details: models.ToJSON(map[string]interface{}{"id": c.ID})})
	})
}
