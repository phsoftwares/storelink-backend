package services

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/phsoftwares/storelink-backend/internal/models"
	"github.com/phsoftwares/storelink-backend/internal/repositories"
)

type IStoreService interface {
	ListStores(context.Context, models.Identity, repositories.Filter) (models.Page, error)
	Store(context.Context, models.Identity, uuid.UUID, models.StoreDTO) (models.Store, error)
	StoreDetail(context.Context, models.Identity, uuid.UUID) (models.StoreView, error)
}

func (s *Service) ListStores(ctx context.Context, i models.Identity, filter repositories.Filter) (models.Page, error) {
	return s.Repo.ListStores(ctx, i.CompanyID, filter, s.Config.OfflineAfter)
}

func (s *Service) Store(ctx context.Context, i models.Identity, id uuid.UUID, d models.StoreDTO) (models.Store, error) {
	st := models.Store{Tenant: models.NewTenant(i.CompanyID), Ativo: true}
	e := s.Repo.Atomic(ctx, i.CompanyID, func(r *repositories.TenantRepository) error {
		var g models.Group
		if e := r.Find(&g, d.GroupID, false); e != nil {
			return e
		}
		if !g.Ativo {
			return models.Error(409, "Grupo inativo")
		}
		action := "store.created"
		if id != uuid.Nil {
			// The create defaults include a fresh UUID; updates must look up by
			// the requested id, not by this placeholder value.
			st.ID = uuid.Nil
			if e := r.Find(&st, id, true); e != nil {
				return e
			}
			action = "store.updated"
			if st.GroupID != d.GroupID {
				return models.Error(409, "Crie uma nova loja para mudar de grupo sem alterar o histórico")
			}
		}
		st.GroupID = d.GroupID
		st.Nome = strings.TrimSpace(d.Nome)
		st.Codigo = strings.TrimSpace(d.Codigo)
		rawCNPJ := strings.TrimSpace(d.CNPJ)
		st.CNPJ = onlyDigits(rawCNPJ)
		st.Tipo = d.Tipo
		if st.Nome == "" || st.Codigo == "" {
			return models.Error(400, "Nome e código obrigatórios")
		}
		if rawCNPJ == "" {
			return models.Error(400, "CNPJ da loja obrigatorio")
		}
		if rawCNPJ != "" && len(st.CNPJ) != 14 {
			return models.Error(400, "CNPJ da loja deve conter 14 dígitos")
		}
		if rawCNPJ != "" {
			another, found, findErr := r.StoreByCNPJ(st.CNPJ)
			if findErr != nil {
				return findErr
			}
			if found && another.ID != st.ID {
				return models.Error(409, "CNPJ já está vinculado a outra loja desta empresa")
			}
		}
		if d.Ativo != nil {
			st.Ativo = *d.Ativo
		}
		var e error
		if id == uuid.Nil {
			e = r.Create(&st)
		} else {
			e = r.Save(&st, id)
		}
		if e != nil {
			return e
		}
		return audit(r, i, action, &st.ID, map[string]interface{}{"id": st.ID, "ativo": st.Ativo})
	})
	return st, e
}
func (s *Service) StoreDetail(ctx context.Context, identity models.Identity, id uuid.UUID) (models.StoreView, error) {
	page, err := s.Repo.ListStores(ctx, identity.CompanyID, repositories.Filter{StoreID: id}, s.Config.OfflineAfter)
	if err != nil {
		return models.StoreView{}, err
	}
	items, ok := page.Items.([]models.StoreView)
	if !ok || len(items) == 0 {
		return models.StoreView{}, models.Error(404, "Loja nao encontrada")
	}
	return items[0], nil
}
