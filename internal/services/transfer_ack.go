package services

import (
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/phsoftwares/storelink-backend/internal/models"
	"github.com/phsoftwares/storelink-backend/internal/repositories"
)

var errorCodePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,79}$`)

func (s *Service) ackTransfer(r *repositories.TenantRepository, i models.Identity, m *models.Message, result models.AckDTO) error {
	payload, err := decodeTransferPayload(m.Payload)
	if err != nil || payload.ID == uuid.Nil {
		return models.Error(400, "Transfer payload is invalid")
	}
	if result.ProcessedCount+len(result.Errors) != len(payload.Items) {
		return models.Error(400, "processed_count plus item errors must equal the transfer item count")
	}
	items := make(map[string]models.TransferLine, len(payload.Items))
	for _, line := range payload.Items {
		if line.ID == uuid.Nil {
			return models.Error(400, "transfer items must have stable item ids")
		}
		key := line.ID.String()
		if _, exists := items[key]; exists {
			return models.Error(400, "transfer item ids must be unique")
		}
		items[key] = line
	}
	itemErrors := make(map[string]models.ItemError, len(result.Errors))
	for _, itemError := range result.Errors {
		if itemError.Error == "" || len(itemError.Error) > 2000 || !errorCodePattern.MatchString(itemError.ErrorCode) {
			return models.Error(400, "transfer item errors require a valid error_code and message")
		}
		if _, exists := items[itemError.ItemKey]; !exists || itemErrors[itemError.ItemKey].ItemKey != "" {
			return models.Error(400, "transfer item errors must refer to unique items in this transfer")
		}
		itemErrors[itemError.ItemKey] = itemError
	}

	var transfer models.Transfer
	if err := r.Find(&transfer, payload.ID, true); err != nil {
		return err
	}
	if transfer.DestinationStoreID != i.StoreID || transfer.ID != *m.TransferID {
		return models.Error(403, "transfer does not belong to the authenticated destination")
	}
	for key := range items {
		itemID, _ := uuid.Parse(key)
		var stored models.TransferItem
		if err := r.Find(&stored, itemID, true); err != nil {
			return err
		}
		if stored.TransferID != transfer.ID {
			return models.Error(400, "transfer item belongs to another transfer")
		}
		if itemError, failed := itemErrors[key]; failed {
			if err := r.Update(&models.IntegrationError{}, map[string]interface{}{"operation": m.Operation, "item_key": key, "status": "OPEN"}, map[string]interface{}{"status": "RESOLVED", "resolved_at": time.Now().UTC()}); err != nil {
				return err
			}
			stored.Status = "FAILED"
			stored.ErrorCode = itemError.ErrorCode
			stored.ErrorDetail = itemError.Error
			integrationError := models.IntegrationError{Tenant: models.NewTenant(i.CompanyID), StoreID: i.StoreID, MessageID: &m.ID, Operation: m.Operation, ErrorCode: itemError.ErrorCode, Error: itemError.Error, ItemKey: key, Attempts: m.Attempts, Status: "OPEN"}
			if err := r.Create(&integrationError); err != nil {
				return err
			}
		} else {
			stored.Status = "STORED"
			stored.ErrorCode = ""
			stored.ErrorDetail = ""
			if err := r.Update(&models.IntegrationError{}, map[string]interface{}{"operation": m.Operation, "item_key": key, "status": "OPEN"}, map[string]interface{}{"status": "RESOLVED", "resolved_at": time.Now().UTC()}); err != nil {
				return err
			}
		}
		if err := r.Save(&stored, stored.ID); err != nil {
			return err
		}
	}
	now := time.Now().UTC()
	transfer.SyncedAt = &now
	if len(itemErrors) == 0 {
		transfer.Status = "SYNCED"
	} else {
		transfer.Status = "FAILED"
	}
	if err := r.Save(&transfer, transfer.ID); err != nil {
		return err
	}
	return audit(r, i, "transfer.destination_ack", &i.StoreID, map[string]interface{}{"transferId": transfer.ID, "processedCount": result.ProcessedCount, "errorCount": len(itemErrors)})
}
