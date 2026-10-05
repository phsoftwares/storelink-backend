package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/phsoftwares/storelink-backend/internal/models"
	"github.com/phsoftwares/storelink-backend/internal/services"
)

type IGroupController interface {
	List(*gin.Context)
	Group(*gin.Context)
	GroupDetail(*gin.Context)
}

type GroupController struct{ Service services.IGroupService }

var _ IGroupController = (*GroupController)(nil)

// List godoc
// @Summary Lista grupos da empresa
// @Tags Grupos
// @Produce json
// @Security BearerAuth
// @Param page query int false "Número da página"
// @Param pageSize query int false "Itens por página"
// @Param q query string false "Busca por nome"
// @Param status query string false "Status"
// @Success 200 {object} models.Page
// @Failure 400,401,403,500 {object} models.AppError
// @Router /groups [get]
func (c *GroupController) List(ctx *gin.Context) {
	filter, ok := Filter(ctx)
	if !ok {
		return
	}
	page, err := c.Service.ListGroups(ctx, Identity(ctx), filter)
	respond(ctx, page, err)
}

// Group godoc
// @Summary Cria grupo de lojas
// @Description Operação isolada por empresa. Identidade e autorização validadas pelo servidor.
// @Tags Grupos
// @Produce json
// @Security BearerAuth
// @Accept json
// @Param body body models.GroupDTO true "Dados da operação"
// @Success 200 {object} models.Group
// @Failure 400,401,403,404,409,429,500 {object} models.AppError
// @Router /groups [post]
func (c *GroupController) Group(ctx *gin.Context) {
	var id uuid.UUID
	if ctx.Param("id") != "" {
		var ok bool
		id, ok = ID(ctx)
		if !ok {
			return
		}
	}
	var d models.GroupDTO
	if !Bind(ctx, &d) {
		return
	}
	v, e := c.Service.Group(ctx, Identity(ctx), id, d)
	respond(ctx, v, e)
}

// GroupDetail godoc
// @Summary Consulta configuração de integração do grupo
// @Tags Grupos
// @Security BearerAuth
// @Produce json
// @Param id path string true "UUID do grupo"
// @Success 200 {object} models.Group
// @Failure 400,401,403,404,500 {object} models.AppError
// @Router /groups/{id} [get]
func (c *GroupController) GroupDetail(ctx *gin.Context) {
	id, ok := ID(ctx)
	if !ok {
		return
	}
	v, e := c.Service.GroupDetail(ctx, Identity(ctx), id)
	respond(ctx, v, e)
}
