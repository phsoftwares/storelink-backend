package services

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/phsoftwares/storelink-backend/internal/models"
	"github.com/phsoftwares/storelink-backend/internal/repositories"
)

type IIntegrationExceptionService interface {
	PublishException(context.Context, models.Identity, models.IntegrationExceptionDTO) (models.IntegrationExceptionResult, error)
}

var _ IIntegrationExceptionService = (*Service)(nil)

func (s *Service) PublishException(ctx context.Context, identity models.Identity, request models.IntegrationExceptionDTO) (models.IntegrationExceptionResult, error) {
	result := models.IntegrationExceptionResult{}
	if !identity.IsAgent() || identity.StoreID == uuid.Nil {
		return result, models.Error(403, "Integracao de excecao exige uma credencial de loja")
	}
	request.StoreCNPJ = onlyDigits(request.StoreCNPJ)
	request.System = strings.TrimSpace(request.System)
	request.Message = strings.TrimSpace(request.Message)
	if request.System == "" || request.Message == "" {
		return result, models.Error(400, "system e message sao obrigatorios")
	}
	payloadHash := hashJSON(request)
	key := strings.TrimSpace(request.IdempotencyKey)
	if key == "" {
		key = uuid.NewSHA1(uuid.Nil, []byte(identity.AgentID.String()+"|"+payloadHash)).String()
	}
	if len(key) > 150 {
		return result, models.Error(400, "idempotencyKey excede 150 caracteres")
	}
	result.IdempotencyKey = key
	err := s.Repo.Atomic(ctx, identity.CompanyID, func(r *repositories.TenantRepository) error {
		var store models.Store
		if err := r.Find(&store, identity.StoreID, false); err != nil {
			return err
		}
		if request.StoreCNPJ != "" && store.CNPJ != "" && request.StoreCNPJ != store.CNPJ {
			return models.Error(409, "storeCnpj nao pertence a credencial autenticada")
		}
		existing, found, err := r.IntegrationExceptionByKey(key)
		if err != nil {
			return err
		}
		if found {
			if existing.HashPayload != payloadHash {
				return models.Error(409, "idempotencyKey ja utilizado com outro conteudo")
			}
			result.ID = existing.ID
			result.Duplicate = true
			return nil
		}
		exception := models.IntegrationException{Tenant: models.NewTenant(identity.CompanyID), StoreID: identity.StoreID,
			IdempotencyKey: key, System: request.System, Form: strings.TrimSpace(request.Form), User: strings.TrimSpace(request.User),
			Caption: strings.TrimSpace(request.Caption), Version: strings.TrimSpace(request.Version), CompanyName: strings.TrimSpace(request.CompanyName),
			Terminal: strings.TrimSpace(request.Terminal), Message: request.Message, OccurredAt: request.OccurredAt, TransmittedAt: request.TransmittedAt,
			HashPayload: payloadHash, Status: "OPEN"}
		if err := r.Create(&exception); err != nil {
			return err
		}
		result.ID = exception.ID
		return audit(r, identity, "integration.exception", &identity.StoreID, map[string]interface{}{"exceptionId": exception.ID, "idempotencyKey": key})
	})
	return result, err
}
