package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/phsoftwares/storelink-backend/internal/services"
)

type IAuditController interface {
	List(*gin.Context)
}

type AuditController struct{ Service services.IAuditService }

var _ IAuditController = (*AuditController)(nil)

// List godoc
// @Summary Lista auditoria da empresa
// @Tags Auditoria
// @Produce json
// @Security BearerAuth
// @Param page query int false "Número da página"
// @Param pageSize query int false "Itens por página"
// @Param storeId query string false "UUID da loja"
// @Param from query string false "Data inicial"
// @Param to query string false "Data final"
// @Success 200 {object} models.Page
// @Failure 400,401,403,500 {object} models.AppError
// @Router /audit [get]
func (c *AuditController) List(ctx *gin.Context) {
	filter, ok := Filter(ctx)
	if !ok {
		return
	}
	page, err := c.Service.ListAudit(ctx, Identity(ctx), filter)
	respond(ctx, page, err)
}
