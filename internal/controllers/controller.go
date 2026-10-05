package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/google/uuid"
	"github.com/phsoftwares/storelink-backend/internal/models"
	"github.com/phsoftwares/storelink-backend/internal/repositories"
	"github.com/phsoftwares/storelink-backend/internal/services"
)

type Controllers struct {
	Health           *HealthController
	Auth             *AuthController
	Companies        *CompanyController
	Groups           *GroupController
	Stores           *StoreController
	Agents           *AgentController
	Audit            *AuditController
	Monitoring       *MonitoringController
	Messages         *MessageController
	Transfers        *TransferController
	Jobs             *JobController
	AgentProtocol    *AgentProtocolController
	Products         *ProductIntegrationController
	Customers        *CustomerIntegrationController
	Employees        *EmployeeIntegrationController
	CashierOperators *CashierOperatorIntegrationController
	Domain           *DomainIntegrationController
	Exceptions       *ExceptionIntegrationController
}

func New(service *services.Service) *Controllers {
	return &Controllers{
		Health:           &HealthController{},
		Auth:             &AuthController{Service: service},
		Companies:        &CompanyController{Service: service},
		Groups:           &GroupController{Service: service},
		Stores:           &StoreController{Service: service},
		Agents:           &AgentController{Service: service},
		Audit:            &AuditController{Service: service},
		Monitoring:       &MonitoringController{Service: service},
		Messages:         &MessageController{Service: service},
		Transfers:        &TransferController{Service: service, Integration: service},
		Jobs:             &JobController{Service: service},
		AgentProtocol:    &AgentProtocolController{Agents: service, Messages: service, Jobs: service},
		Products:         &ProductIntegrationController{Service: service},
		Customers:        &CustomerIntegrationController{Service: service},
		Employees:        &EmployeeIntegrationController{Service: service},
		CashierOperators: &CashierOperatorIntegrationController{Service: service},
		Domain:           &DomainIntegrationController{Service: service},
		Exceptions:       &ExceptionIntegrationController{Service: service},
	}
}

func Identity(c *gin.Context) models.Identity {
	v, _ := c.Get("identity")
	i, _ := v.(models.Identity)
	return i
}

func HandleError(c *gin.Context, err error) {
	var app *models.AppError
	if errors.As(err, &app) {
		c.JSON(app.Codigo, app)
		return
	}
	if repositories.IsNotFound(err) {
		c.JSON(http.StatusNotFound, models.AppError{Codigo: http.StatusNotFound, MensagemErro: "Registro não encontrado"})
		return
	}
	var sqlState interface{ SQLState() string }
	if errors.As(err, &sqlState) {
		switch sqlState.SQLState() {
		case "23505":
			c.JSON(http.StatusConflict, models.AppError{Codigo: http.StatusConflict, MensagemErro: "Registro já existente"})
			return
		case "23503", "23514", "22001", "22P02":
			c.JSON(http.StatusBadRequest, models.AppError{Codigo: http.StatusBadRequest, MensagemErro: "Vínculo ou dado inválido"})
			return
		}
	}
	slog.Error("request failed", "path", c.FullPath(), "error_type", fmt.Sprintf("%T", err), "error", err)
	c.JSON(http.StatusInternalServerError, models.AppError{Codigo: http.StatusInternalServerError, MensagemErro: "Erro interno do servidor"})
}

func Bind(c *gin.Context, value interface{}) bool {
	if err := c.ShouldBindJSON(value); err != nil {
		c.JSON(http.StatusBadRequest, models.AppError{Codigo: http.StatusBadRequest, MensagemErro: "Dados inválidos; confira os campos obrigatórios"})
		return false
	}
	return true
}

func BindStrict(c *gin.Context, value interface{}) bool {
	const maxIntegrationBody = 16 << 20
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxIntegrationBody)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			c.JSON(http.StatusRequestEntityTooLarge, models.AppError{Codigo: http.StatusRequestEntityTooLarge, MensagemErro: "Corpo da requisição excede 16 MB"})
			return false
		}
		c.JSON(http.StatusBadRequest, models.AppError{Codigo: http.StatusBadRequest, MensagemErro: "JSON inválido ou incompatível com o contrato lowerCamelCase"})
		return false
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		c.JSON(http.StatusBadRequest, models.AppError{Codigo: http.StatusBadRequest, MensagemErro: "A requisição deve conter um único objeto JSON"})
		return false
	}
	if err := binding.Validator.ValidateStruct(value); err != nil {
		c.JSON(http.StatusBadRequest, models.AppError{Codigo: http.StatusBadRequest, MensagemErro: "Dados inválidos; confira os campos obrigatórios"})
		return false
	}
	return true
}

func ID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil || id == uuid.Nil {
		HandleError(c, models.Error(http.StatusBadRequest, "Identificador inválido"))
		return uuid.Nil, false
	}
	return id, true
}

func respond(c *gin.Context, body interface{}, err error) {
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, body)
}

func Filter(c *gin.Context) (repositories.Filter, bool) {
	operation := c.Query("operacao")
	if operation == "" {
		operation = c.Query("operation")
	}
	filter := repositories.Filter{Q: c.Query("q"), Status: c.Query("status"), Operation: operation}
	for _, parameter := range []struct {
		name   string
		target *int
	}{{"pagina", &filter.Page}, {"limite", &filter.PageSize}} {
		if raw := c.Query(parameter.name); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil || value < 1 || value > 1000000 {
				HandleError(c, models.Error(http.StatusBadRequest, "Paginação inválida"))
				return filter, false
			}
			*parameter.target = value
		}
	}
	if filter.Page == 0 {
		if raw := c.Query("page"); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil || value < 1 || value > 1000000 {
				HandleError(c, models.Error(http.StatusBadRequest, "Paginação inválida"))
				return filter, false
			}
			filter.Page = value
		}
	}
	if filter.PageSize == 0 {
		if raw := c.Query("pageSize"); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil || value < 1 || value > 1000000 {
				HandleError(c, models.Error(http.StatusBadRequest, "Paginação inválida"))
				return filter, false
			}
			filter.PageSize = value
		}
	}
	for _, parameter := range []struct {
		name       string
		legacyName string
		target     *uuid.UUID
	}{{"id_grupo", "groupId", &filter.GroupID}, {"id_loja", "storeId", &filter.StoreID}, {"id_loja_origem", "originStoreId", &filter.OriginID}, {"id_loja_destino", "destinationStoreId", &filter.DestinationID}} {
		raw := c.Query(parameter.name)
		if raw == "" {
			raw = c.Query(parameter.legacyName)
		}
		if raw != "" {
			value, err := uuid.Parse(raw)
			if err != nil {
				HandleError(c, models.Error(http.StatusBadRequest, "Filtro de identificador inválido"))
				return filter, false
			}
			*parameter.target = value
		}
	}
	for _, parameter := range []struct {
		name   string
		target **time.Time
	}{{"from", &filter.From}, {"to", &filter.To}} {
		if raw := c.Query(parameter.name); raw != "" {
			value, err := time.Parse(time.RFC3339, raw)
			if err != nil {
				value, err = time.Parse("2006-01-02", raw)
				if err == nil && parameter.name == "to" {
					value = value.Add(24 * time.Hour)
				}
			}
			if err != nil {
				HandleError(c, models.Error(http.StatusBadRequest, "Período inválido"))
				return filter, false
			}
			*parameter.target = &value
		}
	}
	if filter.From != nil && filter.To != nil && !filter.To.After(*filter.From) {
		HandleError(c, models.Error(http.StatusBadRequest, "Período inválido"))
		return filter, false
	}
	return filter, true
}
