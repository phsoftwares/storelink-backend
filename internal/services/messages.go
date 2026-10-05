package services

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/phsoftwares/storelink-backend/internal/models"
	"github.com/phsoftwares/storelink-backend/internal/repositories"
)

type IMessageService interface {
	ListMessages(context.Context, models.Identity, repositories.Filter) (models.Page, error)
	ListErrors(context.Context, models.Identity, repositories.Filter) (models.Page, error)
	Send(context.Context, models.Identity, models.MessageDTO) (models.Message, bool, error)
	Claim(context.Context, models.Identity, int) ([]models.Message, error)
	ClaimOperation(context.Context, models.Identity, int, string) ([]models.Message, error)
	ClaimOperationCoalesced(context.Context, models.Identity, int, string) ([]models.Message, error)
	RenewClaims(context.Context, models.Identity, models.IntegrationRenewDTO) (models.IntegrationRenewResult, error)
	Ack(context.Context, models.Identity, uuid.UUID, models.AckDTO) (models.Message, error)
	AckBatch(context.Context, models.Identity, models.IntegrationAckBatchDTO) (models.IntegrationAckBatchResult, error)
	Fail(context.Context, models.Identity, uuid.UUID, models.FailDTO) (models.Message, error)
	Retry(context.Context, models.Identity, uuid.UUID) (models.Message, error)
	RetryError(context.Context, models.Identity, uuid.UUID) (models.Message, error)
	MessageDetail(context.Context, models.Identity, uuid.UUID) (interface{}, error)
}

func (s *Service) ListMessages(ctx context.Context, i models.Identity, filter repositories.Filter) (models.Page, error) {
	return s.Repo.ListMessages(ctx, i.CompanyID, filter)
}

func (s *Service) ListErrors(ctx context.Context, i models.Identity, filter repositories.Filter) (models.Page, error) {
	return s.Repo.ListIntegrationErrors(ctx, i.CompanyID, filter)
}

var operations = map[string]string{
	"product.created": "EVENT", "product.updated": "EVENT", "product.disabled": "EVENT",
	"product_group.created": "EVENT", "product_group.updated": "EVENT", "product_group.disabled": "EVENT",
	"product_subgroup.created": "EVENT", "product_subgroup.updated": "EVENT", "product_subgroup.disabled": "EVENT",
	"customer.created": "EVENT", "customer.updated": "EVENT", "customer.disabled": "EVENT",
	"agreement.created": "EVENT", "agreement.updated": "EVENT", "agreement.disabled": "EVENT",
	"employee.created": "EVENT", "employee.updated": "EVENT", "employee.disabled": "EVENT",
	"cashier_operator.created": "EVENT", "cashier_operator.updated": "EVENT", "cashier_operator.disabled": "EVENT",
	"supplier.created": "EVENT", "supplier.updated": "EVENT", "supplier.disabled": "EVENT",
	"transfer.created": "EVENT", "transfer.received": "EVENT", "transfer.cancelled": "EVENT",
	"product.update_price": "COMMAND", "product.disable": "COMMAND", "price.updated": "EVENT", "stock.changed": "EVENT", "sale.created": "EVENT", "configuration.updated": "EVENT",
}

