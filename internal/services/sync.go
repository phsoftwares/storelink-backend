package services

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/phsoftwares/storelink-backend/internal/models"
	"github.com/phsoftwares/storelink-backend/internal/repositories"
)

type ISyncService interface {
	CreateJob(context.Context, models.Identity, models.JobDTO) (models.SyncJob, error)
	ListJobs(context.Context, models.Identity, repositories.Filter) (models.Page, error)
	ListJobBatches(context.Context, models.Identity, uuid.UUID, repositories.Filter) (models.Page, error)
	ListJobErrors(context.Context, models.Identity, uuid.UUID, repositories.Filter) (models.Page, error)
	Batch(context.Context, models.Identity, uuid.UUID, models.BatchDTO) (models.SyncBatch, bool, error)
	CancelJob(context.Context, models.Identity, uuid.UUID) (models.SyncJob, error)
	RestartJob(context.Context, models.Identity, uuid.UUID) (models.SyncJob, error)
	Job(context.Context, models.Identity, uuid.UUID) (models.SyncJob, error)
}

func (s *Service) ListJobs(ctx context.Context, i models.Identity, filter repositories.Filter) (models.Page, error) {
	return s.Repo.ListJobs(ctx, i.CompanyID, filter)
}

func (s *Service) ListJobBatches(ctx context.Context, i models.Identity, jobID uuid.UUID, filter repositories.Filter) (models.Page, error) {
	if _, err := s.Job(ctx, i, jobID); err != nil {
		return models.Page{}, err
	}
	return s.Repo.ListJobBatches(ctx, i.CompanyID, jobID, filter)
}

func (s *Service) ListJobErrors(ctx context.Context, i models.Identity, jobID uuid.UUID, filter repositories.Filter) (models.Page, error) {
	if _, err := s.Job(ctx, i, jobID); err != nil {
		return models.Page{}, err
	}
	return s.Repo.ListJobErrors(ctx, i.CompanyID, jobID, filter)
}

