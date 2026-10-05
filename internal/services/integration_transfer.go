package services

import (
	"context"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/phsoftwares/storelink-backend/internal/models"
	"github.com/phsoftwares/storelink-backend/internal/repositories"
)

type IIntegrationTransferService interface {
	CreateIntegrationTransfer(context.Context, models.Identity, models.IntegrationTransferDTO) (models.TransferSubmission, error)
	ListIntegrationTransfers(context.Context, models.Identity, repositories.Filter) (models.Page, error)
	IntegrationTransferDetail(context.Context, models.Identity, uuid.UUID) (interface{}, error)
	UpdateIntegrationTransfer(context.Context, models.Identity, uuid.UUID, models.TransferStatusDTO) (models.Transfer, error)
}

var _ IIntegrationTransferService = (*Service)(nil)

func (s *Service) CreateIntegrationTransfer(ctx context.Context, identity models.Identity, request models.IntegrationTransferDTO) (models.TransferSubmission, error) {
	result := models.TransferSubmission{ID: request.ID, Number: strings.TrimSpace(request.Number), Status: "PENDING", ItemCount: len(request.Items)}
	if !identity.IsAgent() || identity.StoreID == uuid.Nil {
		return result, models.Error(403, "Integrações de loja exigem uma credencial de loja")
	}
	if err := validateIntegrationTransfer(request); err != nil {
		return result, err
	}
	request.Number = strings.TrimSpace(request.Number)
	request.OriginCNPJ = onlyDigits(request.OriginCNPJ)
	request.DestinationCNPJ = onlyDigits(request.DestinationCNPJ)
	request.SenderUsername = strings.TrimSpace(request.SenderUsername)
	request.SenderNote = strings.TrimSpace(request.SenderNote)
	request.SentAt = request.SentAt.UTC()
	result.Number = request.Number
	payloadHash := hashJSON(request)
	err := s.Repo.Atomic(ctx, identity.CompanyID, func(r *repositories.TenantRepository) error {
		if err := r.Advisory(request.ID); err != nil {
			return err
		}
		if request.DestinationID == uuid.Nil {
			if request.DestinationCNPJ == "" {
				return models.Error(400, "destinationStoreId ou destinationCnpj deve ser informado")
			}
			store, found, err := r.StoreByCNPJ(request.DestinationCNPJ)
			if err != nil {
				return err
			}
			if !found {
				return models.Error(404, "Loja destino nao encontrada pelo CNPJ")
			}
			request.DestinationID = store.ID
		}
		if _, err := route(r, identity.StoreID, request.DestinationID); err != nil {
			return err
		}
		var originStore, destinationStore models.Store
		if err := r.Find(&originStore, identity.StoreID, false); err != nil {
			return err
		}
		if err := r.Find(&destinationStore, request.DestinationID, false); err != nil {
			return err
		}
		if request.OriginCNPJ == "" {
			request.OriginCNPJ = originStore.CNPJ
		}
		if request.DestinationCNPJ == "" {
			request.DestinationCNPJ = destinationStore.CNPJ
		}
		var existing models.Transfer
		if err := r.Find(&existing, request.ID, true); err == nil {
			if existing.OriginStoreID != identity.StoreID || existing.PayloadHash != payloadHash {
				return models.Error(409, "TRANSFER_ID_CONFLICT: id já utilizado com outra origem ou conteúdo")
			}
			result = models.TransferSubmission{ID: existing.ID, Number: existing.Numero, Status: existing.Status, ItemCount: existing.ItemCount, Duplicate: true}
			return nil
		} else if !repositories.IsNotFound(err) {
			return err
		}
		transfer := models.Transfer{
			Tenant: models.NewTenant(identity.CompanyID), OriginStoreID: identity.StoreID,
			DestinationStoreID: request.DestinationID, Numero: request.Number,
			OriginCNPJ: request.OriginCNPJ, DestinationCNPJ: request.DestinationCNPJ,
			SenderUsername: request.SenderUsername, SenderNote: request.SenderNote,
			SentAt: &request.SentAt, Status: "PENDING", ItemCount: len(request.Items), PayloadHash: payloadHash,
		}
		transfer.ID = request.ID
		if err := r.Create(&transfer); err != nil {
			return err
		}
		items := make([]models.TransferItem, 0, len(request.Items))
		for _, line := range request.Items {
			productID, err := resolveTransferProduct(r, identity.StoreID, line.ProductID, line.LocalProductCode, line.SKU, line.Barcode)
			if err != nil {
				return err
			}
			item := models.TransferItem{
				Tenant: models.NewTenant(identity.CompanyID), TransferID: transfer.ID, ProductID: productID,
				SourceProductKey: strings.TrimSpace(line.LocalProductCode), Barcode: strings.TrimSpace(line.Barcode),
				SKU: strings.TrimSpace(line.SKU), Description: strings.TrimSpace(line.Name), Unit: strings.TrimSpace(line.Unit),
				Quantity: line.Quantity, SalePrice: line.SalePrice, CostPrice: line.CostPrice,
				BatchCode: strings.TrimSpace(line.BatchCode), LotSerial: strings.TrimSpace(line.LotSerial),
				Lot: strings.TrimSpace(line.Lot), Serial: strings.TrimSpace(line.Serial),
				ExpiresAt: line.ExpiresAt, ManufacturedAt: line.ManufacturedAt,
				Controlled: *line.Controlled, TracksBatch: *line.TracksBatch, TracksSerial: *line.TracksSerial,
				RegistrationMS: strings.TrimSpace(line.RegistrationMS), TherapeuticClass: strings.TrimSpace(line.TherapeuticClass),
				SNGPCType: strings.TrimSpace(line.SNGPCType), SNGPCUnit: strings.TrimSpace(line.SNGPCUnit),
				PricesInformed: boolValue(line.PricesInformed), PriceTablePayload: strings.TrimSpace(line.PriceTablePayload),
				UsesPriceTableInformed: boolValue(line.UsesPriceTableInformed), UsesPriceTable: strings.TrimSpace(line.UsesPriceTable), Status: "PENDING",
			}
			item.ID = line.ID
			items = append(items, item)
		}
		if err := r.CreateMany(&items, 500); err != nil {
			return err
		}
		if err := audit(r, identity, "transfer.created", &identity.StoreID, map[string]interface{}{"transferId": transfer.ID, "destinationStoreId": transfer.DestinationStoreID, "itemCount": transfer.ItemCount}); err != nil {
			return err
		}
		return nil
	})
	return result, err
}

