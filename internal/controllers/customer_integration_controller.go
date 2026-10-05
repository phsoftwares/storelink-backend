package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/phsoftwares/storelink-backend/internal/models"
	"github.com/phsoftwares/storelink-backend/internal/services"
)

type ICustomerIntegrationController interface {
	UpsertCustomer(*gin.Context)
	UpsertCustomerBatch(*gin.Context)
}

type CustomerIntegrationController struct {
	Service services.ICustomerIntegrationService
}

var _ ICustomerIntegrationController = (*CustomerIntegrationController)(nil)

// UpsertCustomer godoc
// @Summary Create or update one canonical customer
// @Tags Direct integration
// @Accept json
// @Produce json
// @Security AgentAPIKey
// @Param X-Agent-ID header string true "Store credential ID"
// @Param localCode path string true "Customer code in this store"
// @Param body body models.CustomerSnapshot true "Customer snapshot"
// @Success 200 {object} models.CustomerSnapshot
// @Failure 400,401,403,409 {object} models.AppError
// @Router /integracao/v1/clientes/{codigo_local} [put]
func (c *CustomerIntegrationController) UpsertCustomer(ctx *gin.Context) {
	var snapshot models.CustomerSnapshot
	if !BindStrict(ctx, &snapshot) {
		return
	}
	result, err := c.Service.UpsertCustomer(ctx, Identity(ctx), snapshot, ctx.Param("codigo_local"))
	respond(ctx, result, err)
}

// UpsertCustomerBatch godoc
// @Summary Create or update customers in an atomic batch of at most 1000
// @Tags Direct integration
// @Accept json
// @Produce json
// @Security AgentAPIKey
// @Param X-Agent-ID header string true "Store credential ID"
// @Param body body models.CustomerBatchDTO true "Customer batch"
// @Success 200 {object} models.CatalogBatchResult
// @Failure 400,401,403,409 {object} models.AppError
// @Router /integracao/v1/clientes/lotes [post]
func (c *CustomerIntegrationController) UpsertCustomerBatch(ctx *gin.Context) {
	var request models.CustomerBatchDTO
	if !BindStrict(ctx, &request) {
		return
	}
	result, err := c.Service.UpsertCustomers(ctx, Identity(ctx), request.Items)
	respond(ctx, result, err)
}
