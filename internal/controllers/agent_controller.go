package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/phsoftwares/storelink-backend/internal/models"
	"github.com/phsoftwares/storelink-backend/internal/services"
)

type IAgentController interface {
	List(*gin.Context)
	Provision(*gin.Context)
	UpdateAgent(*gin.Context)
	RotateAPIKey(*gin.Context)
}

type AgentController struct{ Service services.IAgentService }

var _ IAgentController = (*AgentController)(nil)

// List godoc
// @Summary Lista agentes da empresa
// @Tags Agentes
// @Produce json
// @Security BearerAuth
// @Param page query int false "Número da página"
// @Param pageSize query int false "Itens por página"
// @Param storeId query string false "UUID da loja"
// @Success 200 {object} models.Page
// @Failure 400,401,403,500 {object} models.AppError
// @Router /agents [get]
func (c *AgentController) List(ctx *gin.Context) {
	filter, ok := Filter(ctx)
	if !ok {
		return
	}
	page, err := c.Service.ListAgents(ctx, Identity(ctx), filter)
	respond(ctx, page, err)
}

// Provision godoc
// @Summary Provisiona agente; credencial exibida uma vez
// @Description Operação isolada por empresa. Identidade e autorização validadas pelo servidor.
// @Tags Agentes
// @Produce json
// @Security BearerAuth
// @Accept json
// @Param body body models.AgentDTO true "Dados da operação"
// @Success 200 {object} models.AgentProvisionResponse
// @Failure 400,401,403,404,409,429,500 {object} models.AppError
// @Router /agents [post]
func (c *AgentController) Provision(ctx *gin.Context) {
	var d models.AgentDTO
	if !Bind(ctx, &d) {
		return
	}
	v, e := c.Service.Provision(ctx, Identity(ctx), d)
	respond(ctx, v, e)
}

// UpdateAgent godoc
// @Summary Ativa ou desativa agente
// @Description Operação isolada por empresa. Identidade e autorização validadas pelo servidor.
// @Tags Agentes
// @Produce json
// @Security BearerAuth
// @Param id path string true "UUID do recurso"
// @Accept json
// @Param body body models.AgentUpdateDTO true "Dados da operação"
// @Success 200 {object} models.Agent
// @Failure 400,401,403,404,409,429,500 {object} models.AppError
// @Router /agents/{id} [put]
func (c *AgentController) UpdateAgent(ctx *gin.Context) {
	id, ok := ID(ctx)
	if !ok {
		return
	}
	var d models.AgentUpdateDTO
	if !Bind(ctx, &d) {
		return
	}
	v, e := c.Service.AgentUpdate(ctx, Identity(ctx), id, d.Ativo)
	respond(ctx, v.Agent, e)
}

// RotateAPIKey godoc
// @Summary Rotaciona a chave fixa do agente
// @Description A nova chave aparece somente nesta resposta; configurar o agente com STORELINK_API_KEY.
// @Tags Agentes
// @Produce json
// @Security BearerAuth
// @Param id path string true "UUID do agente"
// @Success 200 {object} models.AgentProvisionResponse
// @Failure 401,403,404,429,500 {object} models.AppError
// @Router /agents/{id}/rotate-api-key [post]
func (c *AgentController) RotateAPIKey(ctx *gin.Context) {
	id, ok := ID(ctx)
	if !ok {
		return
	}
	v, err := c.Service.RotateAPIKey(ctx, Identity(ctx), id)
	respond(ctx, v, err)
}
