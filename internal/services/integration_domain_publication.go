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
	case "USERCONTROL_USUARIO", "USERCONTROL_USER":
		canonicalEntity = "USERCONTROL_USUARIO"
	case "USERCONTROL_DIREITO", "USERCONTROL_RIGHT":
		canonicalEntity = "USERCONTROL_DIREITO"
	case "USERCONTROL_DIREITO_EXTRA", "USERCONTROL_RIGHT_EXTRA":
		canonicalEntity = "USERCONTROL_DIREITO_EXTRA"
	default:
		return "", "", models.Error(400, "entity de cadastro nao reconhecida")
	}
	event = strings.ToUpper(strings.TrimSpace(event))
	if event != "CREATED" && event != "UPDATED" && event != "DISABLED" {
		return "", "", models.Error(400, "event deve ser CREATED, UPDATED ou DISABLED")
	}
	prefix := map[string]string{
		"AGREEMENT":                 "agreement",
		"CUSTOMER":                  "customer",
		"EMPLOYEE":                  "employee",
		"CASHIER_OPERATOR":          "cashier_operator",
		"USERCONTROL_USUARIO":       "usercontrol_user",
		"USERCONTROL_DIREITO":       "usercontrol_right",
		"USERCONTROL_DIREITO_EXTRA": "usercontrol_right_extra",
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

func normalizeUserControlIdentityKey(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func userControlIdentityKey(canonicalEntity string, payload map[string]interface{}) (string, error) {
	if !strings.HasPrefix(canonicalEntity, "USERCONTROL_") {
		return "", nil
	}
	key := normalizeUserControlIdentityKey(domainString(payload, "chave_identidade", "identityKey"))
	if key == "" {
		return "", models.Error(400, "UserControl requer chave_identidade estavel")
	}
	if len(key) > 255 {
		return "", models.Error(400, "chave_identidade do UserControl excede 255 caracteres")
	}
	return key, nil
}

func userControlPublicationID(companyID uuid.UUID, canonicalEntity, identityKey string) uuid.UUID {
	return uuid.NewSHA1(uuid.Nil, []byte(companyID.String()+"|"+canonicalEntity+"|"+identityKey))
}

func (s *Service) publishDomainInTransaction(r *repositories.TenantRepository, identity models.Identity, request models.DomainPublicationDTO) (models.DomainPublicationResult, error) {
	result := models.DomainPublicationResult{}
	canonicalEntity, operation, err := domainPublicationOperation(request.Entity, request.Event)
	if err != nil {
		return result, err
	}
	var payload map[string]interface{}
	if len(request.Payload) == 0 || json.Unmarshal(request.Payload, &payload) != nil || payload == nil {
		return result, models.Error(400, "payload deve ser um objeto JSON")
	}
	identityKey, err := userControlIdentityKey(canonicalEntity, payload)
	if err != nil {
		return result, err
	}
	isUserControl := strings.HasPrefix(canonicalEntity, "USERCONTROL_")
	canonicalPayload := request.Payload
	if isUserControl {
		// Todas as lojas precisam publicar a mesma identidade mesmo quando um
		// agente antigo envia espacos ou caixa diferente. O ID global e sempre
		// derivado da empresa + entidade + chave, nunca do UCIdUser local.
		payload["chave_identidade"] = identityKey
		canonicalPayload = models.ToJSON(payload)
	}
	if isUserControl {
		canonicalID := userControlPublicationID(identity.CompanyID, canonicalEntity, identityKey)
		if request.ID != nil && *request.ID != uuid.Nil && *request.ID != canonicalID {
			return result, models.Error(409, "ID da publicacao do UserControl difere da identidade canonica")
		}
		result.ID = canonicalID
	} else if request.ID != nil && *request.ID != uuid.Nil {
		result.ID = *request.ID
	} else {
		result.ID = uuid.NewSHA1(uuid.Nil, []byte(identity.StoreID.String()+"|"+canonicalEntity+"|"+hashJSON(canonicalPayload)))
	}
	if rawID := domainString(payload, "id", "idGlobal"); rawID != "" {
		if parsed, parseErr := uuid.Parse(rawID); parseErr == nil && parsed != result.ID {
			return result, models.Error(409, "ID do payload difere do id da publicacao")
		}
	}
	payload["id"] = result.ID.String()
	if operation == "agreement.disabled" || operation == "customer.disabled" || operation == "employee.disabled" || operation == "cashier_operator.disabled" || operation == "usercontrol_user.disabled" || operation == "usercontrol_right.disabled" || operation == "usercontrol_right_extra.disabled" {
		payload["ativo"] = false
	}
	result.CanonicalCode = domainString(payload, "codigo_canonico", "canonicalCode", "codigo", "code", "codigo_local", "localCode")
	result.RelatedCanonicalCode = domainString(payload, "codigo_relacionado_canonico", "relatedCanonicalCode", "codigo_relacionado", "codigoRelacionadoCanonico", "codigoRelacionado")
	result.RelatedID = domainString(payload, "id_relacionado", "id_cadastro_relacionado", "relatedId", "idCadastroRelacionado", "idRelacionado")
	result.HashPayload = hashJSON(canonicalPayload)
	result.Version = publicationVersion(result.HashPayload)
	// Receivers keep the original table-shaped snapshot in payload while the
	// metadata stays at the root, matching the existing message queue contract.
	payload["versao"] = result.Version
	payload["hash_payload"] = result.HashPayload
	if _, exists := payload["payload"]; !exists {
		payload["payload"] = string(canonicalPayload)
	}
	payloadJSON := models.ToJSON(payload)

	policyEntity := strings.ToLower(canonicalEntity)
	if canonicalEntity == "AGREEMENT" {
		policyEntity = "customer"
	}
	if strings.HasPrefix(canonicalEntity, "USERCONTROL_") {
		policyEntity = "users"
	}
	if err := requireStoreIntegration(r, identity, policyEntity); err != nil {
		return result, err
	}
	group, err := groupForIdentity(r, identity)
	if err != nil {
		return result, err
	}
	destinations, err := publicationDestinations(r, identity, request.DestinationStoreIDs)
	if err != nil {
		return result, err
	}
	for _, destination := range destinations {
		if _, err := route(r, identity.StoreID, destination.ID); err != nil {
			return result, err
		}
		if err := requireMessageIntegration(group, operation, payloadJSON); err != nil {
			return result, err
		}
		messageID := uuid.NewSHA1(uuid.Nil, []byte(result.ID.String()+"|"+destination.ID.String()+"|"+result.HashPayload))
		message := models.Message{Tenant: models.NewTenant(r.CompanyID()), Type: "EVENT", Operation: operation,
			OriginStoreID: identity.StoreID, DestinationStoreID: destination.ID, Payload: payloadJSON, Status: "PENDING"}
		message.ID = messageID
		var existing models.Message
		findErr := r.Find(&existing, messageID, false)
		if findErr == nil {
			if existing.Operation != operation || existing.OriginStoreID != identity.StoreID || existing.DestinationStoreID != destination.ID || !sameJSON(existing.Payload, payloadJSON) {
				return result, models.Error(409, "ID de publicacao ja utilizado com outro conteudo")
			}
			continue
		}
		if !repositories.IsNotFound(findErr) {
			return result, findErr
		}
		created, err := r.InsertUnique(&message)
		if err != nil {
			return result, err
		}
		if created {
			result.DestinationsQueued++
		}
	}
	if err := audit(r, identity, operation, &identity.StoreID, map[string]interface{}{"publicationId": result.ID, "entity": canonicalEntity, "destinationsQueued": result.DestinationsQueued}); err != nil {
		return result, err
	}
	return result, nil
}

func (s *Service) PublishDomain(ctx context.Context, identity models.Identity, request models.DomainPublicationDTO) (models.DomainPublicationResult, error) {
	if !identity.IsAgent() || identity.StoreID == uuid.Nil {
		return models.DomainPublicationResult{}, models.Error(403, "Integracoes de loja exigem uma credencial de loja")
	}
	var result models.DomainPublicationResult
	err := s.Repo.Atomic(ctx, identity.CompanyID, func(r *repositories.TenantRepository) error {
		var err error
		result, err = s.publishDomainInTransaction(r, identity, request)
		return err
	})
	if err != nil {
		return models.DomainPublicationResult{}, err
	}
	return result, nil
}

// PublishDomains is deliberately idempotent per item. A retry of the HTTP
// request therefore reuses the deterministic publication/message identities.
// All items in one request share a database transaction: a validation, routing
// or constraint error rolls the complete batch back instead of leaving only a
// prefix of a customer/employee publication committed. The single-item
// service remains the source of truth for validation and canonicalization.
func (s *Service) PublishDomains(ctx context.Context, identity models.Identity, requests []models.DomainPublicationDTO) (models.DomainPublicationBatchResult, error) {
	result := models.DomainPublicationBatchResult{Items: make([]models.DomainPublicationResult, 0, len(requests))}
	if len(requests) < 1 || len(requests) > 1000 {
		return result, models.Error(400, "O lote deve conter de 1 a 1000 publicacoes")
	}
	if !identity.IsAgent() || identity.StoreID == uuid.Nil {
		return models.DomainPublicationBatchResult{}, models.Error(403, "Integracoes de loja exigem uma credencial de loja")
	}
	err := s.Repo.Atomic(ctx, identity.CompanyID, func(r *repositories.TenantRepository) error {
		for _, request := range requests {
			item, err := s.publishDomainInTransaction(r, identity, request)
			if err != nil {
				return err
			}
			result.Items = append(result.Items, item)
		}
		result.ProcessedCount = len(result.Items)
		return nil
	})
	if err != nil {
		return models.DomainPublicationBatchResult{}, err
	}
	return result, nil
}
