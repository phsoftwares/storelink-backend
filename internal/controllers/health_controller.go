package controllers

import "github.com/gin-gonic/gin"

type IHealthController interface{ Health(*gin.Context) }

type HealthController struct{}

var _ IHealthController = (*HealthController)(nil)

// Health godoc
// @Summary Verifica disponibilidade HTTP
// @Description Endpoint público de liveness. Readiness consulta PostgreSQL separadamente.
// @Tags Saúde
// @Produce json
// @Success 200 {object} map[string]string
// @Router /health [get]
func (c *HealthController) Health(ctx *gin.Context) { ctx.JSON(200, gin.H{"status": "ok"}) }
