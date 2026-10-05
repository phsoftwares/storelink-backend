package controllers

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/phsoftwares/storelink-backend/internal/models"
	"github.com/phsoftwares/storelink-backend/internal/services"
)

type IProductIntegrationController interface {
	UpsertProduct(*gin.Context)
	UpsertProductBatch(*gin.Context)
	UpsertProductPrice(*gin.Context)
	UpsertProductPriceBatch(*gin.Context)
	PublishProduct(*gin.Context)
	PublishProductBatch(*gin.Context)
	ProductCatalog(*gin.Context)
}

type IProductIntegrationApplication interface {
	services.IProductIntegrationService
	services.IProductPriceIntegrationService
	services.IProductPublicationService
}

type ProductIntegrationController struct {
	Service IProductIntegrationApplication
}

var _ IProductIntegrationController = (*ProductIntegrationController)(nil)

// UpsertProduct godoc
// @Summary Create or update one canonical product
// @Tags Direct integration
// @Accept json
// @Produce json
// @Security AgentAPIKey
// @Param X-Agent-ID header string true "Store credential ID"
// @Param localCode path string true "Product code in this store"
// @Param body body models.ProductSnapshot true "Product snapshot"
// @Success 200 {object} models.ProductSnapshot
// @Failure 400,401,403,409 {object} models.AppError
// @Router /integracao/v1/produtos/{codigo_local} [put]
func (c *ProductIntegrationController) UpsertProduct(ctx *gin.Context) {
	var snapshot models.ProductSnapshot
	if !BindStrict(ctx, &snapshot) {
		return
	}
	result, err := c.Service.UpsertProduct(ctx, Identity(ctx), snapshot, ctx.Param("codigo_local"))
	respond(ctx, result, err)
}

// UpsertProductBatch godoc
// @Summary Create or update products in an atomic batch of at most 1000
// @Tags Direct integration
// @Accept json
// @Produce json
// @Security AgentAPIKey
// @Param X-Agent-ID header string true "Store credential ID"
// @Param body body models.ProductBatchDTO true "Product batch"
// @Success 200 {object} models.CatalogBatchResult
// @Failure 400,401,403,409 {object} models.AppError
// @Router /integracao/v1/produtos/lotes [post]
func (c *ProductIntegrationController) UpsertProductBatch(ctx *gin.Context) {
	var request models.ProductBatchDTO
	if !BindStrict(ctx, &request) {
		return
	}
	result, err := c.Service.UpsertProducts(ctx, Identity(ctx), request.Items)
	respond(ctx, result, err)
}

// UpsertProductPrice godoc
// @Summary Create or update one store-specific price tier
// @Tags Direct integration
// @Accept json
// @Produce json
// @Security AgentAPIKey
// @Param X-Agent-ID header string true "Store credential ID"
// @Param localCode path string true "Product code in this store"
// @Param kind path string true "Price kind: COST, SALE, or WHOLESALE"
// @Param quantity path number true "Minimum quantity for this tier"
// @Param body body models.ProductPriceSnapshot true "Product price tier"
// @Success 200 {object} models.ProductPriceSnapshot
// @Failure 400,401,403,404,409 {object} models.AppError
// @Router /integracao/v1/produtos/{codigo_local}/precos/{tipo}/{quantidade} [put]
func (c *ProductIntegrationController) UpsertProductPrice(ctx *gin.Context) {
	var snapshot models.ProductPriceSnapshot
	if !BindStrict(ctx, &snapshot) {
		return
	}
	result, err := c.Service.UpsertProductPrice(ctx, Identity(ctx), snapshot, ctx.Param("codigo_local"), ctx.Param("tipo"), ctx.Param("quantidade"))
	respond(ctx, result, err)
}

// UpsertProductPriceBatch godoc
// @Summary Create or update up to 1000 store-specific price tiers
// @Tags Direct integration
// @Accept json
// @Produce json
// @Security AgentAPIKey
// @Param X-Agent-ID header string true "Store credential ID"
// @Param body body models.ProductPriceBatchDTO true "Product price batch"
// @Success 200 {object} models.ProductPriceBatchResult
// @Failure 400,401,403,404,409 {object} models.AppError
// @Router /integracao/v1/precos-produtos/lotes [post]
func (c *ProductIntegrationController) UpsertProductPriceBatch(ctx *gin.Context) {
	var request models.ProductPriceBatchDTO
	if !BindStrict(ctx, &request) {
		return
	}
	result, err := c.Service.UpsertProductPrices(ctx, Identity(ctx), request.Items)
	respond(ctx, result, err)
}

