package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/phsoftwares/storelink-backend/internal/models"
	"golang.org/x/crypto/bcrypt"
)

type IAuthService interface {
	EnsureInitialAdmin(context.Context) (bool, error)
	Login(context.Context, models.LoginDTO) (models.LoginResponse, error)
	Resolve(context.Context, string, string) (models.Identity, error)
	Me(context.Context, models.Identity) (models.User, []models.Company, error)
}

func (s *Service) EnsureInitialAdmin(ctx context.Context) (bool, error) {
	var user *models.User
	if s.Config.AdminEmail != "" || s.Config.AdminName != "" || s.Config.AdminPassword != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(s.Config.AdminPassword), bcrypt.DefaultCost)
		if err != nil {
			return false, err
		}
		user = &models.User{Base: models.NewBase(), Nome: s.Config.AdminName, Email: s.Config.AdminEmail, Senha: string(hash), Ativo: true}
	}
	return s.Repo.EnsureInitialAdmin(ctx, user)
}

func (s *Service) Login(ctx context.Context, d models.LoginDTO) (models.LoginResponse, error) {
	var out models.LoginResponse
	u, e := s.Repo.UserByEmail(ctx, d.Email)
	if e != nil || bcrypt.CompareHashAndPassword([]byte(u.Senha), []byte(d.Senha)) != nil {
		return out, models.Error(401, "E-mail ou senha inválidos")
	}
	companies, e := s.Repo.Companies(ctx, u.ID)
	if e != nil {
		return out, e
	}
	token, exp, e := s.Tokens.Issue(models.Identity{UserID: u.ID})
	return models.LoginResponse{Token: token, ExpiresAt: exp, User: u, Companies: companies}, e
}
func (s *Service) Resolve(ctx context.Context, raw, tenant string) (models.Identity, error) {
	var i models.Identity
	claims, err := s.Tokens.Parse(raw)
	if err != nil {
		return i, models.Error(401, "Sessão inválida ou expirada")
	}
	id, err := uuid.Parse(claims.Subject)
	if err != nil {
		return i, models.Error(401, "Sessão inválida")
	}
	user, err := s.Repo.User(ctx, id)
	if err != nil {
		return i, models.Error(401, "Usuário inativo")
	}
	i = models.Identity{UserID: user.ID, Nome: user.Nome, Email: user.Email}
	if tenant != "" {
		i.CompanyID, err = uuid.Parse(tenant)
		if err != nil {
			return i, models.Error(400, "Empresa inválida")
		}
		if err = s.Repo.CompanyAuthorized(ctx, user.ID, i.CompanyID); err != nil {
			return i, err
		}
	} else {
		companies, listErr := s.Repo.Companies(ctx, user.ID)
		if listErr != nil {
			return i, listErr
		}
		if len(companies) > 0 {
			i.CompanyID = companies[0].ID
		}
	}
	return i, nil
}
func (s *Service) Me(ctx context.Context, identity models.Identity) (models.User, []models.Company, error) {
	user, err := s.Repo.User(ctx, identity.UserID)
	if err != nil {
		return models.User{}, nil, err
	}
	companies, err := s.Repo.Companies(ctx, identity.UserID)
	return user, companies, err
}
