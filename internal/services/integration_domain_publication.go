package services

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"
	"github.com/phsoftwares/storelink-backend/internal/models"
	"github.com/phsoftwares/storelink-backend/internal/repositories"
)

type IDomainPublicationService interface {
	PublishDomain(context.Context, models.Identity, models.DomainPublicationDTO) (models.DomainPublicationResult, error)
	PublishDomains(context.Context, models.Identity, []models.DomainPublicationDTO) (models.DomainPublicationBatchResult, error)
}

var _ IDomainPublicationService = (*Service)(nil)

func domainPublicationOperation(entity, event string) (string, string, error) {
	canonicalEntity := strings.ToUpper(strings.TrimSpace(entity))
	switch canonicalEntity {
	case "AGREEMENT", "CONVENIO":
		canonicalEntity = "AGREEMENT"
	case "CUSTOMER", "CLIENTE":
		canonicalEntity = "CUSTOMER"
	case "EMPLOYEE", "FUNCIONARIO":
		canonicalEntity = "EMPLOYEE"
	case "CASHIER_OPERATOR", "OPERADOR_CAIXA":
		canonicalEntity = "CASHIER_OPERATOR"
	default:
		return "", "", models.Error(400, "entity deve ser AGREEMENT, CUSTOMER, EMPLOYEE ou CASHIER_OPERATOR")
	}
	event = strings.ToUpper(strings.TrimSpace(event))
	if event != "CREATED" && event != "UPDATED" && event != "DISABLED" {
		return "", "", models.Error(400, "event deve ser CREATED, UPDATED ou DISABLED")
	}
	prefix := map[string]string{
		"AGREEMENT":        "agreement",
		"CUSTOMER":         "customer",
		"EMPLOYEE":         "employee",
		"CASHIER_OPERATOR": "cashier_operator",
	}[canonicalEntity]
	return canonicalEntity, prefix + "." + strings.ToLower(event), nil
}

func domainString(payload map[string]interface{}, names ...string) string {
	for _, name := range names {
		if value, ok := payload[name]; ok && value != nil {
			if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
				return strings.TrimSpace(text)
			}
		}
	}
	return ""
}

