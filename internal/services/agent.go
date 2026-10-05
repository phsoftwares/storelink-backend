package services

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/phsoftwares/storelink-backend/internal/auth"
	"github.com/phsoftwares/storelink-backend/internal/models"
	"github.com/phsoftwares/storelink-backend/internal/repositories"
)

type IAgentService interface {
	ListAgents(context.Context, models.Identity, repositories.Filter) (models.Page, error)
	Provision(context.Context, models.Identity, models.AgentDTO) (models.AgentProvisionResponse, error)
	AgentUpdate(context.Context, models.Identity, uuid.UUID, *bool) (models.AgentProvisionResponse, error)
	RotateAPIKey(context.Context, models.Identity, uuid.UUID) (models.AgentProvisionResponse, error)
	Heartbeat(context.Context, models.Identity, models.HeartbeatDTO) (interface{}, error)
}

func (s *Service) ListAgents(ctx context.Context, i models.Identity, filter repositories.Filter) (models.Page, error) {
	return s.Repo.ListAgents(ctx, i.CompanyID, filter)
}

func (s *Service) Provision(ctx context.Context, i models.Identity, d models.AgentDTO) (models.AgentProvisionResponse, error) {
	apiKey, apiKeyHash, err := auth.GenerateAPIKey()
	if err != nil {
		return models.AgentProvisionResponse{}, err
	}
	a := models.Agent{Tenant: models.NewTenant(i.CompanyID), StoreID: d.StoreID, Nome: strings.TrimSpace(d.Nome), APIKeyHash: apiKeyHash, Ativo: true}
	err = s.Repo.Atomic(ctx, i.CompanyID, func(r *repositories.TenantRepository) error {
		if d.StoreID == nil || *d.StoreID == uuid.Nil {
			var compartilhados []models.Agent
			if err := r.All(&compartilhados, map[string]interface{}{"id_loja": nil}); err != nil {
				return err
			}
			if len(compartilhados) > 0 {
				return models.Error(409, "Ja existe uma credencial compartilhada para esta empresa")
			}
		} else {
			var store models.Store
			if err := r.Find(&store, *d.StoreID, false); err != nil {
				return err
			}
			if !store.Ativo {
				return models.Error(409, "Loja inativa")
			}
		}
		if err := r.Create(&a); err != nil {
			return err
		}
		return audit(r, i, "agent.created", a.StoreID, map[string]interface{}{"id": a.ID, "compartilhado": a.StoreID == nil})
	})
	if err != nil {
		return models.AgentProvisionResponse{}, err
	}
	return models.AgentProvisionResponse{Agent: a, APIKey: apiKey}, nil
}

func (s *Service) AgentUpdate(ctx context.Context, i models.Identity, id uuid.UUID, active *bool) (models.AgentProvisionResponse, error) {
	var agent models.Agent
	err := s.Repo.Atomic(ctx, i.CompanyID, func(r *repositories.TenantRepository) error {
		if err := r.Find(&agent, id, true); err != nil {
			return err
		}
		if active != nil {
			agent.Ativo = *active
		}
		if err := r.Save(&agent, agent.ID); err != nil {
			return err
		}
		return audit(r, i, "agent.updated", agent.StoreID, map[string]interface{}{"id": agent.ID, "ativo": agent.Ativo})
	})
	return models.AgentProvisionResponse{Agent: agent}, err
}

func (s *Service) RotateAPIKey(ctx context.Context, i models.Identity, id uuid.UUID) (models.AgentProvisionResponse, error) {
	apiKey, apiKeyHash, err := auth.GenerateAPIKey()
	if err != nil {
		return models.AgentProvisionResponse{}, err
	}
	var agent models.Agent
	err = s.Repo.Atomic(ctx, i.CompanyID, func(r *repositories.TenantRepository) error {
		if err := r.Find(&agent, id, true); err != nil {
			return err
		}
		agent.APIKeyHash = apiKeyHash
		if err := r.Save(&agent, agent.ID); err != nil {
			return err
		}
		return audit(r, i, "agent.api_key_rotated", agent.StoreID, map[string]interface{}{"id": agent.ID})
	})
	if err != nil {
		return models.AgentProvisionResponse{}, err
	}
	return models.AgentProvisionResponse{Agent: agent, APIKey: apiKey}, nil
}

func (s *Service) Heartbeat(ctx context.Context, i models.Identity, d models.HeartbeatDTO) (interface{}, error) {
	h := models.Heartbeat{Tenant: models.NewTenant(i.CompanyID), AgentID: i.AgentID, StoreID: i.StoreID, Version: d.Version, Status: d.Status, DatabaseOK: d.DatabaseOK, PendingCount: d.PendingCount, ErrorCount: d.ErrorCount, Products: d.Products, Customers: d.Customers, Employees: d.Employees, Suppliers: d.Suppliers}
	if !h.DatabaseOK || h.ErrorCount > 0 {
		h.Status = "DEGRADED"
	}
	e := s.Repo.Scoped(ctx, i.CompanyID).Create(&h)
	return map[string]interface{}{"status": h.Status, "serverTime": time.Now().UTC()}, e
}
