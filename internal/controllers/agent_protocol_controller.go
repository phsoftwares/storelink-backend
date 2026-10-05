package controllers

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/phsoftwares/storelink-backend/internal/models"
	"github.com/phsoftwares/storelink-backend/internal/services"
)

type IAgentProtocolController interface {
	Heartbeat(*gin.Context)
	Send(*gin.Context)
	Pending(*gin.Context)
	IntegrationRenew(*gin.Context)
	Ack(*gin.Context)
	IntegrationAckBatch(*gin.Context)
	Fail(*gin.Context)
	Batch(*gin.Context)
}

type AgentProtocolController struct {
	Agents   services.IAgentService
	Messages services.IMessageService
	Jobs     services.ISyncService
}

var _ IAgentProtocolController = (*AgentProtocolController)(nil)

// Heartbeat godoc
// @Summary Recebe estado recente do agente local
// @Tags Protocolo agente
// @Security AgentAPIKey
// @Param X-Agent-ID header string true "Agent ID"
// @Accept json
// @Produce json
// @Param body body models.HeartbeatDTO true "Estado do agente"
// @Success 200 {object} map[string]interface{}
// @Failure 400,401,500 {object} models.AppError
// @Router /agent/heartbeat [post]
func (c *AgentProtocolController) Heartbeat(ctx *gin.Context) {
	var d models.HeartbeatDTO
	if !Bind(ctx, &d) {
		return
	}
	v, e := c.Agents.Heartbeat(ctx, Identity(ctx), d)
	respond(ctx, v, e)
}

// Send godoc
// @Summary Publica evento ou comando idempotente
// @Description Operação isolada por empresa. Identidade e autorização validadas pelo servidor.
// @Tags Protocolo agente
// @Produce json
// @Security AgentAPIKey
// @Param X-Agent-ID header string true "Agent ID"
// @Accept json
// @Param body body models.MessageDTO true "Dados da operação"
// @Success 200 {object} models.Message
// @Success 201 {object} models.Message
// @Failure 400,401,403,404,409,429,500 {object} models.AppError
// @Router /agent/messages [post]
func (c *AgentProtocolController) Send(ctx *gin.Context) {
	var d models.MessageDTO
	if !Bind(ctx, &d) {
		return
	}
	v, duplicate, e := c.Messages.Send(ctx, Identity(ctx), d)
	if e != nil {
		HandleError(ctx, e)
		return
	}
	code := 201
	if duplicate {
		code = 200
	}
	ctx.JSON(code, v)
}

// Pending godoc
// @Summary Reserva mensagens com claim e lease
// @Description Operação isolada por empresa. Identidade e autorização validadas pelo servidor.
// @Tags Protocolo agente
// @Produce json
// @Security AgentAPIKey
// @Param X-Agent-ID header string true "Agent ID"
// @Param limit query int false "Quantidade de 1 a 1000"
// @Success 200 {object} map[string][]models.Message
// @Failure 400,401,403,404,409,429,500 {object} models.AppError
// @Router /agent/messages/pending [get]
func (c *AgentProtocolController) Pending(ctx *gin.Context) {
	limit := 25
	if raw := ctx.Query("limit"); raw != "" {
		v, e := strconv.Atoi(raw)
		if e != nil || v < 1 || v > 1000 {
			HandleError(ctx, models.Error(400, "Limite deve ser de 1 a 1000"))
			return
		}
		limit = v
	}
	v, e := c.Messages.ClaimOperation(ctx, Identity(ctx), limit, ctx.Query("operation"))
	respond(ctx, gin.H{"items": v}, e)
}

// IntegrationPending godoc
// @Summary Reserva mensagens do contrato de integração
// @Description O FNTS_Server reserva mensagens da fila local do StoreLink com claim e lease.
// @Tags Direct integration
// @Produce json
// @Security AgentAPIKey
// @Param X-Agent-ID header string true "Store credential ID"
// @Param limit query int false "Quantidade de 1 a 1000"
// @Param operation query string false "Prefixo da operação"
// @Success 200 {object} map[string][]models.Message
// @Failure 400,401,403,404,409,429,500 {object} models.AppError
// @Router /integracao/v1/mensagens/pendentes [get]
func (c *AgentProtocolController) IntegrationPending(ctx *gin.Context) {
	limit := 25
	if raw := ctx.Query("limite"); raw == "" {
		raw = ctx.Query("limit")
		if raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil || value < 1 || value > 1000 {
				HandleError(ctx, models.Error(400, "Limite deve ser de 1 a 1000"))
				return
			}
			limit = value
		}
	} else {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 1000 {
			HandleError(ctx, models.Error(400, "Limite deve ser de 1 a 1000"))
			return
		}
		limit = value
	}
	operation := ctx.Query("operacao")
	consolidar := ctx.Query("consolidar") == "1"
	if !consolidar && ctx.Query("consolidar") != "" {
		consolidar, _ = strconv.ParseBool(ctx.Query("consolidar"))
	}
	var v []models.Message
	var e error
	if consolidar {
		v, e = c.Messages.ClaimOperationCoalesced(ctx, Identity(ctx), limit, operation)
	} else {
		v, e = c.Messages.ClaimOperation(ctx, Identity(ctx), limit, operation)
	}
	if e != nil {
		HandleError(ctx, e)
		return
	}
	items := make([]models.IntegrationMessageView, 0, len(v))
	for _, message := range v {
		items = append(items, toIntegrationMessageView(message))
	}
	ctx.JSON(200, models.IntegrationMessagePage{Items: items})
}

