package services

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/phsoftwares/storelink-backend/internal/models"
)

type ICompanyService interface {
	Companies(context.Context, uuid.UUID) ([]models.Company, error)
	Company(context.Context, models.Identity, uuid.UUID, models.CompanyDTO) (models.Company, error)
}

func (s *Service) Companies(ctx context.Context, userID uuid.UUID) ([]models.Company, error) {
	return s.Repo.Companies(ctx, userID)
}

func (s *Service) Company(ctx context.Context, i models.Identity, id uuid.UUID, d models.CompanyDTO) (models.Company, error) {
	c := models.Company{Base: models.NewBase(), Nome: strings.TrimSpace(d.Nome), CNPJ: strings.TrimSpace(d.CNPJ), Ativo: true}
	if c.Nome == "" {
		return c, models.Error(400, "Nome obrigatório")
	}
	if d.Ativo != nil {
		c.Ativo = *d.Ativo
	}
	if id == uuid.Nil {
		return c, s.Repo.CreateCompany(ctx, i.UserID, &c)
	}
	c.ID = id
	return c, s.Repo.UpdateCompany(ctx, i.UserID, &c)
}