func sameJSON(a, b models.JSON) bool {
	var x, y interface{}
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return false
	}
	return reflect.DeepEqual(x, y)
}
func sameMessage(a models.Message, d models.MessageDTO, i models.Identity) bool {
	return a.CompanyID == i.CompanyID && a.OriginStoreID == i.StoreID && a.DestinationStoreID == d.DestinationStoreID && a.Type == d.Type && a.Operation == d.Operation && sameJSON(a.Payload, d.Payload)
}
func route(r *repositories.TenantRepository, origin, destination uuid.UUID) (models.Group, error) {
	if origin == destination {
		return models.Group{}, models.Error(400, "Origem e destino devem ser diferentes")
	}
	var a, b models.Store
	if e := r.Find(&a, origin, false); e != nil {
		return models.Group{}, e
	}
	if e := r.Find(&b, destination, false); e != nil {
		return models.Group{}, e
	}
	if !a.Ativo || !b.Ativo {
		return models.Group{}, models.Error(409, "Loja inativa")
	}
	if a.GroupID != b.GroupID {
		return models.Group{}, models.Error(403, "Lojas devem pertencer ao mesmo grupo")
	}
	var g models.Group
	if e := r.Find(&g, a.GroupID, false); e != nil {
		return models.Group{}, e
	}
	if !g.Ativo {
		return models.Group{}, models.Error(409, "Grupo inativo")
	}
	return g, nil
}
func (s *Service) Send(ctx context.Context, i models.Identity, d models.MessageDTO) (models.Message, bool, error) {
	m := models.Message{Tenant: models.NewTenant(i.CompanyID), Type: d.Type, Operation: d.Operation, OriginStoreID: i.StoreID, DestinationStoreID: d.DestinationStoreID, Payload: d.Payload, Status: "PENDING"}
	m.ID = d.ID
	if kind, ok := operations[d.Operation]; !ok || kind != d.Type {
		return m, false, models.Error(400, "Operação ou tipo de mensagem não suportado")
	}
	var payload map[string]interface{}
	if json.Unmarshal(d.Payload, &payload) != nil || payload == nil {
		return m, false, models.Error(400, "Payload deve ser um objeto de domínio")
	}
	duplicate := false
	err := s.Repo.Atomic(ctx, i.CompanyID, func(r *repositories.TenantRepository) error {
		if e := r.Advisory(d.ID); e != nil {
			return e
		}
		var existing models.Message
		e := r.Find(&existing, d.ID, false)
		if e == nil {
			if !sameMessage(existing, d, i) {
				return models.Error(409, "ID de mensagem já utilizado com outro conteúdo")
			}
			m = existing
			duplicate = true
			return nil
		}
		if !repositories.IsNotFound(e) {
			return e
		}
		group, e := route(r, i.StoreID, d.DestinationStoreID)
		if e != nil {
			return e
		}
		if e := requireMessageIntegration(group, d.Operation, d.Payload); e != nil {
			return e
		}
		if strings.HasPrefix(d.Operation, "transfer.") {
			if e := s.transferEvent(r, i, &m); e != nil {
				return e
			}
		} else if id, ok := payload["id"].(string); !ok || strings.TrimSpace(id) == "" || len(id) > 255 {
			return models.Error(400, "Payload de domínio requer id textual")
		}
		created, e := r.InsertUnique(&m)
		if e != nil {
			return e
		}
		if !created {
			return models.Error(409, "Identificador já utilizado")
		}
		return nil
	})
	return m, duplicate, err
}
func (s *Service) transferEvent(r *repositories.TenantRepository, i models.Identity, m *models.Message) error {
	d, e := decodeTransferPayload(m.Payload)
	if e != nil || d.ID == uuid.Nil {
		return models.Error(400, "Transferência inválida")
	}
	if e := r.Advisory(d.ID); e != nil {
		return e
	}
	m.TransferID = &d.ID
	if m.Operation == "transfer.created" {
		if strings.TrimSpace(d.Number) == "" || len(d.Number) > 100 || len(d.Items) == 0 || len(d.Items) > 1000 {
			return models.Error(400, "Número e 1 a 1000 itens são obrigatórios")
		}
		seenItems := map[uuid.UUID]bool{}
		t := models.Transfer{Tenant: models.NewTenant(i.CompanyID), OriginStoreID: i.StoreID, DestinationStoreID: m.DestinationStoreID, Numero: d.Number, OriginCNPJ: d.OriginCNPJ, DestinationCNPJ: d.DestinationCNPJ, SenderUsername: d.SenderUsername, SenderNote: d.SenderNote, SentAt: d.SentAt, Status: "PENDING", ItemCount: len(d.Items), PayloadHash: hashJSON(d)}
		t.ID = d.ID
		if e := r.Create(&t); e != nil {
			return e
		}
		for _, line := range d.Items {
			if line.ID == uuid.Nil || seenItems[line.ID] || line.Quantity <= 0 || math.IsNaN(line.Quantity) || math.IsInf(line.Quantity, 0) || strings.TrimSpace(line.Name) == "" || len(line.Name) > 255 || len(line.Barcode) > 32 || len(line.SKU) > 100 || len(line.Unit) > 20 || ((line.ProductID == nil || *line.ProductID == uuid.Nil) && strings.TrimSpace(line.SKU) == "" && strings.TrimSpace(line.Barcode) == "") {
				return models.Error(400, "Item de transferência inválido")
			}
			seenItems[line.ID] = true
			productID, e := resolveTransferProduct(r, i.StoreID, line.ProductID, line.LocalProductCode, line.SKU, line.Barcode)
			if e != nil {
				return e
			}
			item := models.TransferItem{Tenant: models.NewTenant(i.CompanyID), TransferID: t.ID, ProductID: productID, SourceProductKey: strings.TrimSpace(line.LocalProductCode), Barcode: strings.TrimSpace(line.Barcode), SKU: strings.TrimSpace(line.SKU), Description: strings.TrimSpace(line.Name), Unit: strings.TrimSpace(line.Unit), Quantity: line.Quantity, SalePrice: line.SalePrice, CostPrice: line.CostPrice, BatchCode: strings.TrimSpace(line.BatchCode), LotSerial: strings.TrimSpace(line.LotSerial), Lot: strings.TrimSpace(line.Lot), Serial: strings.TrimSpace(line.Serial), ExpiresAt: line.ExpiresAt, ManufacturedAt: line.ManufacturedAt, Controlled: boolValue(line.Controlled), TracksBatch: boolValue(line.TracksBatch), TracksSerial: boolValue(line.TracksSerial), RegistrationMS: strings.TrimSpace(line.RegistrationMS), TherapeuticClass: strings.TrimSpace(line.TherapeuticClass), SNGPCType: strings.TrimSpace(line.SNGPCType), SNGPCUnit: strings.TrimSpace(line.SNGPCUnit), PricesInformed: boolValue(line.PricesInformed), PriceTablePayload: strings.TrimSpace(line.PriceTablePayload), UsesPriceTableInformed: boolValue(line.UsesPriceTableInformed), UsesPriceTable: strings.TrimSpace(line.UsesPriceTable), Status: "PENDING"}
			item.ID = line.ID
			if e := r.Create(&item); e != nil {
				return e
			}
		}
		return audit(r, i, "transfer.created", &i.StoreID, map[string]interface{}{"transferId": t.ID, "messageId": m.ID})
	}
	var t models.Transfer
	if e := r.Find(&t, d.ID, true); e != nil {
		return e
	}
	if m.Operation == "transfer.received" {
		if t.DestinationStoreID != i.StoreID || t.OriginStoreID != m.DestinationStoreID {
			return models.Error(403, "Somente o destino confirma recebimento")
		}
		if t.Status != "SYNCED" && t.Status != "PENDING" {
			return models.Error(409, "Transferência ainda não sincronizada ou já finalizada")
		}
		now := time.Now().UTC()
		t.Status = "RECEIVED"
		t.ReceivedAt = &now
	} else {
		if t.OriginStoreID != i.StoreID || t.DestinationStoreID != m.DestinationStoreID {
			return models.Error(403, "Somente a origem cancela transferência")
		}
		if t.Status == "RECEIVED" || t.Status == "CANCELLED" {
			return models.Error(409, "Transferência finalizada")
		}
		t.Status = "CANCELLED"
	}
	if e := r.Save(&t, t.ID); e != nil {
		return e
	}
	return audit(r, i, m.Operation, &i.StoreID, map[string]interface{}{"transferId": t.ID, "messageId": m.ID})
}
func resolveTransferProduct(r *repositories.TenantRepository, storeID uuid.UUID, requestedID *uuid.UUID, localCode, sku, barcode string) (*uuid.UUID, error) {
	var resolved uuid.UUID
	if strings.TrimSpace(localCode) != "" {
		if mapping, found, err := r.ProductMapping(storeID, localCode); err != nil {
			return nil, err
		} else if found {
			resolved = mapping.ProductID
		}
	}
	if requestedID != nil && *requestedID != uuid.Nil {
		var product models.Product
		if err := r.Find(&product, *requestedID, false); err == nil {
			if resolved != uuid.Nil && resolved != product.ID {
				return nil, models.Error(409, "PRODUCT_IDENTITY_CONFLICT: localProductCode and productId identify different products")
			}
			resolved = product.ID
		} else if !repositories.IsNotFound(err) {
			return nil, err
		}
	}
	if strings.TrimSpace(sku) != "" {
		if product, found, err := r.ProductBySKU(strings.ToUpper(strings.TrimSpace(sku))); err != nil {
			return nil, err
		} else if found {
			if resolved != uuid.Nil && resolved != product.ID {
				return nil, models.Error(409, "PRODUCT_IDENTITY_CONFLICT: productId and SKU identify different products")
			}
			resolved = product.ID
		}
	}
	if strings.TrimSpace(barcode) != "" {
		if value, found, err := r.ProductBarcode(strings.ToUpper(strings.TrimSpace(barcode))); err != nil {
			return nil, err
		} else if found {
			if resolved != uuid.Nil && resolved != value.ProductID {
				return nil, models.Error(409, "PRODUCT_IDENTITY_CONFLICT: productId and barcode identify different products")
			}
			resolved = value.ProductID
		}
	}
	if resolved == uuid.Nil {
		return nil, nil
	}
	return &resolved, nil
}

