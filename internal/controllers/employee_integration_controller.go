package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/phsoftwares/storelink-backend/internal/models"
	"github.com/phsoftwares/storelink-backend/internal/services"
)

type IEmployeeIntegrationController interface {
	UpsertEmployee(*gin.Context)
	UpsertEmployeeBatch(*gin.Context)
}

type EmployeeIntegrationController struct {
	Service services.IEmployeeIntegrationService
}

var _ IEmployeeIntegrationController = (*EmployeeIntegrationController)(nil)

// UpsertEmployee godoc
// @Summary Create or update one canonical employee and its cashier operators
// @Tags Direct integration
// @Accept json
// @Produce json
// @Security AgentAPIKey
// @Param X-Agent-ID header string true "Store credential ID"
// @Param localCode path string true "Employee code in this store"
// @Param body body models.EmployeeSnapshot true "Employee snapshot"
// @Success 200 {object} models.EmployeeSnapshot
// @Failure 400,401,403,409 {object} models.AppError
// @Router /integracao/v1/funcionarios/{codigo_local} [put]
func (c *EmployeeIntegrationController) UpsertEmployee(ctx *gin.Context) {
	var snapshot models.EmployeeSnapshot
	if !BindStrict(ctx, &snapshot) {
		return
	}
	result, err := c.Service.UpsertEmployee(ctx, Identity(ctx), snapshot, ctx.Param("codigo_local"))
	respond(ctx, result, err)
}

// UpsertEmployeeBatch godoc
// @Summary Create or update employees in an atomic batch of at most 1000
// @Tags Direct integration
// @Accept json
// @Produce json
// @Security AgentAPIKey
// @Param X-Agent-ID header string true "Store credential ID"
// @Param body body models.EmployeeBatchDTO true "Employee batch"
// @Success 200 {object} models.CatalogBatchResult
// @Failure 400,401,403,409 {object} models.AppError
// @Router /integracao/v1/funcionarios/lotes [post]
func (c *EmployeeIntegrationController) UpsertEmployeeBatch(ctx *gin.Context) {
	var request models.EmployeeBatchDTO
	if !BindStrict(ctx, &request) {
		return
	}
	result, err := c.Service.UpsertEmployees(ctx, Identity(ctx), request.Items)
	respond(ctx, result, err)
}