// PublishProduct godoc
// @Summary Publica um produto normalizado e cria mensagens para as lojas do grupo
// @Tags Domain integration
// @Accept json
// @Produce json
// @Security AgentAPIKey
// @Param X-Agent-ID header string true "Store credential ID"
// @Param body body models.ProductPublicationDTO true "Normalized product publication"
// @Success 200 {object} models.ProductPublicationResult
// @Success 201 {object} models.ProductPublicationResult
// @Failure 400,401,403,409 {object} models.AppError
// @Router /integracao/v1/produtos/publicacoes [post]
func (c *ProductIntegrationController) PublishProduct(ctx *gin.Context) {
	var request models.ProductPublicationDTO
	if !BindStrict(ctx, &request) {
		return
	}
	result, err := c.Service.PublishProduct(ctx, Identity(ctx), request)
	if err != nil {
		HandleError(ctx, err)
		return
	}
	ctx.JSON(201, result)
}

// PublishProductBatch godoc
// @Summary Publica ate 1000 produtos normalizados em uma chamada
// @Tags Domain integration
// @Accept json
// @Produce json
// @Security AgentAPIKey
// @Param X-Agent-ID header string true "Store credential ID"
// @Param body body models.ProductPublicationBatchDTO true "Normalized product publications"
// @Success 200 {object} models.ProductPublicationBatchResult
// @Failure 400,401,403,409 {object} models.AppError
// @Router /integracao/v1/produtos/publicacoes/lotes [post]
func (c *ProductIntegrationController) PublishProductBatch(ctx *gin.Context) {
	var request models.ProductPublicationBatchDTO
	if !BindStrict(ctx, &request) {
		return
	}
	result, err := c.Service.PublishProducts(ctx, Identity(ctx), request.Items)
	respond(ctx, result, err)
}

// ProductCatalog godoc
// @Summary Lê a visão do catálogo da loja autenticada para carga inicial
// @Tags Domain integration
// @Produce json
// @Security AgentAPIKey
// @Param X-Agent-ID header string true "Store credential ID"
// @Param page query int false "Page number"
// @Param pageSize query int false "Rows per page, up to 5000"
// @Success 200 {object} models.ProductCatalogPage
// @Failure 400,401,403,409 {object} models.AppError
// @Router /integracao/v1/produtos/catalogo [get]
func (c *ProductIntegrationController) ProductCatalog(ctx *gin.Context) {
	const maxProductCatalogPageSize = 5000
	page, pageSize := 1, 100
	var err error
	if raw := ctx.Query("pagina"); raw == "" {
		raw = ctx.Query("page")
		if raw != "" {
			page, err = strconv.Atoi(raw)
			if err != nil || page < 1 {
				HandleError(ctx, models.Error(400, "pagina deve ser um numero positivo"))
				return
			}
		}
	} else {
		page, err = strconv.Atoi(raw)
		if err != nil || page < 1 {
			HandleError(ctx, models.Error(400, "pagina deve ser um numero positivo"))
			return
		}
	}
	if raw := ctx.Query("limite"); raw == "" {
		raw = ctx.Query("pageSize")
		if raw != "" {
			pageSize, err = strconv.Atoi(raw)
			if err != nil || pageSize < 1 || pageSize > maxProductCatalogPageSize {
				HandleError(ctx, models.Error(400, "limite deve ser de 1 a 5000"))
				return
			}
		}
	} else {
		pageSize, err = strconv.Atoi(raw)
		if err != nil || pageSize < 1 || pageSize > maxProductCatalogPageSize {
			HandleError(ctx, models.Error(400, "limite deve ser de 1 a 5000"))
			return
		}
	}
	result, serviceErr := c.Service.ProductCatalog(ctx, Identity(ctx), page, pageSize)
	respond(ctx, result, serviceErr)
}