func boolValue(value *bool) bool { return value != nil && *value }

func (s *Service) Claim(ctx context.Context, i models.Identity, limit int) ([]models.Message, error) {
	return s.claim(ctx, i, limit, "", false)
}

func (s *Service) ClaimOperation(ctx context.Context, i models.Identity, limit int, operation string) ([]models.Message, error) {
	return s.claim(ctx, i, limit, operation, false)
}

func (s *Service) ClaimOperationCoalesced(ctx context.Context, i models.Identity, limit int, operation string) ([]models.Message, error) {
	return s.claim(ctx, i, limit, operation, true)
}

func (s *Service) RenewClaims(ctx context.Context, i models.Identity, d models.IntegrationRenewDTO) (models.IntegrationRenewResult, error) {
	result := models.IntegrationRenewResult{}
	err := s.Repo.Atomic(ctx, i.CompanyID, func(r *repositories.TenantRepository) error {
		now := time.Now().UTC()
		count, e := r.RenewClaims(i.StoreID, now, s.Config.Lease, d.Items)
		if e != nil {
			return e
		}
		result.Renewed = count
		result.LeaseUntil = now.Add(s.Config.Lease)
		return nil
	})
	return result, err
}

func (s *Service) claim(ctx context.Context, i models.Identity, limit int, operation string, coalesce bool) ([]models.Message, error) {
	if limit < 1 {
		limit = 25
	}
	if limit > 1000 {
		limit = 1000
	}
	out := []models.Message{}
	err := s.Repo.Atomic(ctx, i.CompanyID, func(r *repositories.TenantRepository) error {
		now := time.Now().UTC()
		expired, e := r.Expired(i.StoreID, now, s.Config.DefaultBatchSize)
		if e != nil {
			return e
		}
		for idx := range expired {
			if e := s.failMessage(r, i, &expired[idx], "Prazo de processamento expirado", now); e != nil {
				return e
			}
		}
		if coalesce {
			out, e = r.DueOperationCoalesced(i.StoreID, now, limit, operation)
		} else {
			out, e = r.DueOperation(i.StoreID, now, limit, operation)
		}
		if e != nil {
			return e
		}
		if coalesce {
			if e := r.AcknowledgeSupersededBulk(i.StoreID, now, out); e != nil {
				return e
			}
		}
		for idx := range out {
			m := &out[idx]
			token := uuid.New()
			lease := now.Add(s.Config.Lease)
			m.Status = "PROCESSING"
			m.Attempts++
			m.ClaimToken = &token
			m.LeaseUntil = &lease
			m.LastAttemptAt = &now
			m.NextAttemptAt = nil
			if e := r.Save(m, m.ID); e != nil {
				return e
			}
			a := models.Attempt{Tenant: models.NewTenant(i.CompanyID), MessageID: m.ID, AgentID: i.AgentID, ClaimToken: token, Status: "PROCESSING", StartedAt: now}
			if e := r.Create(&a); e != nil {
				return e
			}
		}
		return nil
	})
	return out, err
}
func (s *Service) Ack(ctx context.Context, i models.Identity, id uuid.UUID, d models.AckDTO) (models.Message, error) {
	var m models.Message
	e := s.Repo.Atomic(ctx, i.CompanyID, func(r *repositories.TenantRepository) error {
		if e := r.Find(&m, id, true); e != nil {
			return e
		}
		if m.DestinationStoreID != i.StoreID {
			return models.Error(404, "Mensagem não encontrada")
		}
		if m.ClaimToken == nil || *m.ClaimToken != d.ClaimToken {
			return models.Error(409, "Claim inválido ou substituído")
		}
		if m.Status == "ACKNOWLEDGED" {
			return nil
		}
		if m.Status != "PROCESSING" || m.LeaseUntil == nil || !m.LeaseUntil.After(time.Now().UTC()) {
			return models.Error(409, "Claim expirado")
		}
		if m.SyncBatchID != nil {
			if e := s.ackBatch(r, i, &m, d); e != nil {
				return e
			}
		} else if m.TransferID != nil && m.Operation == "transfer.created" {
			if e := s.ackTransfer(r, i, &m, d); e != nil {
				return e
			}
		} else if d.ProcessedCount != 0 || len(d.Errors) > 0 {
			return models.Error(400, "Resultado por item só é válido para lote")
		}
		now := time.Now().UTC()
		m.Status = "ACKNOWLEDGED"
		m.AcknowledgedAt = &now
		m.LeaseUntil = nil
		m.LastError = ""
		if e := r.Save(&m, m.ID); e != nil {
			return e
		}
		if e := r.Update(&models.Attempt{}, map[string]interface{}{"id_mensagem": m.ID, "claim_token": d.ClaimToken}, map[string]interface{}{"status": "ACKNOWLEDGED", "finished_at": now}); e != nil {
			return e
		}
		if e := r.Update(&models.IntegrationError{}, map[string]interface{}{"id_mensagem": m.ID, "status": "OPEN", "item_key": ""}, map[string]interface{}{"status": "RESOLVED", "resolved_at": now}); e != nil {
			return e
		}
		if m.TransferID != nil && m.Operation == "transfer.created" {
			var t models.Transfer
			if e := r.Find(&t, *m.TransferID, true); e != nil {
				return e
			}
			if len(d.Errors) == 0 && (t.Status == "PENDING" || t.Status == "FAILED") {
				t.Status = "SYNCED"
				t.SyncedAt = &now
				if e := r.Save(&t, t.ID); e != nil {
					return e
				}
			}
		}
		return nil
	})
	return m, e
}

