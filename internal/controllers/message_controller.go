package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/phsoftwares/storelink-backend/internal/services"
)

type IMessageController interface {
	List(*gin.Context)
	ListErrors(*gin.Context)
	Message(*gin.Context)
	Retry(*gin.Context)
	RetryError(*gin.Context)
}

type MessageController struct{ Service services.IMessageService }

var _ IMessageController = (*MessageController)(nil)

// List godoc
// @Summary Lista mensagens
// @Tags Mensagens
// @Produce json
// @Security BearerAuth
// @Param page query int false "Número da página"
// @Param pageSize query int false "Itens por página"
// @Param q query string false "Busca por operação"
// @Param status query string false "Status"
// @Param operation query string false "Operação"
// @Param storeId query string false "UUID da loja"
// @Param from query string false "Data inicial"
// @Param to query string false "Data final"
// @Success 200 {object} models.Page
// @Failure 400,401,403,500 {object} models.AppError
// @Router /messages [get]
func (c *MessageController) List(ctx *gin.Context) {
	filter, ok := Filter(ctx)
	if !ok {
		return
	}
	page, err := c.Service.ListMessages(ctx, Identity(ctx), filter)
	respond(ctx, page, err)
}

// ListErrors godoc
// @Summary Lista erros de integração
// @Tags Erros
// @Produce json
// @Security BearerAuth
// @Param page query int false "Número da página"
// @Param pageSize query int false "Itens por página"
// @Param q query string false "Busca por operação"
// @Param status query string false "Status"
// @Param operation query string false "Operação"
// @Param storeId query string false "UUID da loja"
// @Param from query string false "Data inicial"
// @Param to query string false "Data final"
// @Success 200 {object} models.Page
// @Failure 400,401,403,500 {object} models.AppError
// @Router /errors [get]
func (c *MessageController) ListErrors(ctx *gin.Context) {
	filter, ok := Filter(ctx)
	if !ok {
		return
	}
	page, err := c.Service.ListErrors(ctx, Identity(ctx), filter)
	respond(ctx, page, err)
}

// Message godoc
// @Summary Consulta mensagem e tentativas
// @Description Operação isolada por empresa. Identidade e autorização validadas pelo servidor.
// @Tags Mensagens
// @Produce json
// @Security BearerAuth
// @Param id path string true "UUID do recurso"
// @Success 200 {object} map[string]interface{}
// @Failure 400,401,403,404,409,429,500 {object} models.AppError
// @Router /messages/{id} [get]
func (c *MessageController) Message(ctx *gin.Context) {
	id, ok := ID(ctx)
	if !ok {
		return
	}
	v, e := c.Service.MessageDetail(ctx, Identity(ctx), id)
	respond(ctx, v, e)
}

// Retry godoc
// @Summary Reprocessa mensagem com falha
// @Description Operação isolada por empresa. Identidade e autorização validadas pelo servidor.
// @Tags Mensagens
// @Produce json
// @Security BearerAuth
// @Param id path string true "UUID do recurso"
// @Success 200 {object} models.Message
// @Failure 400,401,403,404,409,429,500 {object} models.AppError
// @Router /messages/{id}/retry [post]
func (c *MessageController) Retry(ctx *gin.Context) {
	id, ok := ID(ctx)
	if !ok {
		return
	}
	v, e := c.Service.Retry(ctx, Identity(ctx), id)
	respond(ctx, v, e)
}

// RetryError godoc
// @Summary Reprocessa mensagem do erro de integração
// @Description Operação isolada por empresa. Identidade e autorização validadas pelo servidor.
// @Tags Erros
// @Produce json
// @Security BearerAuth
// @Param id path string true "UUID do recurso"
// @Success 200 {object} models.Message
// @Failure 400,401,403,404,409,429,500 {object} models.AppError
// @Router /errors/{id}/retry [post]
func (c *MessageController) RetryError(ctx *gin.Context) {
	id, ok := ID(ctx)
	if !ok {
		return
	}
	v, e := c.Service.RetryError(ctx, Identity(ctx), id)
	respond(ctx, v, e)
}
