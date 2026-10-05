package routes

import (
	"github.com/gin-gonic/gin"
	_ "github.com/phsoftwares/storelink-backend/docs"
	"github.com/phsoftwares/storelink-backend/internal/controllers"
	"github.com/phsoftwares/storelink-backend/internal/middlewares"
	"github.com/phsoftwares/storelink-backend/internal/services"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func New(service *services.Service) *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery(), middlewares.Security(service.Config.AllowedOrigin))
	_ = router.SetTrustedProxies(nil)
	c := controllers.New(service)
	api := router.Group("/api")
	api.GET("/health", c.Health.Health)
	api.POST("/auth/login", middlewares.LoginLimit(), c.Auth.Login)

	users := api.Group("")
	users.Use(middlewares.Auth(service, "user", false))
	users.GET("/auth/me", c.Auth.Me)
	users.GET("/companies", c.Companies.Companies)
	users.POST("/companies", c.Companies.CreateCompany)
	users.PUT("/companies/:id", c.Companies.UpdateCompany)

	admin := api.Group("")
	admin.Use(middlewares.Auth(service, "user", true))
	admin.GET("/dashboard", c.Monitoring.Dashboard)
	admin.GET("/groups", c.Groups.List)
	admin.GET("/stores", c.Stores.List)
	admin.GET("/agents", c.Agents.List)
	admin.GET("/messages", c.Messages.List)
	admin.GET("/errors", c.Messages.ListErrors)
	admin.GET("/audit", c.Audit.List)
	admin.POST("/groups", c.Groups.Group)
	admin.GET("/groups/:id", c.Groups.GroupDetail)
	admin.PUT("/groups/:id", c.Groups.Group)
	admin.POST("/stores", c.Stores.Store)
	admin.PUT("/stores/:id", c.Stores.Store)
	admin.GET("/stores/:id", c.Stores.StoreDetail)
	admin.POST("/agents", c.Agents.Provision)
	admin.PUT("/agents/:id", c.Agents.UpdateAgent)
	admin.POST("/agents/:id/rotate-api-key", c.Agents.RotateAPIKey)
	admin.GET("/messages/:id", c.Messages.Message)
	admin.POST("/messages/:id/retry", c.Messages.Retry)
	admin.POST("/errors/:id/retry", c.Messages.RetryError)
	admin.GET("/transfers/:id", c.Transfers.Transfer)
	admin.GET("/transfers", c.Transfers.List)
	admin.POST("/transfers/:id/retry", c.Transfers.RetryTransfer)
	// Compatibilidade com os consumidores administrativos que usam o
	// vocabulário português do contrato de integração.
	admin.GET("/transferencias/:id", c.Transfers.Transfer)
	admin.GET("/transferencias", c.Transfers.List)
	admin.POST("/transferencias/:id/retry", c.Transfers.RetryTransfer)
	admin.GET("/sync/jobs", c.Jobs.ListJobs)
	admin.POST("/sync/jobs", c.Jobs.CreateJob)
	admin.GET("/sync/jobs/:id", c.Jobs.Job)
	admin.GET("/sync/jobs/:id/batches", c.Jobs.JobList("batches"))
	admin.GET("/sync/jobs/:id/errors", c.Jobs.JobList("job_errors"))
	admin.POST("/sync/jobs/:id/cancel", c.Jobs.CancelJob)
	admin.POST("/sync/jobs/:id/restart", c.Jobs.RestartJob)

	agents := api.Group("/agent")
	agents.Use(middlewares.AgentAuth(service.Repo))
	agents.POST("/heartbeat", c.AgentProtocol.Heartbeat)
	agents.POST("/messages", c.AgentProtocol.Send)
	agents.GET("/messages/pending", c.AgentProtocol.Pending)
	agents.POST("/messages/:id/ack", c.AgentProtocol.Ack)
	agents.POST("/messages/:id/fail", c.AgentProtocol.Fail)
	agents.POST("/sync/jobs/:id/batches", c.AgentProtocol.Batch)

	// The FNTS server calls these resource routes directly; it does not need
	// the separate StoreLink Agent executable.
	integration := api.Group("/integracao/v1")
	integration.Use(middlewares.AgentAuth(service.Repo))
	integration.GET("/produtos/catalogo", c.Products.ProductCatalog)
	integration.POST("/produtos/publicacoes", c.Products.PublishProduct)
	integration.POST("/produtos/publicacoes/lotes", c.Products.PublishProductBatch)
	integration.GET("/mensagens/pendentes", c.AgentProtocol.IntegrationPending)
	integration.POST("/mensagens/renovar", c.AgentProtocol.IntegrationRenew)
	integration.POST("/mensagens/:id/confirmar", c.AgentProtocol.IntegrationAck)
	integration.POST("/mensagens/confirmar-lote", c.AgentProtocol.IntegrationAckBatch)
	integration.POST("/mensagens/:id/falhar", c.AgentProtocol.IntegrationFail)
	integration.PUT("/produtos/:codigo_local", c.Products.UpsertProduct)
	integration.POST("/produtos/lotes", c.Products.UpsertProductBatch)
	integration.PUT("/produtos/:codigo_local/precos/:tipo/:quantidade", c.Products.UpsertProductPrice)
	integration.POST("/precos-produtos/lotes", c.Products.UpsertProductPriceBatch)
	integration.PUT("/clientes/:codigo_local", c.Customers.UpsertCustomer)
	integration.POST("/clientes/lotes", c.Customers.UpsertCustomerBatch)
	integration.PUT("/funcionarios/:codigo_local", c.Employees.UpsertEmployee)
	integration.POST("/funcionarios/lotes", c.Employees.UpsertEmployeeBatch)
	integration.PUT("/operadores-caixa/:codigo_local", c.CashierOperators.UpsertCashierOperator)
	integration.POST("/operadores-caixa/lotes", c.CashierOperators.UpsertCashierOperatorBatch)
	integration.POST("/publicacoes-cadastros", c.Domain.Publish)
	integration.POST("/publicacoes-cadastros/lotes", c.Domain.PublishBatch)
	integration.POST("/excecoes", c.Exceptions.Publish)
	integration.POST("/transferencias", c.Transfers.CreateIntegrationTransfer)
	integration.GET("/transferencias", c.Transfers.ListIntegrationTransfers)
	integration.GET("/transferencias/:id", c.Transfers.IntegrationTransfer)
	integration.PATCH("/transferencias/:id", c.Transfers.UpdateIntegrationTransfer)

	if service.Config.Swagger {
		router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}
	return router
}
