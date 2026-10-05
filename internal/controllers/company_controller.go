package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/phsoftwares/storelink-backend/internal/models"
	"github.com/phsoftwares/storelink-backend/internal/services"
)

type ICompanyController interface {
	Companies(*gin.Context)
	CreateCompany(*gin.Context)
	UpdateCompany(*gin.Context)
}

type CompanyController struct{ Service services.ICompanyService }

var _ ICompanyController = (*CompanyController)(nil)

// Companies godoc
// @Summary Lista empresas autorizadas
// @Tags Empresas
// @Security BearerAuth
// @Produce json
// @Success 200 {object} models.Page
// @Failure 401,403,500 {object} models.AppError
// @Router /companies [get]
func (c *CompanyController) Companies(ctx *gin.Context) {
	companies, err := c.Service.Companies(ctx, Identity(ctx).UserID)
	respond(ctx, models.Page{Items: companies, Total: int64(len(companies)), Page: 1, PageSize: len(companies)}, err)
}

// CreateCompany godoc
// @Summary Cria empresa e associa administrador
// @Tags Empresas
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body models.CompanyDTO true "Empresa"
// @Success 200 {object} models.Company
// @Failure 400,401,403,409,500 {object} models.AppError
// @Router /companies [post]
func (c *CompanyController) CreateCompany(ctx *gin.Context) {
	var d models.CompanyDTO
	if !Bind(ctx, &d) {
		return
	}
	v, e := c.Service.Company(ctx, Identity(ctx), uuid.Nil, d)
	respond(ctx, v, e)
}

// UpdateCompany godoc
// @Summary Atualiza empresa autorizada
// @Tags Empresas
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "UUID da empresa"
// @Param body body models.CompanyDTO true "Empresa"
// @Success 200 {object} models.Company
// @Failure 400,401,403,404,409,500 {object} models.AppError
// @Router /companies/{id} [put]
func (c *CompanyController) UpdateCompany(ctx *gin.Context) {
	id, ok := ID(ctx)
	if !ok {
		return
	}
	var d models.CompanyDTO
	if !Bind(ctx, &d) {
		return
	}
	v, e := c.Service.Company(ctx, Identity(ctx), id, d)
	respond(ctx, v, e)
}
