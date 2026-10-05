package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/phsoftwares/storelink-backend/internal/models"
	"github.com/phsoftwares/storelink-backend/internal/services"
)

type ITransferController interface {
	List(*gin.Context)
	Transfer(*gin.Context)
	RetryTransfer(*gin.Context)
	CreateIntegrationTransfer(*gin.Context)
	ListIntegrationTransfers(*gin.Context)
	IntegrationTransfer(*gin.Context)
	UpdateIntegrationTransfer(*gin.Context)
}

type TransferController struct {
	Service     services.ITransferService
	Integration services.IIntegrationTransferService
}

var _ ITransferController = (*TransferController)(nil)

// List godoc
// @Summary Lista transferências
// @Tags Transferências
// @Produce json
// @Security BearerAuth
// @Param page query int false "Número da página"
// @Param pageSize query int false "Itens por página"
// @Param q query string false "Número da transferência"
// @Param status query string false "Status da transferência"
// @Param groupId query string false "UUID do grupo"
// @Param storeId query string false "UUID da loja"
// @Param originStoreId query string false "UUID da loja de origem"
// @Param destinationStoreId query string false "UUID da loja de destino"
// @Param from query string false "Data inicial"
// @Param to query string false "Data final"
// @Success 200 {object} models.Page
// @Failure 400,401,403,500 {object} models.AppError
// @Router /transfers [get]
func (c *TransferController) List(ctx *gin.Context) {
	filter, ok := Filter(ctx)
	if !ok {
		return
	}
	page, err := c.Service.ListTransfers(ctx, Identity(ctx), filter)
	respond(ctx, page, err)
}

// Transfer godoc
// @Summary Consulta transferência, itens e timeline
// @Description Operação isolada por empresa. Identidade e autorização validadas pelo servidor.
// @Tags Transferências
// @Produce json
// @Security BearerAuth
// @Param id path string true "UUID do recurso"
// @Success 200 {object} map[string]interface{}
// @Failure 400,401,403,404,409,429,500 {object} models.AppError
// @Router /transfers/{id} [get]
func (c *TransferController) Transfer(ctx *gin.Context) {
	id, ok := ID(ctx)
	if !ok {
		return
	}
	v, e := c.Service.TransferDetail(ctx, Identity(ctx), id)
	respond(ctx, v, e)
}

// RetryTransfer godoc
// @Summary Reenvia transferência após corrigir o catálogo de destino
// @Description Cria uma nova tentativa preservando transferência e itens originais.
// @Tags Transferências
// @Produce json
// @Security BearerAuth
// @Param id path string true "UUID da transferência"
// @Success 200 {object} models.Message
// @Failure 400,401,403,404,409,500 {object} models.AppError
// @Router /transfers/{id}/retry [post]
func (c *TransferController) RetryTransfer(ctx *gin.Context) {
	id, ok := ID(ctx)
	if !ok {
		return
	}
	v, err := c.Service.RetryTransfer(ctx, Identity(ctx), id)
	respond(ctx, v, err)
}

// CreateIntegrationTransfer godoc
// @Summary Publish an idempotent transfer from FNTS
// @Tags Direct integration
// @Accept json
// @Produce json
// @Security AgentAPIKey
// @Param X-Agent-ID header string true "Store credential ID"
// @Param body body models.IntegrationTransferDTO true "Transfer and full line snapshots"
// @Success 200 {object} models.TransferSubmission
// @Success 201 {object} models.TransferSubmission
// @Failure 400,401,403,404,409 {object} models.AppError
// @Router /integracao/v1/transferencias [post]
func (c *TransferController) CreateIntegrationTransfer(ctx *gin.Context) {
	var request models.IntegrationTransferDTO
	if !BindStrict(ctx, &request) {
		return
	}
	result, err := c.Integration.CreateIntegrationTransfer(ctx, Identity(ctx), request)
	if err != nil {
		HandleError(ctx, err)
		return
	}
	status := 201
	if result.Duplicate {
		status = 200
	}
	ctx.JSON(status, result)
}

