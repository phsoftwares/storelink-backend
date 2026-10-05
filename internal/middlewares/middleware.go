package middlewares

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/phsoftwares/storelink-backend/internal/auth"
	"github.com/phsoftwares/storelink-backend/internal/controllers"
	"github.com/phsoftwares/storelink-backend/internal/models"
	"github.com/phsoftwares/storelink-backend/internal/repositories"
	"github.com/phsoftwares/storelink-backend/internal/services"
	"net/http"
	"strings"
	"sync"
	"time"
)

func Auth(s *services.Service, kind string, tenantRequired bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		parts := strings.Fields(c.GetHeader("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			controllers.HandleError(c, models.Error(401, "Token obrigatório"))
			c.Abort()
			return
		}
		i, e := s.Resolve(c, parts[1], c.GetHeader("idempresa"))
		if e != nil {
			controllers.HandleError(c, e)
			c.Abort()
			return
		}
		if (kind == "agent") != i.IsAgent() {
			controllers.HandleError(c, models.Error(403, "Tipo de sessão não autorizado"))
			c.Abort()
			return
		}
		if tenantRequired && i.CompanyID == uuid.Nil {
			controllers.HandleError(c, models.Error(403, "Selecione ou cadastre uma empresa"))
			c.Abort()
			return
		}
		c.Set("identity", i)
		c.Next()
	}
}

func AgentAuth(repository repositories.IAgentIdentityRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		agentID, err := uuid.Parse(c.GetHeader("X-Agent-ID"))
		if err != nil || agentID == uuid.Nil {
			controllers.HandleError(c, models.Error(http.StatusUnauthorized, "Invalid agent ID"))
			c.Abort()
			return
		}
		cnpj := strings.TrimSpace(c.GetHeader("X-CNPJ"))
		if cnpj == "" {
			cnpj = strings.TrimSpace(c.GetHeader("X-Store-CNPJ"))
		}
		if cnpj == "" {
			cnpj = strings.TrimSpace(c.GetHeader("CNPJ"))
		}
		agent, store, err := repository.AgentForAPIKey(c.Request.Context(), agentID, cnpj)
		if err != nil {
			controllers.HandleError(c, models.Error(http.StatusUnauthorized, "Agent inactive or not found"))
			c.Abort()
			return
		}
		if !auth.CompareAPIKey(agent.APIKeyHash, c.GetHeader("X-API-Key")) {
			controllers.HandleError(c, models.Error(http.StatusUnauthorized, "Invalid API key"))
			c.Abort()
			return
		}
		c.Set("identity", models.Identity{CompanyID: agent.CompanyID, AgentID: agent.ID, StoreID: store.ID, GroupID: store.GroupID, Nome: agent.Nome})
		c.Next()
	}
}
func Security(origin string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "no-referrer")
		if c.GetHeader("Origin") == origin {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, idempresa, X-API-Key, X-Agent-ID, X-CNPJ, X-Store-CNPJ")
			c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,OPTIONS")
		}
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<20)
		c.Next()
	}
}

type bucket struct {
	start time.Time
	count int
}

func LoginLimit() gin.HandlerFunc {
	var mu sync.Mutex
	entries := map[string]bucket{}
	return func(c *gin.Context) {
		now := time.Now()
		key := c.ClientIP()
		mu.Lock()
		if len(entries) > 10000 {
			for k, v := range entries {
				if now.Sub(v.start) > time.Minute {
					delete(entries, k)
				}
			}
		}
		b := entries[key]
		if now.Sub(b.start) > time.Minute {
			b = bucket{start: now}
		}
		b.count++
		entries[key] = b
		mu.Unlock()
		if b.count > 60 {
			c.Header("Retry-After", "60")
			c.AbortWithStatusJSON(429, models.AppError{Codigo: 429, MensagemErro: "Muitas tentativas; tente em um minuto"})
			return
		}
		c.Next()
	}
}
