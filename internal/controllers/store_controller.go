package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/phsoftwares/storelink-backend/internal/models"
	"github.com/phsoftwares/storelink-backend/internal/services"
)

type IStoreController interface {
	List(*gin.Context)
	Store(*gin.Context)
	StoreDetail(*gin.Context)
}

type StoreController struct{ Service services.IStoreService }

var _ IStoreController = (*StoreController)(nil)

// List godoc
// @Summary Lista lojas com estado recente do agente
// @Tags Lojas
// @Produce json
// @Security BearerAuth
// @Param page query int false "Número da página"
// @Param pageSize query int false "Itens por página"
// @Param q query string false "Busca por nome ou código"
// @Param status query string false "Estado calculado"
// @Param groupId query string false "UUID do grupo"
// @Param storeId query string false "UUID da loja"
// @Success 200 {object} models.Page
// @Failure 400,401,403,500 {object} models.AppError
// @Router /stores [get]
func (c *StoreController) List(ctx *gin.Context) {
	filter, ok := Filter(ctx)
	if !ok {
		return
	}
	page, err := c.Service.ListStores(ctx, Identity(ctx), filter)
	respond(ctx, page, err)
}

// Store godoc
// @Summary Cria loja
// @Description Operação isolada por empresa. Identidade e autorização validadas pelo servidor.
// @Tags Lojas
// @Produce json
// @Security BearerAuth
// @Accept json
// @Param body body models.StoreDTO true "Dados da operação"
// @Success 200 {object} models.Store
// @Failure 400,401,403,404,409,429,500 {object} models.AppError
// @Router /stores [post]
func (c *StoreController) Store(ctx *gin.Context) {
	var id uuid.UUID
	if ctx.Param("id") != "" {
		var ok bool
		id, ok = ID(ctx)
		if !ok {
			return
		}
	}
	var d models.StoreDTO
	if !Bind(ctx, &d) {
		return
	}
	v, e := c.Service.Store(ctx, Identity(ctx), id, d)
	respond(ctx, v, e)
}

// StoreDetail godoc
// @Summary Consulta detalhes e métricas da loja
// @Description Operação isolada por empresa. Identidade e autorização validadas pelo servidor.
// @Tags Lojas
// @Produce json
// @Security BearerAuth
// @Param id path string true "UUID do recurso"
// @Success 200 {object} models.StoreView
// @Failure 400,401,403,404,409,429,500 {object} models.AppError
// @Router /stores/{id} [get]
func (c *StoreController) StoreDetail(ctx *gin.Context) {
	id, ok := ID(ctx)
	if !ok {
		return
	}
	store, err := c.Service.StoreDetail(ctx, Identity(ctx), id)
	respond(ctx, store, err)
}