func (s *Service) AckBatch(ctx context.Context, i models.Identity, d models.IntegrationAckBatchDTO) (models.IntegrationAckBatchResult, error) {
	result := models.IntegrationAckBatchResult{}
	if len(d.Items) == 0 || len(d.Items) > 1000 {
		return result, models.Error(400, "O lote de confirmaÃ§Ã£o deve conter de 1 a 1000 mensagens")
	}
	err := s.Repo.Atomic(ctx, i.CompanyID, func(r *repositories.TenantRepository) error {
		count, err := r.AcknowledgeMessagesBulk(i.StoreID, time.Now().UTC(), d.Items)
		if err != nil {
			return err
		}
		result.AcknowledgedCount = count
		return nil
	})
	return result, err
}

func (s *Service) Fail(ctx context.Context, i models.Identity, id uuid.UUID, d models.FailDTO) (models.Message, error) {
	var m models.Message
	e := s.Repo.Atomic(ctx, i.CompanyID, func(r *repositories.TenantRepository) error {
		if e := r.Find(&m, id, true); e != nil {
			return e
		}
		if m.DestinationStoreID != i.StoreID {
			return models.Error(404, "Mensagem não encontrada")
		}
		if m.ClaimToken == nil || *m.ClaimToken != d.ClaimToken {
			return models.Error(409, "Claim inválido")
		}
		if m.Status == "RETRY" || m.Status == "DEAD_LETTER" {
			return nil
		}
		if m.Status != "PROCESSING" || m.LeaseUntil == nil || !m.LeaseUntil.After(time.Now().UTC()) {
			return models.Error(409, "Mensagem fora de processamento")
		}
		return s.failMessage(r, i, &m, d.Error, time.Now().UTC())
	})
	return m, e
}
func (s *Service) retryDelay(attempt int) time.Duration {
	idx := attempt - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(s.Config.RetryDelays) {
		idx = len(s.Config.RetryDelays) - 1
	}
	if idx < 0 {
		return 5 * time.Second
	}
	return s.Config.RetryDelays[idx]
}
func (s *Service) failMessage(r *repositories.TenantRepository, i models.Identity, m *models.Message, reason string, now time.Time) error {
	m.LastError = reason
	m.LeaseUntil = nil
	m.Status = "RETRY"
	next := now.Add(s.retryDelay(m.Attempts))
	m.NextAttemptAt = &next
	if m.Attempts >= s.Config.MaxAttempts {
		m.Status = "DEAD_LETTER"
		m.NextAttemptAt = nil
	}
	if e := r.Save(m, m.ID); e != nil {
		return e
	}
	if m.ClaimToken != nil {
		if e := r.Update(&models.Attempt{}, map[string]interface{}{"id_mensagem": m.ID, "claim_token": *m.ClaimToken}, map[string]interface{}{"status": m.Status, "error": reason, "finished_at": now}); e != nil {
			return e
		}
	}
	integrationError := models.IntegrationError{Tenant: models.NewTenant(i.CompanyID), StoreID: m.DestinationStoreID, MessageID: &m.ID, SyncJobID: m.SyncJobID, SyncBatchID: m.SyncBatchID, Operation: m.Operation, Error: reason, Attempts: m.Attempts, Status: "OPEN"}
	if e := r.Create(&integrationError); e != nil {
		return e
	}
	if m.Status == "DEAD_LETTER" {
		if m.TransferID != nil && m.Operation == "transfer.created" {
			var t models.Transfer
			if e := r.Find(&t, *m.TransferID, true); e != nil {
				return e
			}
			if t.Status == "PENDING" {
				t.Status = "FAILED"
				if e := r.Save(&t, t.ID); e != nil {
					return e
				}
			}
		}
		if m.SyncBatchID != nil {
			var b models.SyncBatch
			if e := r.Find(&b, *m.SyncBatchID, true); e != nil {
				return e
			}
			b.Status = "FAILED"
			b.ErrorCount = b.ItemCount
			if e := r.Save(&b, b.ID); e != nil {
				return e
			}
			if e := s.recalculateJob(r, b.JobID); e != nil {
				return e
			}
		} else if m.SyncJobID != nil {
			var j models.SyncJob
			if e := r.Find(&j, *m.SyncJobID, true); e != nil {
				return e
			}
			if j.Status != "CANCELLED" {
				j.Status = "FAILED"
				j.FinishedAt = &now
				if e := r.Save(&j, j.ID); e != nil {
					return e
				}
			}
		}
	}
	return nil
}
func (s *Service) Retry(ctx context.Context, i models.Identity, id uuid.UUID) (models.Message, error) {
	var m models.Message
	e := s.Repo.Atomic(ctx, i.CompanyID, func(r *repositories.TenantRepository) error {
		if e := r.Find(&m, id, true); e != nil {
			return e
		}
		if m.Status != "RETRY" && m.Status != "DEAD_LETTER" && m.Status != "FAILED" {
			return models.Error(409, "Somente mensagem com falha pode ser reprocessada")
		}
		if m.LastError == "Carga cancelada" {
			return models.Error(409, "Carga cancelada; inicie nova carga")
		}
		if m.SyncJobID != nil {
			var j models.SyncJob
			if e := r.Find(&j, *m.SyncJobID, true); e != nil {
				return e
			}
			if j.Status == "CANCELLED" {
				return models.Error(409, "Carga cancelada; inicie nova carga")
			}
			if m.SyncBatchID == nil {
				j.Status = "RUNNING"
				j.FinishedAt = nil
				if e := r.Save(&j, j.ID); e != nil {
					return e
				}
			}
		}
		m.Status = "PENDING"
		m.Attempts = 0
		m.ClaimToken = nil
		m.LeaseUntil = nil
		m.NextAttemptAt = nil
		m.LastError = ""
		if e := r.Save(&m, m.ID); e != nil {
			return e
		}
		now := time.Now().UTC()
		if e := r.Update(&models.IntegrationError{}, map[string]interface{}{"id_mensagem": m.ID, "status": "OPEN", "item_key": ""}, map[string]interface{}{"status": "RESOLVED", "resolved_at": now}); e != nil {
			return e
		}
		if m.SyncBatchID != nil {
			var b models.SyncBatch
			if e := r.Find(&b, *m.SyncBatchID, true); e != nil {
				return e
			}
			b.Status = "PENDING"
			b.ErrorCount = 0
			b.ProcessedCount = 0
			if e := r.Save(&b, b.ID); e != nil {
				return e
			}
			if e := s.recalculateJob(r, b.JobID); e != nil {
				return e
			}
		}
		if m.TransferID != nil && m.Operation == "transfer.created" {
			var t models.Transfer
			if e := r.Find(&t, *m.TransferID, true); e != nil {
				return e
			}
			if t.Status == "FAILED" {
				t.Status = "PENDING"
				if e := r.Save(&t, t.ID); e != nil {
					return e
				}
			}
		}
		return audit(r, i, "message.retry", &m.DestinationStoreID, map[string]interface{}{"messageId": id})
	})
	return m, e
}
func (s *Service) RetryError(ctx context.Context, i models.Identity, id uuid.UUID) (models.Message, error) {
	var e models.IntegrationError
	if err := s.Repo.Scoped(ctx, i.CompanyID).Find(&e, id, false); err != nil {
		return models.Message{}, err
	}
	if e.MessageID == nil || e.ItemKey != "" {
		return models.Message{}, models.Error(409, "Corrija a origem e refaça a carga para reprocessar itens")
	}
	return s.Retry(ctx, i, *e.MessageID)
}
func (s *Service) MessageDetail(ctx context.Context, i models.Identity, id uuid.UUID) (interface{}, error) {
	r := s.Repo.Scoped(ctx, i.CompanyID)
	var m models.Message
	if e := r.Find(&m, id, false); e != nil {
		return nil, e
	}
	attempts := []models.Attempt{}
	if e := r.All(&attempts, map[string]interface{}{"id_mensagem": id}); e != nil {
		return nil, e
	}
	return map[string]interface{}{"message": m, "attempts": attempts}, nil
}
func (s *Service) TransferDetail(ctx context.Context, i models.Identity, id uuid.UUID) (interface{}, error) {
	r := s.Repo.Scoped(ctx, i.CompanyID)
	var t models.Transfer
	if e := r.Find(&t, id, false); e != nil {
		return nil, e
	}
	items := []models.TransferItem{}
	if e := r.All(&items, map[string]interface{}{"id_transferencia": id}); e != nil {
		return nil, e
	}
	timeline := []map[string]interface{}{{"event": "Criada", "createdAt": t.CreatedAt, "detail": "Transferência registrada; estoque inalterado"}}
	messages := []models.Message{}
	if e := r.All(&messages, map[string]interface{}{"id_transferencia": id}); e != nil {
		return nil, e
	}
	for _, m := range messages {
		timeline = append(timeline, map[string]interface{}{"event": "Disponibilizada", "createdAt": m.CreatedAt, "detail": m.Operation})
		if m.LastAttemptAt != nil {
			timeline = append(timeline, map[string]interface{}{"event": "Obtida pelo Agent", "createdAt": m.LastAttemptAt, "detail": fmt.Sprintf("%d tentativa(s)", m.Attempts)})
		}
		if m.AcknowledgedAt != nil {
			timeline = append(timeline, map[string]interface{}{"event": "ACK", "createdAt": m.AcknowledgedAt, "detail": "Persistida localmente: " + m.Operation})
		}
	}
	if t.ReceivedAt != nil {
		timeline = append(timeline, map[string]interface{}{"event": "Recebida fisicamente", "createdAt": t.ReceivedAt, "detail": "Confirmada pela loja destino"})
	}
	return map[string]interface{}{"transfer": t, "items": items, "timeline": timeline}, nil
}
