package repositories

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/phsoftwares/storelink-backend/internal/models"
	"gorm.io/gorm"
)

type IAuthRepository interface {
	UserByEmail(context.Context, string) (models.User, error)
	User(context.Context, uuid.UUID) (models.User, error)
	Companies(context.Context, uuid.UUID) ([]models.Company, error)
	CompanyAuthorized(context.Context, uuid.UUID, uuid.UUID) error
	EnsureInitialAdmin(context.Context, *models.User) (bool, error)
}

func (r *Repository) UserByEmail(ctx context.Context, email string) (models.User, error) {
	var u models.User
	e := r.db.WithContext(ctx).Where("email = ? AND ativo = true", strings.ToLower(strings.TrimSpace(email))).First(&u).Error
	return u, e
}
func (r *Repository) User(ctx context.Context, id uuid.UUID) (models.User, error) {
	var u models.User
	e := r.db.WithContext(ctx).Where("id = ? AND ativo = true", id).First(&u).Error
	return u, e
}
func (r *Repository) Companies(ctx context.Context, user uuid.UUID) ([]models.Company, error) {
	result := []models.Company{}
	e := r.db.WithContext(ctx).Table("empresa e").Select("e.*").Joins("JOIN usuario_empresa ue ON ue.id_empresa = e.id").Where("ue.id_usuario = ? AND e.ativo = true", user).Order("e.nome").Scan(&result).Error
	return result, e
}
func (r *Repository) CompanyAuthorized(ctx context.Context, user, company uuid.UUID) error {
	var n int64
	e := r.db.WithContext(ctx).Table("usuario_empresa ue").Joins("JOIN empresa e ON e.id = ue.id_empresa").Where("ue.id_usuario = ? AND ue.id_empresa = ? AND e.ativo = true", user, company).Count(&n).Error
	if e != nil {
		return e
	}
	if n != 1 {
		return models.Error(403, "Empresa não autorizada")
	}
	return nil
}
func (r *Repository) EnsureInitialAdmin(ctx context.Context, user *models.User) (bool, error) {
	created := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", "storelink.initial-admin").Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&models.User{}).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return nil
		}
		if user == nil || strings.TrimSpace(user.Nome) == "" || strings.TrimSpace(user.Email) == "" || user.Senha == "" {
			return models.Error(500, "Configure ADMIN_EMAIL, ADMIN_NAME e ADMIN_PASSWORD para criar o primeiro administrador")
		}
		if err := tx.Create(user).Error; err != nil {
			return err
		}
		created = true
		return nil
	})
	return created, err
}