// IntegrationRenew godoc
// @Summary Renova as reservas de um lote em processamento
// @Description Mantem a reserva ativa enquanto o FNTS_Server aplica um lote local longo.
// @Tags Direct integration
// @Produce json
// @Security AgentAPIKey
// @Param X-Agent-ID header string true "Store credential ID"
// @Accept json
// @Param body body models.IntegrationRenewDTO true "Mensagens reservadas"
// @Success 200 {object} models.IntegrationRenewResult
// @Failure 400,401,403,500 {object} models.AppError
// @Router /integracao/v1/mensagens/renovar [post]
func (c *AgentProtocolController) IntegrationRenew(ctx *gin.Context) {
	var d models.IntegrationRenewDTO
	if !Bind(ctx, &d) {
		return
	}
	v, e := c.Messages.RenewClaims(ctx, Identity(ctx), d)
	if e != nil {
		HandleError(ctx, e)
		return
	}
	ctx.JSON(200, v)
}

// Ack godoc
// @Summary Confirma somente após commit local
// @Description Operação isolada por empresa. Identidade e autorização validadas pelo servidor.
// @Tags Protocolo agente
// @Produce json
// @Security AgentAPIKey
// @Param X-Agent-ID header string true "Agent ID"
// @Param id path string true "UUID do recurso"
// @Accept json
// @Param body body models.AckDTO true "Dados da operação"
// @Success 200 {object} models.Message
// @Failure 400,401,403,404,409,429,500 {object} models.AppError
// @Router /agent/messages/{id}/ack [post]
func (c *AgentProtocolController) Ack(ctx *gin.Context) {
	id, ok := ID(ctx)
	if !ok {
		return
	}
	var d models.AckDTO
	if !Bind(ctx, &d) {
		return
	}
	v, e := c.Messages.Ack(ctx, Identity(ctx), id, d)
	respond(ctx, v, e)
}

// IntegrationAck godoc
// @Summary Confirma uma mensagem do contrato de integração
// @Description O FNTS_Server confirma somente depois de aplicar a mensagem na base local.
// @Tags Direct integration
// @Produce json
// @Security AgentAPIKey
// @Param X-Agent-ID header string true "Store credential ID"
// @Param id path string true "UUID da mensagem"
// @Accept json
// @Param body body models.AckDTO true "Resultado do processamento"
// @Success 200 {object} models.Message
// @Failure 400,401,403,404,409,429,500 {object} models.AppError
// @Router /integracao/v1/mensagens/{id}/confirmar [post]
func (c *AgentProtocolController) IntegrationAck(ctx *gin.Context) {
	id, ok := ID(ctx)
	if !ok {
		return
	}
	var d models.IntegrationAckDTO
	if !Bind(ctx, &d) {
		return
	}
	v, e := c.Messages.Ack(ctx, Identity(ctx), id, models.AckDTO{
		ClaimToken: d.ClaimToken, ProcessedCount: d.ProcessedCount,
		Errors: integrationItemErrors(d.Errors),
	})
	if e != nil {
		HandleError(ctx, e)
		return
	}
	ctx.JSON(200, toIntegrationMessageView(v))
}

// IntegrationAckBatch godoc
// @Summary Confirma uma pagina de mensagens de integraÃ§Ã£o em uma transaÃ§Ã£o
// @Description Confirma somente claims vÃ¡lidos e ainda dentro do lease.
// @Tags Direct integration
// @Produce json
// @Security AgentAPIKey
// @Param X-Agent-ID header string true "Store credential ID"
// @Accept json
// @Param body body models.IntegrationAckBatchDTO true "Claims da pagina"
// @Success 200 {object} models.IntegrationAckBatchResult
// @Failure 400,401,403,409,500 {object} models.AppError
// @Router /integracao/v1/mensagens/confirmar-lote [post]
func (c *AgentProtocolController) IntegrationAckBatch(ctx *gin.Context) {
	var d models.IntegrationAckBatchDTO
	if !Bind(ctx, &d) {
		return
	}
	v, e := c.Messages.AckBatch(ctx, Identity(ctx), d)
	if e != nil {
		HandleError(ctx, e)
		return
	}
	ctx.JSON(200, v)
}

