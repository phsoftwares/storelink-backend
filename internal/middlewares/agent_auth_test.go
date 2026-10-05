package middlewares

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/phsoftwares/storelink-backend/internal/auth"
	"github.com/phsoftwares/storelink-backend/internal/models"
	"github.com/phsoftwares/storelink-backend/internal/repositories"
)

type agentIdentityFixture struct {
	agents map[uuid.UUID]models.Agent
	stores map[uuid.UUID]models.Store
}

var _ repositories.IAgentIdentityRepository = agentIdentityFixture{}

func normalizeFixtureCNPJ(value string) string {
	var result strings.Builder
	for _, r := range strings.ToUpper(value) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			result.WriteRune(r)
		}
	}
	return result.String()
}

func (f agentIdentityFixture) AgentForAPIKey(_ context.Context, id uuid.UUID, cnpj string) (models.Agent, models.Store, error) {
	agent, ok := f.agents[id]
	if !ok {
		return models.Agent{}, models.Store{}, models.Error(http.StatusNotFound, "not found")
	}
	if agent.StoreID != nil {
		store := f.stores[*agent.StoreID]
		if cnpj != "" && store.CNPJ != cnpj {
			return models.Agent{}, models.Store{}, models.Error(http.StatusUnauthorized, "cnpj mismatch")
		}
		return agent, store, nil
	}
	cnpj = normalizeFixtureCNPJ(cnpj)
	for _, store := range f.stores {
		if normalizeFixtureCNPJ(store.CNPJ) == cnpj {
			return agent, store, nil
		}
	}
	return models.Agent{}, models.Store{}, models.Error(http.StatusUnauthorized, "cnpj not found")
}

func TestAgentAuthUsesPerAgentKeyAndIgnoresBearer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	companyID, otherCompanyID := uuid.New(), uuid.New()
	agentAID, agentBID := uuid.New(), uuid.New()
	storeAID, storeBID := uuid.New(), uuid.New()
	keyA, keyB := "agent-a-fixed-api-key", "agent-b-fixed-api-key"
	agentA := models.Agent{Tenant: models.NewTenant(companyID), StoreID: &storeAID, Nome: "Agent A", APIKeyHash: auth.HashAPIKey(keyA), Ativo: true}
	agentA.ID = agentAID
	agentB := models.Agent{Tenant: models.NewTenant(otherCompanyID), StoreID: &storeBID, Nome: "Agent B", APIKeyHash: auth.HashAPIKey(keyB), Ativo: true}
	agentB.ID = agentBID
	storeA := models.Store{Tenant: models.NewTenant(companyID), GroupID: uuid.New()}
	storeA.ID = storeAID
	storeA.CNPJ = "11111111111111"
	storeB := models.Store{Tenant: models.NewTenant(otherCompanyID), GroupID: uuid.New()}
	storeB.ID = storeBID
	storeB.CNPJ = "22222222222222"
	fixture := agentIdentityFixture{
		agents: map[uuid.UUID]models.Agent{
			agentAID: agentA,
			agentBID: agentB,
		},
		stores: map[uuid.UUID]models.Store{
			storeAID: storeA,
			storeBID: storeB,
		},
	}
	router := gin.New()
	router.Use(AgentAuth(fixture))
	router.GET("/agent", func(c *gin.Context) {
		identity, _ := c.Get("identity")
		c.JSON(http.StatusOK, identity)
	})

	request := func(id, key, bearer string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/agent", nil)
		req.Header.Set("X-Agent-ID", id)
		if key != "" {
			req.Header.Set("X-API-Key", key)
		}
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		return response
	}

	valid := request(agentAID.String(), keyA, "")
	if valid.Code != http.StatusOK {
		t.Fatalf("valid per-agent key returned HTTP %d: %s", valid.Code, valid.Body.String())
	}
	var identity models.Identity
	if err := json.NewDecoder(valid.Body).Decode(&identity); err != nil {
		t.Fatal(err)
	}
	if identity.AgentID != agentAID || identity.CompanyID != companyID || identity.StoreID != storeAID {
		t.Fatalf("identity was not resolved from the agent record: %#v", identity)
	}
	if wrongAgent := request(agentBID.String(), keyA, ""); wrongAgent.Code != http.StatusUnauthorized {
		t.Fatalf("an agent key must not authenticate as another agent, got HTTP %d", wrongAgent.Code)
	}
	if bearerOnly := request(agentAID.String(), "", keyA); bearerOnly.Code != http.StatusUnauthorized {
		t.Fatalf("Bearer credentials must not authorize agent endpoints, got HTTP %d", bearerOnly.Code)
	}
	if invalidID := request("not-a-uuid", keyA, ""); invalidID.Code != http.StatusUnauthorized {
		t.Fatalf("invalid agent ID must be rejected, got HTTP %d", invalidID.Code)
	}
}

func TestAgentAuthResolvesSharedAgentByCNPJ(t *testing.T) {
	gin.SetMode(gin.TestMode)
	companyID := uuid.New()
	sharedID := uuid.New()
	key := "shared-agent-fixed-api-key"
	shared := models.Agent{Tenant: models.NewTenant(companyID), Nome: "Agent compartilhado", APIKeyHash: auth.HashAPIKey(key), Ativo: true}
	shared.ID = sharedID
	storeID := uuid.New()
	store := models.Store{Tenant: models.NewTenant(companyID), GroupID: uuid.New(), CNPJ: "12.345.678/0001-90"}
	store.ID = storeID
	fixture := agentIdentityFixture{
		agents: map[uuid.UUID]models.Agent{sharedID: shared},
		stores: map[uuid.UUID]models.Store{storeID: store},
	}
	router := gin.New()
	router.Use(AgentAuth(fixture))
	router.GET("/agent", func(c *gin.Context) {
		identity, _ := c.Get("identity")
		c.JSON(http.StatusOK, identity)
	})

	request := func(cnpj string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/agent", nil)
		req.Header.Set("X-Agent-ID", sharedID.String())
		req.Header.Set("X-API-Key", key)
		req.Header.Set("X-CNPJ", cnpj)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		return response
	}

	valid := request("12345678000190")
	if valid.Code != http.StatusOK {
		t.Fatalf("shared credential with the store CNPJ returned HTTP %d: %s", valid.Code, valid.Body.String())
	}
	var identity models.Identity
	if err := json.NewDecoder(valid.Body).Decode(&identity); err != nil {
		t.Fatal(err)
	}
	if identity.AgentID != sharedID || identity.StoreID != storeID || identity.CompanyID != companyID {
		t.Fatalf("shared credential did not resolve the CNPJ store: %#v", identity)
	}
	if missing := request(""); missing.Code != http.StatusUnauthorized {
		t.Fatalf("shared credential without CNPJ must be rejected, got HTTP %d", missing.Code)
	}
}
