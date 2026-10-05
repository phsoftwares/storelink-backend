package repositories

import (
	"context"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"github.com/phsoftwares/storelink-backend/internal/models"
	"gorm.io/gorm"
)

type IAgentIdentityRepository interface {
	AgentForAPIKey(context.Context, uuid.UUID, string) (models.Agent, models.Store, error)
}

type IAgentRepository interface {
	ListAgents(context.Context, uuid.UUID, Filter) (models.Page, error)
}

func (r *Repository) ListAgents(ctx context.Context, companyID uuid.UUID, filter Filter) (models.Page, error) {
	items := []models.Agent{}
	return r.Scoped(ctx, companyID).List(&items, filter)
}

func normalizarCNPJ(valor string) string {
	var builder strings.Builder
	for _, caractere := range strings.ToUpper(strings.TrimSpace(valor)) {
		if unicode.IsLetter(caractere) || unicode.IsDigit(caractere) {
			builder.WriteRune(caractere)
		}
	}
	return builder.String()
}

func (r *Repository) AgentForAPIKey(ctx context.Context, id uuid.UUID, cnpj string) (models.Agent, models.Store, error) {
	var a models.Agent
	var s models.Store
	e := r.db.WithContext(ctx).Table("agente a").Select("a.*").Joins("JOIN empresa e ON e.id = a.id_empresa AND e.ativo = true").Where("a.id = ? AND a.ativo = true", id).Scan(&a).Error
	if e != nil {
		return a, s, e
	}
	if a.ID == uuid.Nil {
		return a, s, gorm.ErrRecordNotFound
	}
	cnpjNormalizado := normalizarCNPJ(cnpj)
	query := r.db.WithContext(ctx).Table("loja l").Select("l.*").Joins("JOIN grupo_loja g ON g.id = l.id_grupo AND g.id_empresa = l.id_empresa AND g.ativo = true").Where("l.id_empresa = ? AND l.ativo = true", a.CompanyID)
	if a.StoreID != nil && *a.StoreID != uuid.Nil {
		query = query.Where("l.id = ?", *a.StoreID)
	} else {
		if cnpjNormalizado == "" {
			return a, s, models.Error(401, "CNPJ da loja obrigatorio para credencial compartilhada")
		}
		query = query.Where("regexp_replace(upper(coalesce(l.cnpj, '')), '[^A-Z0-9]', '', 'g') = ?", cnpjNormalizado)
	}
	e = query.Scan(&s).Error
	if e == nil && s.ID == uuid.Nil {
		e = gorm.ErrRecordNotFound
	}
	if e == nil && cnpjNormalizado != "" && normalizarCNPJ(s.CNPJ) != cnpjNormalizado {
		e = models.Error(401, "CNPJ nao corresponde a loja autenticada")
	}
	return a, s, e
}