// ListIntegrationTransfers godoc
// @Summary List transfers addressed to this store
// @Tags Direct integration
// @Produce json
// @Security AgentAPIKey
// @Param X-Agent-ID header string true "Store credential ID"
// @Param page query int false "Page number"
// @Param pageSize query int false "Rows per page, up to 100"
// @Param status query string false "Transfer status"
// @Success 200 {object} models.Page
// @Failure 400,401,403 {object} models.AppError
// @Router /integracao/v1/transferencias [get]
func (c *TransferController) ListIntegrationTransfers(ctx *gin.Context) {
	filter, ok := Filter(ctx)
	if !ok {
		return
	}
	page, err := c.Integration.ListIntegrationTransfers(ctx, Identity(ctx), filter)
	if err != nil {
		HandleError(ctx, err)
		return
	}
	items, ok := transferPageItems(page.Items)
	if !ok {
		HandleError(ctx, models.Error(500, "Resposta de transferências inválida"))
		return
	}
	result := models.IntegrationTransferPage{
		Total: page.Total, Page: page.Page, PageSize: page.PageSize,
		Items: make([]models.IntegrationTransferView, 0, len(items)),
	}
	for _, item := range items {
		result.Items = append(result.Items, toIntegrationTransferView(item))
	}
	ctx.JSON(200, result)
}

func transferPageItems(value interface{}) ([]models.Transfer, bool) {
	switch items := value.(type) {
	case *[]models.Transfer:
		if items == nil {
			return nil, true
		}
		return *items, true
	case []models.Transfer:
		return items, true
	default:
		return nil, false
	}
}

// IntegrationTransfer godoc
// @Summary Read a transfer involving this store
// @Tags Direct integration
// @Produce json
// @Security AgentAPIKey
// @Param X-Agent-ID header string true "Store credential ID"
// @Param id path string true "Transfer UUID"
// @Success 200 {object} map[string]interface{}
// @Failure 400,401,403,404 {object} models.AppError
// @Router /integracao/v1/transferencias/{id} [get]
func (c *TransferController) IntegrationTransfer(ctx *gin.Context) {
	parsed, err := uuid.Parse(ctx.Param("id"))
	if err != nil || parsed == uuid.Nil {
		HandleError(ctx, models.Error(400, "Identificador inválido"))
		return
	}
	result, err := c.Integration.IntegrationTransferDetail(ctx, Identity(ctx), parsed)
	if err != nil {
		HandleError(ctx, err)
		return
	}
	view, err := toIntegrationTransferDetail(result)
	if err != nil {
		HandleError(ctx, err)
		return
	}
	ctx.JSON(200, view)
}

// UpdateIntegrationTransfer godoc
// @Summary Record transfer receipt, failure, or origin cancellation
// @Tags Direct integration
// @Accept json
// @Produce json
// @Security AgentAPIKey
// @Param X-Agent-ID header string true "Store credential ID"
// @Param id path string true "Transfer UUID"
// @Param body body models.TransferStatusDTO true "New status and optional item errors"
// @Success 200 {object} models.Transfer
// @Failure 400,401,403,404,409 {object} models.AppError
// @Router /integracao/v1/transferencias/{id} [patch]
func (c *TransferController) UpdateIntegrationTransfer(ctx *gin.Context) {
	parsed, err := uuid.Parse(ctx.Param("id"))
	if err != nil || parsed == uuid.Nil {
		HandleError(ctx, models.Error(400, "Identificador inválido"))
		return
	}
	var request models.TransferStatusDTO
	if !BindStrict(ctx, &request) {
		return
	}
	result, err := c.Integration.UpdateIntegrationTransfer(ctx, Identity(ctx), parsed, request)
	if err != nil {
		HandleError(ctx, err)
		return
	}
	ctx.JSON(200, toIntegrationTransferView(result))
}

