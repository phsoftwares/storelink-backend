package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/phsoftwares/storelink-backend/internal/models"
	"github.com/phsoftwares/storelink-backend/internal/services"
)

type ExceptionIntegrationController struct {
	Service services.IIntegrationExceptionService
}

func (c *ExceptionIntegrationController) Publish(ctx *gin.Context) {
	var request models.IntegrationExceptionDTO
	if !BindStrict(ctx, &request) {
		return
	}
	result, err := c.Service.PublishException(ctx, Identity(ctx), request)
	if err != nil {
		HandleError(ctx, err)
		return
	}
	ctx.JSON(201, result)
}