func (s *Service) CreateJob(ctx context.Context, i models.Identity, d models.JobDTO) (models.SyncJob, error) {
	j := models.SyncJob{Tenant: models.NewTenant(i.CompanyID), OriginStoreID: d.OriginStoreID, DestinationStoreID: d.DestinationStoreID, Entity: d.Entity, BatchSize: d.BatchSize, Status: "PENDING"}
	if j.BatchSize == 0 {
		j.BatchSize = s.Config.DefaultBatchSize
	}
	e := s.Repo.Atomic(ctx, i.CompanyID, func(r *repositories.TenantRepository) error {
		group, e := route(r, j.OriginStoreID, j.DestinationStoreID)
		if e != nil {
			return e
		}
		if e := requireEntityIntegration(group, j.Entity); e != nil {
			return e
		}
		if e := r.Create(&j); e != nil {
			return e
		}
		command := models.Message{Tenant: models.NewTenant(i.CompanyID), Type: "COMMAND", Operation: "sync." + j.Entity + "s", OriginStoreID: j.DestinationStoreID, DestinationStoreID: j.OriginStoreID, Status: "PENDING", SyncJobID: &j.ID, Payload: models.ToJSON(map[string]interface{}{"jobId": j.ID, "destinationStoreId": j.DestinationStoreID, "batchSize": j.BatchSize, "entity": j.Entity})}
		if e := r.Create(&command); e != nil {
			return e
		}
		return audit(r, i, "sync.created", &j.DestinationStoreID, map[string]interface{}{"jobId": j.ID, "entity": j.Entity})
	})
	return j, e
}
func (s *Service) Batch(ctx context.Context, i models.Identity, jobID uuid.UUID, d models.BatchDTO) (models.SyncBatch, bool, error) {
	b := models.SyncBatch{Tenant: models.NewTenant(i.CompanyID), JobID: jobID, BatchNumber: d.BatchNumber, ItemCount: len(d.Items), Status: "PENDING", TotalItems: d.TotalItems, IsLast: d.IsLast, PayloadHash: hashJSON(d)}
	b.ID = d.ID
	duplicate := false
	e := s.Repo.Atomic(ctx, i.CompanyID, func(r *repositories.TenantRepository) error {
		if e := r.Advisory(jobID); e != nil {
			return e
		}
		var j models.SyncJob
		if e := r.Find(&j, jobID, true); e != nil {
			return e
		}
		if j.OriginStoreID != i.StoreID {
			return models.Error(404, "Carga não encontrada")
		}
		var existing models.SyncBatch
		if e := r.Find(&existing, d.ID, false); e == nil {
			if existing.JobID != jobID || existing.PayloadHash != b.PayloadHash {
				return models.Error(409, "ID de lote já utilizado com outro conteúdo")
			}
			b = existing
			duplicate = true
			return nil
		} else if !repositories.IsNotFound(e) {
			return e
		}
		group, e := route(r, j.OriginStoreID, j.DestinationStoreID)
		if e != nil {
			return e
		}
		if e := requireEntityIntegration(group, j.Entity); e != nil {
			return e
		}
		if j.Status == "CANCELLED" || j.Status == "FAILED" || j.SourceFinished {
			return models.Error(409, "Carga não aceita novos lotes")
		}
		if len(d.Items) > j.BatchSize {
			return models.Error(400, "Lote excede o tamanho configurado")
		}
		if len(d.Items) == 0 && (!d.IsLast || d.TotalItems != 0 || d.BatchNumber != 1) {
			return models.Error(400, "Lote vazio somente para carga vazia")
		}
		count, e := r.Count(&models.SyncBatch{}, map[string]interface{}{"id_sync_job": jobID})
		if e != nil {
			return e
		}
		if int64(d.BatchNumber) != count+1 {
			return models.Error(409, "Número do lote deve seguir sequência sem lacunas")
		}
		if count > 0 && j.TotalItems != d.TotalItems {
			return models.Error(409, "Total da carga mudou durante snapshot")
		}
		if j.ReceivedItems+len(d.Items) > d.TotalItems {
			return models.Error(400, "Itens excedem total informado")
		}
		if d.IsLast && j.ReceivedItems+len(d.Items) != d.TotalItems {
			return models.Error(400, "Último lote não completa o total")
		}
		// IDs dos itens são usados para vincular erros; posições inválidas usam índice do lote.
		seen := map[string]bool{}
		for index, item := range d.Items {
			var obj map[string]interface{}
			if json.Unmarshal(item, &obj) != nil || obj == nil {
				return models.Error(400, "Item deve ser objeto")
			}
			key := syncItemKey(obj, index)
			if seen[key] {
				return models.Error(400, "Item duplicado no mesmo lote")
			}
			seen[key] = true
		}
		if j.Entity == "product" {
			canonical := make([]models.JSON, len(d.Items))
			for index, item := range d.Items {
				canonical[index], e = canonicalizeProductSnapshot(r, j.OriginStoreID, item)
				if e != nil {
					return e
				}
			}
			d.Items = canonical
		}
		m := models.Message{Tenant: models.NewTenant(i.CompanyID), Type: "EVENT", Operation: "sync.batch", OriginStoreID: j.OriginStoreID, DestinationStoreID: j.DestinationStoreID, Status: "PENDING", SyncJobID: &j.ID, SyncBatchID: &b.ID, Payload: models.ToJSON(map[string]interface{}{"jobId": j.ID, "batchId": b.ID, "entity": j.Entity, "items": d.Items})}
		b.MessageID = m.ID
		if e := r.Create(&m); e != nil {
			return e
		}
		if e := r.Create(&b); e != nil {
			return e
		}
		j.TotalItems = d.TotalItems
		j.ReceivedItems += len(d.Items)
		j.SourceFinished = d.IsLast
		j.Status = "RUNNING"
		if e := r.Save(&j, j.ID); e != nil {
			return e
		}
		return nil
	})
	return b, duplicate, e
}
func (s *Service) ackBatch(r *repositories.TenantRepository, i models.Identity, m *models.Message, d models.AckDTO) error {
	var b models.SyncBatch
	if e := r.Find(&b, *m.SyncBatchID, true); e != nil {
		return e
	}
	if d.ProcessedCount+len(d.Errors) != b.ItemCount {
		return models.Error(400, "Processados mais erros devem corresponder ao lote")
	}
	var payload struct {
		Items []map[string]interface{} `json:"items"`
	}
	if e := json.Unmarshal(m.Payload, &payload); e != nil {
		return e
	}
	keys := map[string]bool{}
	for idx, item := range payload.Items {
		keys[syncItemKey(item, idx)] = true
	}
	seen := map[string]bool{}
	for _, item := range d.Errors {
		if item.Error == "" || len(item.Error) > 2000 || !keys[item.ItemKey] || seen[item.ItemKey] {
			return models.Error(400, "Erros devem referenciar itens únicos do lote")
		}
		seen[item.ItemKey] = true
		code := item.ErrorCode
		if code == "" {
			code = "ITEM_PROCESSING_FAILED"
		}
		if !errorCodePattern.MatchString(code) {
			return models.Error(400, "Invalid item error code")
		}
		row := models.SyncItem{Tenant: models.NewTenant(i.CompanyID), JobID: b.JobID, BatchID: b.ID, ItemKey: item.ItemKey, ErrorCode: code, Error: item.Error, Status: "FAILED"}
		if e := r.Create(&row); e != nil {
			return e
		}
		ie := models.IntegrationError{Tenant: models.NewTenant(i.CompanyID), StoreID: i.StoreID, MessageID: &m.ID, SyncJobID: &b.JobID, SyncBatchID: &b.ID, Operation: m.Operation, ErrorCode: code, Error: item.Error, ItemKey: item.ItemKey, Attempts: m.Attempts, Status: "OPEN"}
		if e := r.Create(&ie); e != nil {
			return e
		}
	}
	b.ProcessedCount = d.ProcessedCount
	b.ErrorCount = len(d.Errors)
	b.Status = "COMPLETED"
	if b.ErrorCount > 0 {
		b.Status = "COMPLETED_WITH_ERRORS"
	}
	if e := r.Save(&b, b.ID); e != nil {
		return e
	}
	return s.recalculateJob(r, b.JobID)
}