func (s *Service) ListIntegrationTransfers(ctx context.Context, identity models.Identity, filter repositories.Filter) (models.Page, error) {
	if !identity.IsAgent() || identity.StoreID == uuid.Nil {
		return models.Page{}, models.Error(403, "Credencial de loja obrigatória")
	}
	filter.DestinationID = identity.StoreID
	filter.OriginID = uuid.Nil
	filter.StoreID = uuid.Nil
	filter.GroupID = uuid.Nil
	if filter.Status == "" {
		filter.Status = "PENDING"
	}
	return s.Repo.ListTransfers(ctx, identity.CompanyID, filter)
}

func (s *Service) IntegrationTransferDetail(ctx context.Context, identity models.Identity, id uuid.UUID) (interface{}, error) {
	if !identity.IsAgent() || identity.StoreID == uuid.Nil {
		return nil, models.Error(403, "Credencial de loja obrigatória")
	}
	r := s.Repo.Scoped(ctx, identity.CompanyID)
	var transfer models.Transfer
	if err := r.Find(&transfer, id, false); err != nil {
		return nil, err
	}
	if transfer.OriginStoreID != identity.StoreID && transfer.DestinationStoreID != identity.StoreID {
		return nil, models.Error(404, "Transferência não encontrada")
	}
	return s.TransferDetail(ctx, identity, id)
}

func (s *Service) UpdateIntegrationTransfer(ctx context.Context, identity models.Identity, id uuid.UUID, request models.TransferStatusDTO) (models.Transfer, error) {
	var transfer models.Transfer
	err := s.Repo.Atomic(ctx, identity.CompanyID, func(r *repositories.TenantRepository) error {
		if err := r.Find(&transfer, id, true); err != nil {
			return err
		}
		if !identity.IsAgent() || (transfer.OriginStoreID != identity.StoreID && transfer.DestinationStoreID != identity.StoreID) {
			return models.Error(404, "Transferência não encontrada")
		}
		status := strings.ToUpper(strings.TrimSpace(request.Status))
		switch status {
		case "RECEIVED":
			if transfer.DestinationStoreID != identity.StoreID {
				return models.Error(403, "Somente a loja de destino pode confirmar o recebimento")
			}
			if transfer.Status == "CANCELLED" {
				return models.Error(409, "Transferência cancelada")
			}
			if transfer.Status != "RECEIVED" {
				now := time.Now().UTC()
				transfer.Status = "RECEIVED"
				transfer.ReceivedAt = &now
				transfer.SyncedAt = &now
				if err := r.Update(&models.TransferItem{}, map[string]interface{}{"id_transferencia": id}, map[string]interface{}{"status": "STORED", "error_code": "", "erro_detalhe": ""}); err != nil {
					return err
				}
			}
		case "FAILED":
			if transfer.DestinationStoreID != identity.StoreID {
				return models.Error(403, "Somente a loja de destino pode informar falha no recebimento")
			}
			if transfer.Status == "RECEIVED" || transfer.Status == "CANCELLED" {
				return models.Error(409, "Transferência finalizada")
			}
			if err := applyTransferFailures(r, id, request.ItemErrors, request.Error); err != nil {
				return err
			}
			now := time.Now().UTC()
			transfer.Status = "FAILED"
			transfer.SyncedAt = &now
		case "CANCELLED":
			if transfer.OriginStoreID != identity.StoreID {
				return models.Error(403, "Somente a loja de origem pode cancelar a transferência")
			}
			if transfer.Status == "RECEIVED" {
				return models.Error(409, "Transferência já recebida")
			}
			transfer.Status = "CANCELLED"
		default:
			return models.Error(400, "status deve ser RECEIVED, FAILED ou CANCELLED")
		}
		if err := r.Save(&transfer, transfer.ID); err != nil {
			return err
		}
		return audit(r, identity, "transfer."+strings.ToLower(status), &identity.StoreID, map[string]interface{}{"transferId": id, "status": status})
	})
	return transfer, err
}

