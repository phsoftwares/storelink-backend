package services

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/phsoftwares/storelink-backend/internal/models"
	"github.com/phsoftwares/storelink-backend/internal/repositories"
)

type IGroupService interface {
	ListGroups(context.Context, models.Identity, repositories.Filter) (models.Page, error)
	Group(context.Context, models.Identity, uuid.UUID, models.GroupDTO) (models.Group, error)
	GroupDetail(context.Context, models.Identity, uuid.UUID) (models.Group, error)
}

func (s *Service) ListGroups(ctx context.Context, i models.Identity, filter repositories.Filter) (models.Page, error) {
	return s.Repo.ListGroups(ctx, i.CompanyID, filter)
}

func (s *Service) Group(ctx context.Context, i models.Identity, id uuid.UUID, d models.GroupDTO) (models.Group, error) {
	g := models.Group{Tenant: models.NewTenant(i.CompanyID), Nome: strings.TrimSpace(d.Nome), Ativo: true, IntegrarPrecos: true, IntegrarProdutos: true, IntegrarGruposSubgrupos: true, IntegrarClientes: true, IntegrarFuncionarios: true}
	if g.Nome == "" {
		return g, models.Error(400, "Nome obrigatório")
	}
	e := s.Repo.Atomic(ctx, i.CompanyID, func(r *repositories.TenantRepository) error {
		action := "group.created"
		if id != uuid.Nil {
			// Clear the constructor's UUID before First so GORM does not use it
			// as an additional primary-key predicate.
			g.ID = uuid.Nil
			if e := r.Find(&g, id, true); e != nil {
				return e
			}
			action = "group.updated"
		}
		g.Nome = strings.TrimSpace(d.Nome)
		if d.Ativo != nil {
			g.Ativo = *d.Ativo
		}
		if d.IntegrarPrecos != nil {
			g.IntegrarPrecos = *d.IntegrarPrecos
		}
		if d.IntegrarProdutos != nil {
			g.IntegrarProdutos = *d.IntegrarProdutos
		}
		if d.IntegrarGruposSubgrupos != nil {
			g.IntegrarGruposSubgrupos = *d.IntegrarGruposSubgrupos
		}
		// Produtos carregam as referencias de grupo e subgrupo. Nunca permita
		// um catalogo de produtos ativo sem suas dependencias cadastrais.
		if g.IntegrarProdutos {
			g.IntegrarGruposSubgrupos = true
		}
		if d.IntegrarClientes != nil {
			g.IntegrarClientes = *d.IntegrarClientes
		}
		if d.IntegrarFuncionarios != nil {
			g.IntegrarFuncionarios = *d.IntegrarFuncionarios
		}
		var e error
		if id == uuid.Nil {
			e = r.Create(&g)
		} else {
			e = r.Save(&g, id)
		}
		if e != nil {
			return e
		}
		return audit(r, i, action, nil, map[string]interface{}{"id": g.ID, "ativo": g.Ativo, "integrarPrecos": g.IntegrarPrecos, "integrarProdutos": g.IntegrarProdutos, "integrarGruposSubgrupos": g.IntegrarGruposSubgrupos, "integrarClientes": g.IntegrarClientes, "integrarFuncionarios": g.IntegrarFuncionarios})
	})
	return g, e
}
func (s *Service) GroupDetail(ctx context.Context, i models.Identity, id uuid.UUID) (models.Group, error) {
	var g models.Group
	err := s.Repo.Scoped(ctx, i.CompanyID).Find(&g, id, false)
	if repositories.IsNotFound(err) {
		return g, models.Error(404, "Grupo não encontrado")
	}
	return g, err
}
