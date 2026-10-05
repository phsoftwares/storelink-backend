package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/phsoftwares/storelink-backend/internal/models"
	"github.com/phsoftwares/storelink-backend/internal/services"
)

type DomainIntegrationController struct {
	Service services.IDomainPublicationService
}

func (c *DomainIntegrationController) Publish(ctx *gin.Context) {
	var request models.DomainPublicationDTO
	if !BindStrict(ctx, &request) {
		return
	}
	result, err := c.Service.PublishDomain(ctx, Identity(ctx), request)
	if err != nil {
		HandleError(ctx, err)
		return
	}
	ctx.JSON(201, result)
}

// PublishBatch godoc
// @Summary Publica ate 1000 cadastros normalizados em uma chamada
// @Tags Domain integration
// @Accept json
// @Produce json
// @Security AgentAPIKey
// @Param X-Agent-ID header string true "Store credential ID"
// @Param body body models.DomainPublicationBatchDTO true "Normalized master-data publications"
// @Success 200 {object} models.DomainPublicationBatchResult
// @Failure 400,401,403,409 {object} models.AppError
// @Router /integracao/v1/publicacoes-cadastros/lotes [post]
func (c *DomainIntegrationController) PublishBatch(ctx *gin.Context) {
	var request models.DomainPublicationBatchDTO
	if !BindStrict(ctx, &request) {
		return
	}
	result, err := c.Service.PublishDomains(ctx, Identity(ctx), request.Items)
	respond(ctx, result, err)
}