func validateIntegrationTransfer(request models.IntegrationTransferDTO) error {
	if request.ID == uuid.Nil || (request.DestinationID == uuid.Nil && strings.TrimSpace(request.DestinationCNPJ) == "") || strings.TrimSpace(request.Number) == "" || len(strings.TrimSpace(request.Number)) > 100 || request.SentAt.IsZero() || len(request.Items) < 1 || len(request.Items) > 1000 {
		return models.Error(400, "Transferência exige id, destinationStoreId, number, sentAt e de 1 a 1000 itens")
	}
	if len(request.OriginCNPJ) > 20 || len(request.DestinationCNPJ) > 20 || len(request.SenderUsername) > 100 || len(request.SenderNote) > 2000 {
		return models.Error(400, "Dados de identificação da transferência fora do limite")
	}
	seen := make(map[uuid.UUID]struct{}, len(request.Items))
	for _, item := range request.Items {
		if item.ID == uuid.Nil {
			return models.Error(400, "Cada item exige um id estável")
		}
		if _, duplicate := seen[item.ID]; duplicate {
			return models.Error(400, "A transferência contém item duplicado")
		}
		seen[item.ID] = struct{}{}
		if item.Quantity <= 0 || math.IsNaN(item.Quantity) || math.IsInf(item.Quantity, 0) || item.SalePrice < 0 || math.IsNaN(item.SalePrice) || math.IsInf(item.SalePrice, 0) || item.CostPrice < 0 || math.IsNaN(item.CostPrice) || math.IsInf(item.CostPrice, 0) {
			return models.Error(400, "Quantidade e preços do item devem ser números válidos e não negativos")
		}
		if strings.TrimSpace(item.Name) == "" || len(item.Name) > 255 || len(item.Unit) > 20 || len(item.LocalProductCode) > 100 || len(item.SKU) > 100 || len(item.Barcode) > 32 || len(item.BatchCode) > 100 || len(item.LotSerial) > 100 || len(item.Lot) > 100 || len(item.Serial) > 100 || len(item.RegistrationMS) > 100 || len(item.TherapeuticClass) > 100 || len(item.SNGPCType) > 50 || len(item.SNGPCUnit) > 20 || len(item.PriceTablePayload) > 10000 || len(item.UsesPriceTable) > 20 {
			return models.Error(400, "Identificação ou dados do item de transferência fora do limite")
		}
		if (item.ProductID == nil || *item.ProductID == uuid.Nil) && strings.TrimSpace(item.LocalProductCode) == "" && strings.TrimSpace(item.SKU) == "" && strings.TrimSpace(item.Barcode) == "" {
			return models.Error(400, "Cada item deve informar productId, localProductCode, sku ou barcode")
		}
		if item.Controlled == nil || item.TracksBatch == nil || item.TracksSerial == nil {
			return models.Error(400, "controlled, tracksBatch e tracksSerial devem ser booleanos explícitos")
		}
		if item.ManufacturedAt != nil && item.ExpiresAt != nil && item.ManufacturedAt.After(*item.ExpiresAt) {
			return models.Error(400, "manufacturedAt não pode ser posterior a expiresAt")
		}
	}
	return nil
}

func applyTransferFailures(r *repositories.TenantRepository, transferID uuid.UUID, failures []models.IntegrationItemError, fallback string) error {
	if len(failures) == 0 {
		return models.Error(400, "FAILED exige pelo menos um erro por item")
	}
	items := []models.TransferItem{}
	if err := r.All(&items, map[string]interface{}{"id_transferencia": transferID}); err != nil {
		return err
	}
	byID := make(map[string]*models.TransferItem, len(items))
	for index := range items {
		items[index].Status = "STORED"
		items[index].ErrorCode = ""
		items[index].ErrorDetail = ""
		byID[items[index].ID.String()] = &items[index]
	}
	seen := make(map[string]struct{}, len(failures))
	for _, failure := range failures {
		key := strings.TrimSpace(failure.ItemKey)
		item := byID[key]
		if item == nil || strings.TrimSpace(failure.Error) == "" || len(failure.Error) > 2000 || !errorCodePattern.MatchString(failure.ErrorCode) {
			return models.Error(400, "itemErrors precisa referenciar itens desta transferência com errorCode e error válidos")
		}
		if _, duplicate := seen[key]; duplicate {
			return models.Error(400, "itemErrors contém item duplicado")
		}
		seen[key] = struct{}{}
		item.Status = "FAILED"
		item.ErrorCode = failure.ErrorCode
		item.ErrorDetail = failure.Error
	}
	for index := range items {
		if err := r.Save(&items[index], items[index].ID); err != nil {
			return err
		}
	}
	return nil
}
