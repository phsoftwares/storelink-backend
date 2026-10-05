package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/phsoftwares/storelink-backend/internal/models"
	"github.com/phsoftwares/storelink-backend/internal/services"
)

type IJobController interface {
	CreateJob(*gin.Context)
	ListJobs(*gin.Context)
	Job(*gin.Context)
	JobList(string) gin.HandlerFunc
	CancelJob(*gin.Context)
	RestartJob(*gin.Context)
}

type JobController struct {
	Service services.ISyncService
}

var _ IJobController = (*JobController)(nil)

// CreateJob godoc
// @Summary Inicia carga e instrui agente origem
// @Description Operação isolada por empresa. Identidade e autorização validadas pelo servidor.
// @Tags Cargas
// @Produce json
// @Security BearerAuth
// @Accept json
// @Param body body models.JobDTO true "Dados da operação"
// @Success 200 {object} models.SyncJob
// @Failure 400,401,403,404,409,429,500 {object} models.AppError
// @Router /sync/jobs [post]
func (c *JobController) CreateJob(ctx *gin.Context) {
	var d models.JobDTO
	if !Bind(ctx, &d) {
		return
	}
	v, e := c.Service.CreateJob(ctx, Identity(ctx), d)
	respond(ctx, v, e)
}

// ListJobs godoc
// @Summary Lista cargas de sincronização
// @Tags Cargas
// @Produce json
// @Security BearerAuth
// @Param page query int false "Número da página"
// @Param pageSize query int false "Itens por página"
// @Param status query string false "Status da carga"
// @Param groupId query string false "UUID do grupo"
// @Param storeId query string false "UUID da loja"
// @Param originStoreId query string false "UUID da loja de origem"
// @Param destinationStoreId query string false "UUID da loja de destino"
// @Param from query string false "Data inicial"
// @Param to query string false "Data final"
// @Success 200 {object} models.Page
// @Failure 400,401,403,500 {object} models.AppError
// @Router /sync/jobs [get]
func (c *JobController) ListJobs(ctx *gin.Context) {
	filter, ok := Filter(ctx)
	if !ok {
		return
	}
	page, err := c.Service.ListJobs(ctx, Identity(ctx), filter)
	respond(ctx, page, err)
}

// Job godoc
// @Summary Consulta progresso da carga
// @Description Operação isolada por empresa. Identidade e autorização validadas pelo servidor.
// @Tags Cargas
// @Produce json
// @Security BearerAuth
// @Param id path string true "UUID do recurso"
// @Success 200 {object} models.SyncJob
// @Failure 400,401,403,404,409,429,500 {object} models.AppError
// @Router /sync/jobs/{id} [get]
func (c *JobController) Job(ctx *gin.Context) {
	id, ok := ID(ctx)
	if !ok {
		return
	}
	v, e := c.Service.Job(ctx, Identity(ctx), id)
	respond(ctx, v, e)
}

// JobList godoc
// @Summary Lista lotes ou erros da carga
// @Description Operação isolada por empresa. Identidade e autorização validadas pelo servidor.
// @Tags Cargas
// @Produce json
// @Security BearerAuth
// @Param id path string true "UUID do recurso"
// @Param page query int false "Paginação"
// @Param pageSize query int false "Paginação"
// @Param q query string false "Filtro opcional"
// @Param status query string false "Filtro opcional"
// @Param groupId query string false "Filtro opcional"
// @Param storeId query string false "Filtro opcional"
// @Param originStoreId query string false "Filtro opcional"
// @Param destinationStoreId query string false "Filtro opcional"
// @Param operation query string false "Filtro opcional"
// @Param from query string false "Filtro opcional"
// @Param to query string false "Filtro opcional"
// @Success 200 {object} models.Page
// @Failure 400,401,403,404,409,429,500 {object} models.AppError
// @Router /sync/jobs/{id}/batches [get]
// @Router /sync/jobs/{id}/errors [get]
func (c *JobController) JobList(kind string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		id, ok := ID(ctx)
		if !ok {
			return
		}
		i := Identity(ctx)
		if _, e := c.Service.Job(ctx, i, id); e != nil {
			HandleError(ctx, e)
			return
		}
		f, ok := Filter(ctx)
		if !ok {
			return
		}
		f.Where = map[string]interface{}{"id_sync_job": id}
		var page models.Page
		var err error
		switch kind {
		case "batches":
			page, err = c.Service.ListJobBatches(ctx, i, id, f)
		case "job_errors":
			page, err = c.Service.ListJobErrors(ctx, i, id, f)
		default:
			err = models.Error(404, "Recurso da carga não encontrado")
		}
		respond(ctx, page, err)
	}
}

// CancelJob godoc
// @Summary Cancela carga sem lote em processamento
// @Description Operação isolada por empresa. Identidade e autorização validadas pelo servidor.
// @Tags Cargas
// @Produce json
// @Security BearerAuth
// @Param id path string true "UUID do recurso"
// @Success 200 {object} models.SyncJob
// @Failure 400,401,403,404,409,429,500 {object} models.AppError
// @Router /sync/jobs/{id}/cancel [post]
func (c *JobController) CancelJob(ctx *gin.Context) {
	id, ok := ID(ctx)
	if !ok {
		return
	}
	v, e := c.Service.CancelJob(ctx, Identity(ctx), id)
	respond(ctx, v, e)
}

// RestartJob godoc
// @Summary Refaz carga finalizada em novo job
// @Description Operação isolada por empresa. Identidade e autorização validadas pelo servidor.
// @Tags Cargas
// @Produce json
// @Security BearerAuth
// @Param id path string true "UUID do recurso"
// @Success 200 {object} models.SyncJob
// @Failure 400,401,403,404,409,429,500 {object} models.AppError
// @Router /sync/jobs/{id}/restart [post]
func (c *JobController) RestartJob(ctx *gin.Context) {
	id, ok := ID(ctx)
	if !ok {
		return
	}
	v, e := c.Service.RestartJob(ctx, Identity(ctx), id)
	respond(ctx, v, e)
}