func syncItemKey(item map[string]interface{}, index int) string {
	for _, field := range []string{"id", "id_produto", "codigo_local", "productId", "localCode"} {
		if value, ok := item[field].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return strconv.Itoa(index)
}
func (s *Service) recalculateJob(r *repositories.TenantRepository, id uuid.UUID) error {
	var j models.SyncJob
	if e := r.Find(&j, id, true); e != nil {
		return e
	}
	if j.Status == "CANCELLED" {
		return nil
	}
	batches := []models.SyncBatch{}
	if e := r.All(&batches, map[string]interface{}{"id_sync_job": id}); e != nil {
		return e
	}
	j.ProcessedItems = 0
	j.ErrorItems = 0
	done := j.SourceFinished
	for _, b := range batches {
		j.ProcessedItems += b.ProcessedCount
		j.ErrorItems += b.ErrorCount
		if b.Status == "PENDING" {
			done = false
		}
	}
	j.Status = "RUNNING"
	j.FinishedAt = nil
	if done && j.ProcessedItems+j.ErrorItems == j.TotalItems {
		j.Status = "COMPLETED"
		if j.ErrorItems > 0 {
			j.Status = "COMPLETED_WITH_ERRORS"
		}
		now := time.Now().UTC()
		j.FinishedAt = &now
	}
	return r.Save(&j, j.ID)
}
func (s *Service) CancelJob(ctx context.Context, i models.Identity, id uuid.UUID) (models.SyncJob, error) {
	var j models.SyncJob
	e := s.Repo.Atomic(ctx, i.CompanyID, func(r *repositories.TenantRepository) error {
		// Message locks precede the job lock, matching ACK/failure and avoiding inverted locks.
		messages, e := r.PendingJobMessages(id)
		if e != nil {
			return e
		}
		if e := r.Find(&j, id, true); e != nil {
			return e
		}
		if j.Status == "CANCELLED" {
			return nil
		}
		if j.Status == "COMPLETED" || j.Status == "COMPLETED_WITH_ERRORS" {
			return models.Error(409, "Carga já concluída")
		}
		for _, m := range messages {
			if m.Status == "PROCESSING" {
				return models.Error(409, "Aguarde o lote em processamento antes de cancelar")
			}
		}
		now := time.Now().UTC()
		j.Status = "CANCELLED"
		j.FinishedAt = &now
		for idx := range messages {
			m := &messages[idx]
			m.Status = "FAILED"
			m.LeaseUntil = nil
			m.NextAttemptAt = nil
			m.LastError = "Carga cancelada"
			if e := r.Save(m, m.ID); e != nil {
				return e
			}
		}
		if e := r.Save(&j, j.ID); e != nil {
			return e
		}
		return audit(r, i, "sync.cancelled", &j.DestinationStoreID, map[string]interface{}{"jobId": id})
	})
	j.SetProgress()
	return j, e
}
func (s *Service) RestartJob(ctx context.Context, i models.Identity, id uuid.UUID) (models.SyncJob, error) {
	var j models.SyncJob
	if e := s.Repo.Scoped(ctx, i.CompanyID).Find(&j, id, false); e != nil {
		return j, e
	}
	if j.Status == "RUNNING" || j.Status == "PENDING" {
		return j, models.Error(409, "Cancele ou conclua a carga antes de refazer")
	}
	out, e := s.CreateJob(ctx, i, models.JobDTO{OriginStoreID: j.OriginStoreID, DestinationStoreID: j.DestinationStoreID, Entity: j.Entity, BatchSize: j.BatchSize})
	return out, e
}
func (s *Service) Job(ctx context.Context, i models.Identity, id uuid.UUID) (models.SyncJob, error) {
	var j models.SyncJob
	e := s.Repo.Scoped(ctx, i.CompanyID).Find(&j, id, false)
	j.SetProgress()
	return j, e
}