func toIntegrationTransferView(transfer models.Transfer) models.IntegrationTransferView {
	return models.IntegrationTransferView{
		ID: transfer.ID, OriginStoreID: transfer.OriginStoreID,
		DestinationStoreID: transfer.DestinationStoreID, Number: transfer.Numero,
		OriginCNPJ: transfer.OriginCNPJ, DestinationCNPJ: transfer.DestinationCNPJ,
		SenderUsername: transfer.SenderUsername, SenderNote: transfer.SenderNote,
		SentAt: transfer.SentAt, Status: transfer.Status, ItemCount: transfer.ItemCount,
		SyncedAt: transfer.SyncedAt, ReceivedAt: transfer.ReceivedAt,
	}
}

func toIntegrationTransferItemView(item models.TransferItem) models.IntegrationTransferItemView {
	return models.IntegrationTransferItemView{
		ID: item.ID, TransferID: item.TransferID, ProductID: item.ProductID,
		SourceProductKey: item.SourceProductKey, Barcode: item.Barcode, SKU: item.SKU,
		Description: item.Description, Unit: item.Unit, Quantity: item.Quantity,
		SalePrice: item.SalePrice, CostPrice: item.CostPrice, BatchCode: item.BatchCode,
		LotSerial: item.LotSerial, Lot: item.Lot, Serial: item.Serial,
		ExpiresAt: item.ExpiresAt, ManufacturedAt: item.ManufacturedAt,
		Controlled: item.Controlled, TracksBatch: item.TracksBatch,
		TracksSerial: item.TracksSerial, RegistrationMS: item.RegistrationMS,
		TherapeuticClass: item.TherapeuticClass, SNGPCType: item.SNGPCType,
		SNGPCUnit: item.SNGPCUnit, PricesInformed: item.PricesInformed,
		PriceTablePayload:      item.PriceTablePayload,
		UsesPriceTableInformed: item.UsesPriceTableInformed,
		UsesPriceTable:         item.UsesPriceTable, Status: item.Status,
		ErrorCode: item.ErrorCode, ErrorDetail: item.ErrorDetail,
	}
}

func toIntegrationTransferDetail(value interface{}) (models.IntegrationTransferDetail, error) {
	detail, ok := value.(map[string]interface{})
	if !ok {
		return models.IntegrationTransferDetail{}, models.Error(500, "Detalhe de transferência inválido")
	}
	transfer, ok := detail["transfer"].(models.Transfer)
	if !ok {
		return models.IntegrationTransferDetail{}, models.Error(500, "Transferência ausente no detalhe")
	}
	items, ok := detail["items"].([]models.TransferItem)
	if !ok {
		return models.IntegrationTransferDetail{}, models.Error(500, "Itens ausentes no detalhe da transferência")
	}
	timeline, ok := detail["timeline"].([]map[string]interface{})
	if !ok {
		timeline = []map[string]interface{}{}
	}
	integrationTimeline := make([]map[string]interface{}, 0, len(timeline))
	for _, entry := range timeline {
		integrationEntry := make(map[string]interface{}, len(entry))
		for key, value := range entry {
			switch key {
			case "event":
				integrationEntry["evento"] = value
			case "createdAt":
				integrationEntry["criado_em"] = value
			case "detail":
				integrationEntry["detalhe"] = value
			default:
				integrationEntry[key] = value
			}
		}
		integrationTimeline = append(integrationTimeline, integrationEntry)
	}
	result := models.IntegrationTransferDetail{
		Transfer: toIntegrationTransferView(transfer),
		Items:    make([]models.IntegrationTransferItemView, 0, len(items)),
		Timeline: integrationTimeline,
	}
	for _, item := range items {
		result.Items = append(result.Items, toIntegrationTransferItemView(item))
	}
	return result, nil
}
