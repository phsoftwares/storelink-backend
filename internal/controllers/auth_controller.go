package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/phsoftwares/storelink-backend/internal/models"
	"github.com/phsoftwares/storelink-backend/internal/services"
)

type IAuthController interface {
	Login(*gin.Context)
	Me(*gin.Context)
}

type AuthController struct{ Service services.IAuthService }

var _ IAuthController = (*AuthController)(nil)

// Login godoc
// @Summary Autentica administrador
// @Description Retorna JWT e empresas associadas. Selecione empresa pelo header idempresa.
// @Tags Autenticação
// @Accept json
// @Produce json
// @Param body body models.LoginDTO true "E-mail e senha"
// @Success 200 {object} models.LoginResponse
// @Failure 400,401,429,500 {object} models.AppError
// @Router /auth/login [post]
func (c *AuthController) Login(ctx *gin.Context) {
	var d models.LoginDTO
	if !Bind(ctx, &d) {
		return
	}
	v, e := c.Service.Login(ctx, d)
	respond(ctx, v, e)
}

// Me godoc
// @Summary Consulta sessão e empresas autorizadas
// @Tags Autenticação
// @Security BearerAuth
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401,403,500 {object} models.AppError
// @Router /auth/me [get]
func (c *AuthController) Me(ctx *gin.Context) {
	i := Identity(ctx)
	user, companies, err := c.Service.Me(ctx, i)
	respond(ctx, gin.H{"user": user, "companies": companies}, err)
}
