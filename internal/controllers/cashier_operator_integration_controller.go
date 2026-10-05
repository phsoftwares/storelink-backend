package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/phsoftwares/storelink-backend/internal/models"
	"github.com/phsoftwares/storelink-backend/internal/services"
)

type ICashierOperatorIntegrationController interface {
	UpsertCashierOperator(*gin.Context)
	UpsertCashierOperatorBatch(*gin.Context)
}

type CashierOperatorIntegrationController struct {
	Service services.ICashierOperatorIntegrationService
}

var _ ICashierOperatorIntegrationController = (*CashierOperatorIntegrationController)(nil)

// UpsertCashierOperator godoc
// @Summary Create or update one cashier operator
// @Tags Direct integration
// @Accept json
// @Produce json
// @Security AgentAPIKey
// @Param X-Agent-ID header string true "Store credential ID"
// @Param localCode path string true "Cashier operator code in this store"
// @Param body body models.CashierOperatorSnapshot true "Cashier operator snapshot"
// @Success 200 {object} models.CatalogWriteResult
// @Failure 400,401,403,409 {object} models.AppError
// @Router /integracao/v1/operadores-caixa/{codigo_local} [put]
func (c *CashierOperatorIntegrationController) UpsertCashierOperator(ctx *gin.Context) {
	var snapshot models.CashierOperatorSnapshot
	if !BindStrict(ctx, &snapshot) {
		return
	}
	result, err := c.Service.UpsertCashierOperator(ctx, Identity(ctx), snapshot, ctx.Param("codigo_local"))
	respond(ctx, result, err)
}

// UpsertCashierOperatorBatch godoc
// @Summary Create or update cashier operators in an atomic batch of at most 1000
// @Tags Direct integration
// @Accept json
// @Produce json
// @Security AgentAPIKey
// @Param X-Agent-ID header string true "Store credential ID"
// @Param body body models.CashierOperatorSnapshotBatchDTO true "Cashier operator batch"
// @Success 200 {object} models.CatalogBatchResult
// @Failure 400,401,403,409 {object} models.AppError
// @Router /integracao/v1/operadores-caixa/lotes [post]
func (c *CashierOperatorIntegrationController) UpsertCashierOperatorBatch(ctx *gin.Context) {
	var request models.CashierOperatorSnapshotBatchDTO
	if !BindStrict(ctx, &request) {
		return
	}
	result, err := c.Service.UpsertCashierOperators(ctx, Identity(ctx), request.Items)
	respond(ctx, result, err)
}
