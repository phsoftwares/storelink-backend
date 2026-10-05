package services

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/phsoftwares/storelink-backend/internal/models"
	"github.com/phsoftwares/storelink-backend/internal/repositories"
)

type ITransferService interface {
	ListTransfers(context.Context, models.Identity, repositories.Filter) (models.Page, error)
	TransferDetail(context.Context, models.Identity, uuid.UUID) (interface{}, error)
	RetryTransfer(context.Context, models.Identity, uuid.UUID) (models.Message, error)
}

func (s *Service) ListTransfers(ctx context.Context, i models.Identity, filter repositories.Filter) (models.Page, error) {
	return s.Repo.ListTransfers(ctx, i.CompanyID, filter)
}

// RetryTransfer creates a new delivery attempt from the durable transfer and
// item rows. The receiving Agent can then resolve the product identities
// again after its local catalog has been corrected.
func (s *Service) RetryTransfer(ctx context.Context, i models.Identity, id uuid.UUID) (models.Message, error) {
	var message models.Message
	if i.IsAgent() {
		return message, models.Error(403, "Only an administrator can retry a transfer")
	}
	err := s.Repo.Atomic(ctx, i.CompanyID, func(r *repositories.TenantRepository) error {
		var transfer models.Transfer
		if err := r.Find(&transfer, id, true); err != nil {
			return err
		}
		if transfer.Status != "FAILED" {
			return models.Error(409, "Only failed transfers can be retried")
		}
		group, err := route(r, transfer.OriginStoreID, transfer.DestinationStoreID)
		if err != nil {
			return err
		}
		if err := requireGroupIntegration(group, "products"); err != nil {
			return err
		}
		items := []models.TransferItem{}
		if err := r.All(&items, map[string]interface{}{"id_transferencia": transfer.ID}); err != nil {
			return err
		}
		if len(items) == 0 || len(items) != transfer.ItemCount {
			return models.Error(409, "Transfer items are incomplete and cannot be retried")
		}
		payload := models.TransferPayload{ID: transfer.ID, Number: transfer.Numero, OriginCNPJ: transfer.OriginCNPJ, DestinationCNPJ: transfer.DestinationCNPJ, SenderUsername: transfer.SenderUsername, SenderNote: transfer.SenderNote, SentAt: transfer.SentAt, Items: make([]models.TransferLine, 0, len(items))}
		for _, item := range items {
			if (item.ProductID == nil || *item.ProductID == uuid.Nil) && strings.TrimSpace(item.SKU) == "" && strings.TrimSpace(item.Barcode) == "" {
				return models.Error(409, "A transfer item has no stable product identity")
			}
			controlled, tracksBatch, tracksSerial := item.Controlled, item.TracksBatch, item.TracksSerial
			pricesInformed, usesPriceTableInformed := item.PricesInformed, item.UsesPriceTableInformed
			payload.Items = append(payload.Items, models.TransferLine{ID: item.ID, ProductID: item.ProductID, LocalProductCode: item.SourceProductKey, SKU: item.SKU, Barcode: item.Barcode, Name: item.Description, Unit: item.Unit, Quantity: item.Quantity, SalePrice: item.SalePrice, CostPrice: item.CostPrice, BatchCode: item.BatchCode, LotSerial: item.LotSerial, Lot: item.Lot, Serial: item.Serial, ExpiresAt: item.ExpiresAt, ManufacturedAt: item.ManufacturedAt, Controlled: &controlled, TracksBatch: &tracksBatch, TracksSerial: &tracksSerial, RegistrationMS: item.RegistrationMS, TherapeuticClass: item.TherapeuticClass, SNGPCType: item.SNGPCType, SNGPCUnit: item.SNGPCUnit, PricesInformed: &pricesInformed, PriceTablePayload: item.PriceTablePayload, UsesPriceTableInformed: &usesPriceTableInformed, UsesPriceTable: item.UsesPriceTable})
			item.Status = "PENDING"
			item.ErrorCode = ""
			item.ErrorDetail = ""
			if err := r.Save(&item, item.ID); err != nil {
				return err
			}
		}
		transfer.Status = "PENDING"
		transfer.SyncedAt = nil
		if err := r.Save(&transfer, transfer.ID); err != nil {
			return err
		}
		transferID := transfer.ID
		message = models.Message{Tenant: models.NewTenant(i.CompanyID), Type: "EVENT", Operation: "transfer.created", OriginStoreID: transfer.OriginStoreID, DestinationStoreID: transfer.DestinationStoreID, TransferID: &transferID, Status: "PENDING", Payload: models.ToJSON(payload)}
		if err := r.Create(&message); err != nil {
			return err
		}
		return audit(r, i, "transfer.retry", &transfer.OriginStoreID, map[string]interface{}{"transferId": transfer.ID, "messageId": message.ID})
	})
	return message, err
}
