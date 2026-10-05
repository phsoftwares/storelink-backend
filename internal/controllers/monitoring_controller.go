package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/phsoftwares/storelink-backend/internal/services"
)

type IMonitoringController interface{ Dashboard(*gin.Context) }

type MonitoringController struct{ Service services.IMonitoringService }

var _ IMonitoringController = (*MonitoringController)(nil)

// Dashboard godoc
// @Summary Consulta indicadores e estado das lojas
// @Tags Monitoramento
// @Produce json
// @Security BearerAuth
// @Success 200 {object} models.Dashboard
// @Failure 400,401,403,500 {object} models.AppError
// @Router /dashboard [get]
func (c *MonitoringController) Dashboard(ctx *gin.Context) {
	f, ok := Filter(ctx)
	if !ok {
		return
	}
	dashboard, err := c.Service.Dashboard(ctx, Identity(ctx), f)
	respond(ctx, dashboard, err)
}
