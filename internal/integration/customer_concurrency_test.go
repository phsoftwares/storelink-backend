package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/phsoftwares/storelink-backend/internal/models"
)

func TestConcurrentCustomerPublicationsFromDifferentStores(t *testing.T) {
	h := newHarness(t)
	var user models.User
	if err := h.db.First(&user, "id = ?", h.user).Error; err != nil {
		t.Fatal(err)
	}
	_, login := h.request("POST", "/api/auth/login", models.LoginDTO{
		Email: user.Email, Senha: "password-de-teste-segura",
	}, "", "")
	admin := login["token"].(string)
	company := bodyID(h, "POST", "/api/companies", models.CompanyDTO{
		Nome: "Empresa para corrida entre filiais",
		CNPJ: "TEST-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:10],
	}, admin, "")
	group := bodyID(h, "POST", "/api/groups", models.GroupDTO{
		Nome: "Grupo para corrida entre filiais",
	}, admin, company)
	storeA := bodyID(h, "POST", "/api/stores", models.StoreDTO{
		GroupID: mustUUID(t, group), Codigo: "601", Nome: "Filial A", Tipo: "FILIAL",
	}, admin, company)
	storeB := bodyID(h, "POST", "/api/stores", models.StoreDTO{
		GroupID: mustUUID(t, group), Codigo: "602", Nome: "Filial B", Tipo: "FILIAL",
	}, admin, company)
	agentA, keyA := createAgent(h, storeA, admin, company)
	agentB, keyB := createAgent(h, storeB, admin, company)

	type publication struct {
		localCode string
		agentID   string
		apiKey    string
		snapshot  models.CustomerSnapshot
	}
	type response struct {
		status int
		body   map[string]interface{}
		err    error
	}
	concurrentlyPublish := func(a, b publication) (string, string) {
		t.Helper()
		start := make(chan struct{})
		results := make(chan response, 2)
		for _, item := range []publication{a, b} {
			item := item
			payload, err := json.Marshal(item.snapshot)
			if err != nil {
				t.Fatal(err)
			}
			go func() {
				<-start
				req, err := http.NewRequestWithContext(context.Background(),
					http.MethodPut, h.server.URL+
						"/api/integracao/v1/clientes/"+item.localCode,
					bytes.NewReader(payload))
				if err != nil {
					results <- response{err: err}
					return
				}
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-API-Key", item.apiKey)
				req.Header.Set("X-Agent-ID", item.agentID)
				resp, err := h.client.Do(req)
				if err != nil {
					results <- response{err: err}
					return
				}
				defer resp.Body.Close()
				data, err := io.ReadAll(resp.Body)
				if err != nil {
					results <- response{status: resp.StatusCode, err: err}
					return
				}
				var body map[string]interface{}
				if err = json.Unmarshal(data, &body); err != nil {
					results <- response{status: resp.StatusCode,
						err: fmt.Errorf("resposta %q: %w", string(data), err)}
					return
				}
				results <- response{status: resp.StatusCode, body: body}
			}()
		}
		close(start)
		first, second := <-results, <-results
		for _, result := range []response{first, second} {
			if result.err != nil {
				t.Fatalf("publicacao concorrente falhou: %v", result.err)
			}
			if result.status != http.StatusOK {
				t.Fatalf("publicacao concorrente HTTP %d: %#v", result.status, result.body)
			}
		}
		return first.body["id"].(string), second.body["id"].(string)
	}
	makeSnapshot := func(code, document string) models.CustomerSnapshot {
		return models.CustomerSnapshot{
			LocalCode: code, Name: "Cliente cadastrado simultaneamente",
			TaxIdentifier: document, Phone: "1133334444", Mobile: "11999998888",
			Email: "cliente-filiais@example.test", CreditStatus: "STANDARD",
			Address: models.Address{Street: "Rua de Teste", Number: "10",
				District: "Centro", City: "Sao Paulo", State: "SP", PostalCode: "01001000"},
		}
	}

	// Duas filiais criam o mesmo cliente ao mesmo tempo, cada uma com seu
	// codigo local e formato de CPF. Ambas devem receber a mesma identidade.
	idA, idB := concurrentlyPublish(
		publication{localCode: "CLI-FILIAL-A-001", agentID: agentA, apiKey: keyA,
			snapshot: makeSnapshot("CLI-FILIAL-A-001", "529.982.247-25")},
		publication{localCode: "CLI-FILIAL-B-001", agentID: agentB, apiKey: keyB,
			snapshot: makeSnapshot("CLI-FILIAL-B-001", "52998224725")},
	)
	if idA != idB {
		t.Fatalf("filiais receberam identidades canonicas diferentes: %s / %s", idA, idB)
	}
	companyID, canonicalID := mustUUID(t, company), mustUUID(t, idA)
	var canonicalCount int64
	if err := h.db.Model(&models.Customer{}).
		Where("id_empresa = ? AND documento = ?", companyID, "52998224725").
		Count(&canonicalCount).Error; err != nil {
		t.Fatal(err)
	}
	if canonicalCount != 1 {
		t.Fatalf("esperava um cliente canonico para o CPF, encontrei %d", canonicalCount)
	}
	for _, expected := range []struct {
		store string
		code  string
	}{{storeA, "CLI-FILIAL-A-001"}, {storeB, "CLI-FILIAL-B-001"}} {
		var mapping models.CustomerStoreMapping
		if err := h.db.Where("id_empresa = ? AND id_loja = ? AND codigo_local = ?",
			companyID, mustUUID(t, expected.store), expected.code).
			First(&mapping).Error; err != nil {
			t.Fatalf("mapeamento local %s nao foi persistido: %v", expected.code, err)
		}
		if mapping.CustomerID != canonicalID {
			t.Fatalf("codigo local %s aponta para %s, esperado %s",
				expected.code, mapping.CustomerID, canonicalID)
		}
	}

	// Pessoas distintas, mesmo cadastradas em filiais diferentes, nao podem
	// ser fundidas apenas por coincidencia de nome/contato.
	distinctA, distinctB := concurrentlyPublish(
		publication{localCode: "CLI-FILIAL-A-002", agentID: agentA, apiKey: keyA,
			snapshot: makeSnapshot("CLI-FILIAL-A-002", "11144477735")},
		publication{localCode: "CLI-FILIAL-B-002", agentID: agentB, apiKey: keyB,
			snapshot: makeSnapshot("CLI-FILIAL-B-002", "12345678909")},
	)
	if distinctA == distinctB {
		t.Fatalf("clientes com documentos diferentes foram fundidos: %s", distinctA)
	}
	t.Logf("PASS: duas filiais simultaneas; mesmo CPF = 1 cliente canonico/2 mapeamentos; documentos distintos = 2 identidades")
}