func (s *Service) PublishDomain(ctx context.Context, identity models.Identity, request models.DomainPublicationDTO) (models.DomainPublicationResult, error) {
	result := models.DomainPublicationResult{}
	if !identity.IsAgent() || identity.StoreID == uuid.Nil {
		return result, models.Error(403, "Integracoes de loja exigem uma credencial de loja")
	}
	canonicalEntity, operation, err := domainPublicationOperation(request.Entity, request.Event)
	if err != nil {
		return result, err
	}
	var payload map[string]interface{}
	if len(request.Payload) == 0 || json.Unmarshal(request.Payload, &payload) != nil || payload == nil {
		return result, models.Error(400, "payload deve ser um objeto JSON")
	}
	if request.ID != nil && *request.ID != uuid.Nil {
		result.ID = *request.ID
	} else {
		result.ID = uuid.NewSHA1(uuid.Nil, []byte(identity.StoreID.String()+"|"+canonicalEntity+"|"+hashJSON(request.Payload)))
	}
	if rawID := domainString(payload, "id", "idGlobal"); rawID != "" {
		if parsed, parseErr := uuid.Parse(rawID); parseErr == nil && parsed != result.ID {
			return result, models.Error(409, "ID do payload difere do id da publicacao")
		}
	}
	payload["id"] = result.ID.String()
	if operation == "agreement.disabled" || operation == "customer.disabled" || operation == "employee.disabled" || operation == "cashier_operator.disabled" {
		payload["ativo"] = false
	}
	result.CanonicalCode = domainString(payload, "codigo_canonico", "canonicalCode", "codigo", "code", "codigo_local", "localCode")
	result.RelatedCanonicalCode = domainString(payload, "codigo_relacionado_canonico", "relatedCanonicalCode", "codigo_relacionado", "codigoRelacionadoCanonico", "codigoRelacionado")
	result.RelatedID = domainString(payload, "id_relacionado", "id_cadastro_relacionado", "relatedId", "idCadastroRelacionado", "idRelacionado")
	result.HashPayload = hashJSON(request.Payload)
	result.Version = publicationVersion(result.HashPayload)
	// Receivers keep the original table-shaped snapshot in payload while the
	// metadata stays at the root, matching the existing message queue contract.
	payload["versao"] = result.Version
	payload["hash_payload"] = result.HashPayload
	if _, exists := payload["payload"]; !exists {
		payload["payload"] = string(request.Payload)
	}
	payloadJSON := models.ToJSON(payload)

	err = s.Repo.Atomic(ctx, identity.CompanyID, func(r *repositories.TenantRepository) error {
		policyEntity := strings.ToLower(canonicalEntity)
		if canonicalEntity == "AGREEMENT" {
			policyEntity = "customer"
		}
		if err := requireStoreIntegration(r, identity, policyEntity); err != nil {
			return err
		}
		group, err := groupForIdentity(r, identity)
		if err != nil {
			return err
		}
		destinations, err := publicationDestinations(r, identity, request.DestinationStoreIDs)
		if err != nil {
			return err
		}
		for _, destination := range destinations {
			if _, err := route(r, identity.StoreID, destination.ID); err != nil {
				return err
			}
			if err := requireMessageIntegration(group, operation, payloadJSON); err != nil {
				return err
			}
			messageID := uuid.NewSHA1(uuid.Nil, []byte(result.ID.String()+"|"+destination.ID.String()+"|"+result.HashPayload))
			message := models.Message{Tenant: models.NewTenant(r.CompanyID()), Type: "EVENT", Operation: operation,
				OriginStoreID: identity.StoreID, DestinationStoreID: destination.ID, Payload: payloadJSON, Status: "PENDING"}
			message.ID = messageID
			var existing models.Message
			findErr := r.Find(&existing, messageID, false)
			if findErr == nil {
				if existing.Operation != operation || existing.OriginStoreID != identity.StoreID || existing.DestinationStoreID != destination.ID || !sameJSON(existing.Payload, payloadJSON) {
					return models.Error(409, "ID de publicacao ja utilizado com outro conteudo")
				}
				continue
			}
			if !repositories.IsNotFound(findErr) {
				return findErr
			}
			created, err := r.InsertUnique(&message)
			if err != nil {
				return err
			}
			if created {
				result.DestinationsQueued++
			}
		}
		return audit(r, identity, operation, &identity.StoreID, map[string]interface{}{"publicationId": result.ID, "entity": canonicalEntity, "destinationsQueued": result.DestinationsQueued})
	})
	if err != nil {
		return models.DomainPublicationResult{}, err
	}
	return result, nil
}

// PublishDomains is deliberately idempotent per item. A retry of the HTTP
// request therefore reuses the deterministic publication/message identities,
// while one request removes the per-record network round trip that made a
// full customer load unnecessarily slow. The single-item service remains the
// source of truth for validation and canonicalization.
func (s *Service) PublishDomains(ctx context.Context, identity models.Identity, requests []models.DomainPublicationDTO) (models.DomainPublicationBatchResult, error) {
	result := models.DomainPublicationBatchResult{Items: make([]models.DomainPublicationResult, 0, len(requests))}
	if len(requests) < 1 || len(requests) > 1000 {
		return result, models.Error(400, "O lote deve conter de 1 a 1000 publicacoes")
	}
	for _, request := range requests {
		item, err := s.PublishDomain(ctx, identity, request)
		if err != nil {
			return models.DomainPublicationBatchResult{}, err
		}
		result.Items = append(result.Items, item)
	}
	result.ProcessedCount = len(result.Items)
	return result, nil
}