func integrationItemErrors(errors []models.IntegrationItemError) []models.ItemError {
	result := make([]models.ItemError, 0, len(errors))
	for _, item := range errors {
		result = append(result, models.ItemError{
			ItemKey: item.ItemKey, ErrorCode: item.ErrorCode, Error: item.Error,
		})
	}
	return result
}

// Fail godoc
// @Summary Registra falha com backoff limitado
// @Description Operação isolada por empresa. Identidade e autorização validadas pelo servidor.
// @Tags Protocolo agente
// @Produce json
// @Security AgentAPIKey
// @Param X-Agent-ID header string true "Agent ID"
// @Param id path string true "UUID do recurso"
// @Accept json
// @Param body body models.FailDTO true "Dados da operação"
// @Success 200 {object} models.Message
// @Failure 400,401,403,404,409,429,500 {object} models.AppError
// @Router /agent/messages/{id}/fail [post]
func (c *AgentProtocolController) Fail(ctx *gin.Context) {
	id, ok := ID(ctx)
	if !ok {
		return
	}
	var d models.FailDTO
	if !Bind(ctx, &d) {
		return
	}
	v, e := c.Messages.Fail(ctx, Identity(ctx), id, d)
	respond(ctx, v, e)
}

// IntegrationFail godoc
// @Summary Registra falha de uma mensagem do contrato de integração
// @Description Recoloca a mensagem em retry com backoff ou em dead-letter conforme a política do StoreLink.
// @Tags Direct integration
// @Produce json
// @Security AgentAPIKey
// @Param X-Agent-ID header string true "Store credential ID"
// @Param id path string true "UUID da mensagem"
// @Accept json
// @Param body body models.FailDTO true "Falha do processamento"
// @Success 200 {object} models.Message
// @Failure 400,401,403,404,409,429,500 {object} models.AppError
// @Router /integracao/v1/mensagens/{id}/falhar [post]
func (c *AgentProtocolController) IntegrationFail(ctx *gin.Context) {
	id, ok := ID(ctx)
	if !ok {
		return
	}
	var d models.IntegrationFailDTO
	if !Bind(ctx, &d) {
		return
	}
	v, e := c.Messages.Fail(ctx, Identity(ctx), id, models.FailDTO{
		ClaimToken: d.ClaimToken, Error: d.Error,
	})
	if e != nil {
		HandleError(ctx, e)
		return
	}
	ctx.JSON(200, toIntegrationMessageView(v))
}

func toIntegrationMessageView(message models.Message) models.IntegrationMessageView {
	return models.IntegrationMessageView{
		ID: message.ID, Type: message.Type, Operation: message.Operation,
		OriginStoreID: message.OriginStoreID, DestinationStoreID: message.DestinationStoreID,
		Payload: message.Payload, Status: message.Status, Attempts: message.Attempts,
		ClaimToken: message.ClaimToken, LeaseUntil: message.LeaseUntil,
		NextAttemptAt: message.NextAttemptAt, LastAttemptAt: message.LastAttemptAt,
		LastError: message.LastError, AcknowledgedAt: message.AcknowledgedAt,
		TransferID: message.TransferID,
	}
}

// Batch godoc
// @Summary Recebe lote idempotente da carga
// @Description Operação isolada por empresa. Identidade e autorização validadas pelo servidor.
// @Tags Protocolo agente
// @Produce json
// @Security AgentAPIKey
// @Param X-Agent-ID header string true "Agent ID"
// @Param id path string true "UUID do recurso"
// @Accept json
// @Param body body models.BatchDTO true "Dados da operação"
// @Success 200 {object} models.SyncBatch
// @Success 201 {object} models.SyncBatch
// @Failure 400,401,403,404,409,429,500 {object} models.AppError
// @Router /agent/sync/jobs/{id}/batches [post]
func (c *AgentProtocolController) Batch(ctx *gin.Context) {
	id, ok := ID(ctx)
	if !ok {
		return
	}
	var d models.BatchDTO
	if !Bind(ctx, &d) {
		return
	}
	v, dup, e := c.Jobs.Batch(ctx, Identity(ctx), id, d)
	if e != nil {
		HandleError(ctx, e)
		return
	}
	code := 201
	if dup {
		code = 200
	}
	ctx.JSON(code, v)
}
