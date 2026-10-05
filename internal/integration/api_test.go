package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/phsoftwares/storelink-backend/internal/auth"
	"github.com/phsoftwares/storelink-backend/internal/config"
	database "github.com/phsoftwares/storelink-backend/internal/config/database/migrations"
	"github.com/phsoftwares/storelink-backend/internal/models"
	"github.com/phsoftwares/storelink-backend/internal/repositories"
	"github.com/phsoftwares/storelink-backend/internal/routes"
	"github.com/phsoftwares/storelink-backend/internal/services"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type harness struct {
	server    *httptest.Server
	db        *gorm.DB
	user      uuid.UUID
	client    *http.Client
	t         *testing.T
	agentKeys map[string]string
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	dsn := os.Getenv("STORELINK_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("defina STORELINK_TEST_DATABASE_URL com um PostgreSQL de teste isolado")
	}
	db, e := database.InitConnection(dsn)
	if e != nil {
		t.Fatal(e)
	}
	sqlDB, e := db.DB()
	if e != nil {
		t.Fatal(e)
	}
	if e = sqlDB.PingContext(context.Background()); e != nil {
		t.Fatal(e)
	}
	user := models.User{Base: models.NewBase(), Nome: "Teste de integraÃ§Ã£o", Email: "integration-" + uuid.NewString() + "@example.test", Ativo: true}
	hash, e := bcrypt.GenerateFromPassword([]byte("password-de-teste-segura"), bcrypt.MinCost)
	if e != nil {
		t.Fatal(e)
	}
	user.Senha = string(hash)
	if e = db.Create(&user).Error; e != nil {
		t.Fatal(e)
	}
	cfg := config.Config{Port: "8080", AllowedOrigin: "http://127.0.0.1:5173", JWTSecret: "integration-only-secret-with-over-32-characters", Lease: time.Second, OfflineAfter: time.Minute, RequestTimeout: 5 * time.Second, MaxAttempts: 2, DefaultBatchSize: 500, RetryDelays: []time.Duration{250 * time.Millisecond}}
	server := httptest.NewServer(routes.New(services.New(repositories.New(db), auth.New(cfg.JWTSecret), cfg)))
	h := &harness{server: server, db: db, user: user.ID, client: server.Client(), t: t, agentKeys: make(map[string]string)}
	t.Cleanup(func() {
		server.Close()
		sql, _ := db.DB()
		_, _ = sql.ExecContext(context.Background(), "DELETE FROM empresa WHERE id IN (SELECT id_empresa FROM usuario_empresa WHERE id_usuario = $1)", user.ID)
		_, _ = sql.ExecContext(context.Background(), "DELETE FROM usuario WHERE id = $1", user.ID)
		_ = sql.Close()
	})
	return h
}
func (h *harness) request(method, path string, body interface{}, token, company string) (int, map[string]interface{}) {
	h.t.Helper()
	direct := strings.HasPrefix(path, "/api/integracao/v1/")
	var b bytes.Buffer
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			h.t.Fatal(err)
		}
		var payload interface{}
		if err := json.Unmarshal(encoded, &payload); err != nil {
			h.t.Fatal(err)
		}
		if !direct {
			payload = camelizeJSON(payload)
		}
		encoded, err = json.Marshal(payload)
		if err != nil {
			h.t.Fatal(err)
		}
		b.Write(encoded)
	}
	if !direct {
		path = camelizeQuery(path)
	}
	req, e := http.NewRequest(method, h.server.URL+path, &b)
	if e != nil {
		h.t.Fatal(e)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if strings.HasPrefix(path, "/api/agent/") || strings.HasPrefix(path, "/api/integracao/v1/") {
		req.Header.Set("X-API-Key", h.agentKeys[token])
		req.Header.Set("X-Agent-ID", token)
	} else if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if company != "" {
		req.Header.Set("idempresa", company)
	}
	response, e := h.client.Do(req)
	if e != nil {
		h.t.Fatal(e)
	}
	defer response.Body.Close()
	var result map[string]interface{}
	if response.StatusCode != http.StatusNoContent {
		if e = json.NewDecoder(response.Body).Decode(&result); e != nil {
			h.t.Fatalf("%s %s status=%d: resposta JSON invÃ¡lida: %v", method, path, response.StatusCode, e)
		}
	}
	return response.StatusCode, result
}
func mustUUID(t *testing.T, value interface{}) uuid.UUID {
	t.Helper()
	s, ok := value.(string)
	if !ok {
		t.Fatalf("esperava UUID string, recebeu %#v", value)
	}
	v, e := uuid.Parse(s)
	if e != nil {
		t.Fatal(e)
	}
	return v
}

func floatPointer(value float64) *float64 { return &value }
func bodyID(h *harness, method, path string, body interface{}, token, company string) string {
	h.t.Helper()
	if store, ok := body.(models.StoreDTO); ok && store.CNPJ == "" {
		// Existing flow tests predate the mandatory store identity contract.
		// Keep those fixtures concise while still exercising the real API with
		// a deterministic, unique, syntactically valid CNPJ.
		value := uint64(crc32.ChecksumIEEE([]byte(store.GroupID.String()+"|"+store.Codigo))) % 1000000000000
		store.CNPJ = fmt.Sprintf("99%012d", value)
		body = store
	}
	status, data := h.request(method, path, body, token, company)
	if status < 200 || status >= 300 {
		h.t.Fatalf("%s %s recebeu HTTP %d: %#v", method, path, status, data)
	}
	return data["id"].(string)
}
func createAgent(h *harness, store, admin, company string) (string, string) {
	h.t.Helper()
	_, provision := h.statusRequest("POST", "/api/agents", map[string]interface{}{"store_id": store, "nome": "Agent de integraÃ§Ã£o"}, admin, company, 200)
	agent := provision["agent"].(map[string]interface{})
	id := agent["id"].(string)
	apiKey := provision["apiKey"].(string)
	h.agentKeys[id] = apiKey
	return id, apiKey
}

func (h *harness) requestAgentWithKey(method, path string, body interface{}, agentID, apiKey string) (int, map[string]interface{}) {
	h.t.Helper()
	direct := strings.HasPrefix(path, "/api/integracao/v1/")
	var b bytes.Buffer
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			h.t.Fatal(err)
		}
		var payload interface{}
		if err := json.Unmarshal(encoded, &payload); err != nil {
			h.t.Fatal(err)
		}
		if !direct {
			payload = camelizeJSON(payload)
		}
		encoded, err = json.Marshal(payload)
		if err != nil {
			h.t.Fatal(err)
		}
		b.Write(encoded)
	}
	if !direct {
		path = camelizeQuery(path)
	}
	req, err := http.NewRequest(method, h.server.URL+path, &b)
	if err != nil {
		h.t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-API-Key", apiKey)
	req.Header.Set("X-Agent-ID", agentID)
	response, err := h.client.Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	defer response.Body.Close()
	var result map[string]interface{}
	if response.StatusCode != http.StatusNoContent {
		if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
			h.t.Fatalf("%s %s status=%d: resposta JSON invÃƒÂ¡lida: %v", method, path, response.StatusCode, err)
		}
	}
	return response.StatusCode, result
}

func camelizeJSON(value interface{}) interface{} {
	switch value := value.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{}, len(value))
		for key, nested := range value {
			parts := strings.Split(key, "_")
			for index := 1; index < len(parts); index++ {
				if parts[index] != "" {
					parts[index] = strings.ToUpper(parts[index][:1]) + parts[index][1:]
				}
			}
			result[strings.Join(parts, "")] = camelizeJSON(nested)
		}
		return result
	case []interface{}:
		for index := range value {
			value[index] = camelizeJSON(value[index])
		}
		return value
	default:
		return value
	}
}

func camelizeQuery(path string) string {
	for legacy, current := range map[string]string{
		"page_size=": "pageSize=", "group_id=": "groupId=", "store_id=": "storeId=",
		"origin_store_id=": "originStoreId=", "destination_store_id=": "destinationStoreId=",
	} {
		path = strings.ReplaceAll(path, legacy, current)
	}
	return path
}
func (h *harness) statusRequest(method, path string, body interface{}, token, company string, expected int) (int, map[string]interface{}) {
	h.t.Helper()
	s, v := h.request(method, path, body, token, company)
	if s != expected {
		h.t.Fatalf("%s %s: recebeu HTTP %d, esperado %d: %#v", method, path, s, expected, v)
	}
	return s, v
}
func claim(h *harness, token string, limit int) []models.Message {
	h.t.Helper()
	s, v := h.request("GET", fmt.Sprintf("/api/integracao/v1/mensagens/pendentes?limite=%d", limit), nil, token, "")
	if s != 200 {
		h.t.Fatalf("claim HTTP %d %#v", s, v)
	}
	raw, _ := json.Marshal(v["itens"])
	var views []models.IntegrationMessageView
	if e := json.Unmarshal(raw, &views); e != nil {
		h.t.Fatal(e)
	}
	messages := make([]models.Message, 0, len(views))
	for _, view := range views {
		messages = append(messages, models.Message{
			Tenant: models.Tenant{Base: models.Base{ID: view.ID}},
			Type:   view.Type, Operation: view.Operation,
			OriginStoreID: view.OriginStoreID, DestinationStoreID: view.DestinationStoreID,
			Payload: view.Payload, Status: view.Status, Attempts: view.Attempts,
			ClaimToken: view.ClaimToken, LeaseUntil: view.LeaseUntil,
			NextAttemptAt: view.NextAttemptAt, LastAttemptAt: view.LastAttemptAt,
			LastError: view.LastError, AcknowledgedAt: view.AcknowledgedAt,
			TransferID: view.TransferID,
		})
	}
	return messages
}
func ack(h *harness, token string, m models.Message, processed int, failures []models.ItemError) {
	h.t.Helper()
	if m.ClaimToken == nil {
		h.t.Fatalf("mensagem %s veio sem claim", m.ID)
	}
	integrationFailures := make([]models.IntegrationItemError, 0, len(failures))
	for _, failure := range failures {
		integrationFailures = append(integrationFailures, models.IntegrationItemError{
			ItemKey: failure.ItemKey, ErrorCode: failure.ErrorCode, Error: failure.Error,
		})
	}
	body := models.IntegrationAckDTO{ClaimToken: *m.ClaimToken, ProcessedCount: processed, Errors: integrationFailures}
	h.statusRequest("POST", "/api/integracao/v1/mensagens/"+m.ID.String()+"/confirmar", body, token, "", 200)
}

func renew(h *harness, token string, messages []models.Message) models.IntegrationRenewResult {
	h.t.Helper()
	items := make([]models.IntegrationRenewItem, 0, len(messages))
	for _, message := range messages {
		if message.ClaimToken == nil {
			h.t.Fatalf("mensagem %s veio sem claim para renovacao", message.ID)
		}
		items = append(items, models.IntegrationRenewItem{
			ID: message.ID, ClaimToken: *message.ClaimToken,
		})
	}
	status, raw := h.statusRequest("POST", "/api/integracao/v1/mensagens/renovar", models.IntegrationRenewDTO{Items: items}, token, "", 200)
	_ = status
	var result models.IntegrationRenewResult
	encoded, _ := json.Marshal(raw)
	if err := json.Unmarshal(encoded, &result); err != nil {
		h.t.Fatalf("resposta de renovacao invalida: %v %#v", err, raw)
	}
	return result
}

// TestStoreLinkCloudFlows validates real PostgreSQL migrations, tenant isolation,
// queue leasing, idempotence, batch progress, retry/dead-letter and transfers.
func TestStoreLinkCloudFlows(t *testing.T) {
	h := newHarness(t)
	s, login := h.request("POST", "/api/auth/login", models.LoginDTO{Email: "unused", Senha: "unused"}, "", "")
	_ = s
	_ = login
	// The harness user is retrieved from its isolated row and authenticated through the real login route.
	var u models.User
	if e := h.db.First(&u, "id = ?", h.user).Error; e != nil {
		t.Fatal(e)
	}
	s, login = h.request("POST", "/api/auth/login", models.LoginDTO{Email: u.Email, Senha: "password-de-teste-segura"}, "", "")
	if s != 200 {
		t.Fatalf("login HTTP %d %#v", s, login)
	}
	admin := login["token"].(string)
	cnpj := "TEST-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:10]
	company := bodyID(h, "POST", "/api/companies", models.CompanyDTO{Nome: "Empresa integraÃ§Ã£o", CNPJ: cnpj}, admin, "")
	group := bodyID(h, "POST", "/api/groups", models.GroupDTO{Nome: "Grupo principal"}, admin, company)
	otherGroup := bodyID(h, "POST", "/api/groups", models.GroupDTO{Nome: "Grupo isolado"}, admin, company)
	origin := bodyID(h, "POST", "/api/stores", models.StoreDTO{GroupID: mustUUID(t, group), Codigo: "001", Nome: "Matriz", Tipo: "MATRIZ"}, admin, company)
	destination := bodyID(h, "POST", "/api/stores", models.StoreDTO{GroupID: mustUUID(t, group), Codigo: "002", Nome: "Filial", Tipo: "FILIAL"}, admin, company)
	wrongGroupStore := bodyID(h, "POST", "/api/stores", models.StoreDTO{GroupID: mustUUID(t, otherGroup), Codigo: "003", Nome: "Outra rede", Tipo: "FILIAL"}, admin, company)
	originAgent, originKey := createAgent(h, origin, admin, company)
	destinationAgent, destinationKey := createAgent(h, destination, admin, company)
	if status, _ := h.requestAgentWithKey("GET", "/api/agent/messages/pending?limit=1", nil, destinationAgent, originKey); status != http.StatusUnauthorized {
		t.Fatalf("a chave de outro agente deve ser rejeitada no destino, HTTP %d", status)
	}
	if status, _ := h.requestAgentWithKey("GET", "/api/agent/messages/pending?limit=1", nil, originAgent, destinationKey); status != http.StatusUnauthorized {
		t.Fatalf("a chave de outro agente deve ser rejeitada na origem, HTTP %d", status)
	}
	_, rotated := h.statusRequest("POST", "/api/agents/"+originAgent+"/rotate-api-key", nil, admin, company, http.StatusOK)
	newOriginKey, ok := rotated["apiKey"].(string)
	if !ok || newOriginKey == "" || newOriginKey == originKey {
		t.Fatalf("a rota deve devolver uma chave nova apenas ao rotacionar: %#v", rotated)
	}
	if status, _ := h.requestAgentWithKey("GET", "/api/agent/messages/pending?limit=1", nil, originAgent, originKey); status != http.StatusUnauthorized {
		t.Fatalf("a chave anterior deve ser revogada, HTTP %d", status)
	}
	h.agentKeys[originAgent] = newOriginKey
	if status, _ := h.requestAgentWithKey("GET", "/api/agent/messages/pending?limit=1", nil, originAgent, newOriginKey); status != http.StatusOK {
		t.Fatalf("a chave rotacionada deve autenticar o agente, HTTP %d", status)
	}
	status, _ := h.requestAgentWithKey("POST", "/api/agent/heartbeat", models.HeartbeatDTO{Version: "1.0.0", Status: "ONLINE", DatabaseOK: true}, originAgent, "wrong-api-key")
	if status != 401 {
		t.Fatalf("invalid agent API key should return 401; got %d", status)
	}
	status, hb := h.request("POST", "/api/agent/heartbeat", models.HeartbeatDTO{Version: "1.0.0", Status: "ONLINE", DatabaseOK: true, Products: 12}, originAgent, company)
	if status != 200 || hb["status"] != "ONLINE" {
		t.Fatalf("heartbeat HTTP %d %#v", status, hb)
	}
	status, wrongHB := h.request("POST", "/api/agent/heartbeat", models.HeartbeatDTO{Version: "1.0.0", Status: "ONLINE", DatabaseOK: true}, originAgent, "")
	if status != 200 {
		t.Fatalf("agente nÃ£o deve precisar header de tenant: %d %#v", status, wrongHB)
	}

	messageID := uuid.NewString()
	messageBody := map[string]interface{}{"id": messageID, "type": "EVENT", "operation": "product.updated", "destination_store_id": destination, "payload": map[string]interface{}{"id": "p-100", "name": "Produto teste"}}
	status, _ = h.request("POST", "/api/agent/messages", messageBody, originAgent, "")
	if status != 201 {
		t.Fatalf("nova mensagem HTTP %d", status)
	}
	status, _ = h.request("POST", "/api/agent/messages", messageBody, originAgent, "")
	if status != 200 {
		t.Fatalf("reenvio idempotente deveria retornar 200; recebeu %d", status)
	}
	conflict := map[string]interface{}{"id": messageID, "type": "EVENT", "operation": "product.updated", "destination_store_id": destination, "payload": map[string]interface{}{"id": "p-100", "name": "conteÃºdo divergente"}}
	status, _ = h.request("POST", "/api/agent/messages", conflict, originAgent, "")
	if status != 409 {
		t.Fatalf("ID reutilizado com outro conteÃºdo deveria retornar 409; recebeu %d", status)
	}
	status, _ = h.request("POST", "/api/agent/messages", map[string]interface{}{"id": uuid.NewString(), "type": "EVENT", "operation": "product.updated", "destination_store_id": wrongGroupStore, "payload": map[string]string{"id": "p-101"}}, originAgent, "")
	if status != 403 {
		t.Fatalf("rota fora do grupo deveria ser negada, HTTP %d", status)
	}
	initial := claim(h, destinationAgent, 10)
	if len(initial) != 1 || initial[0].ID.String() != messageID {
		t.Fatalf("mensagem foi entregue Ã  loja errada ou duplicada: %#v", initial)
	}
	if renewed := renew(h, destinationAgent, initial); renewed.Renewed != 1 || renewed.LeaseUntil.IsZero() {
		t.Fatalf("reserva nao foi renovada: %#v", renewed)
	}
	h.statusRequest("POST", "/api/agent/messages/"+messageID+"/ack", models.AckDTO{ClaimToken: uuid.New()}, originAgent, "", 404)
	ack(h, destinationAgent, initial[0], 0, nil)
	ack(h, destinationAgent, initial[0], 0, nil)

	jobID := bodyID(h, "POST", "/api/sync/jobs", models.JobDTO{OriginStoreID: mustUUID(t, origin), DestinationStoreID: mustUUID(t, destination), Entity: "product", BatchSize: 2}, admin, company)
	jobListStatus, jobList := h.request("GET", "/api/sync/jobs?page=1&page_size=10", nil, admin, company)
	if jobListStatus != http.StatusOK || jobList["total"].(float64) != 1 {
		t.Fatalf("contexto de cargas nÃ£o listou a carga criada: HTTP %d %#v", jobListStatus, jobList)
	}
	command := claim(h, originAgent, 10)
	if len(command) != 1 || command[0].Operation != "sync.products" {
		t.Fatalf("agente de origem nÃ£o recebeu comando da carga: %#v", command)
	}
	ack(h, originAgent, command[0], 0, nil)
	firstBatch := uuid.New()
	lastBatch := uuid.New()
	productSnapshot := func(localCode, barcode string) map[string]interface{} {
		return map[string]interface{}{"local_code": localCode, "name": "Product " + localCode, "unit": "UN", "active": true, "tracks_serial": false, "group": map[string]interface{}{"local_code": "G1", "name": "General"}, "subgroup": map[string]interface{}{"local_code": "SG1", "name": "Misc"}, "barcodes": []map[string]interface{}{{"value": barcode, "is_primary": true, "active": true}}}
	}
	for _, invalidBoolean := range []interface{}{"N", 0, 1} {
		invalidSnapshot := productSnapshot("invalid", "000000")
		invalidSnapshot["active"] = invalidBoolean
		h.statusRequest("POST", "/api/agent/sync/jobs/"+jobID+"/batches", map[string]interface{}{"id": firstBatch, "batch_number": 1, "total_items": 3, "is_last": false, "items": []map[string]interface{}{invalidSnapshot}}, originAgent, "", 400)
	}
	firstProduct := productSnapshot("item-a", "111111")
	firstProduct["barcodes"] = []map[string]interface{}{{"value": "111111", "is_primary": true, "active": true}, {"value": "111112", "is_primary": false, "active": true}}
	firstItems := []map[string]interface{}{firstProduct, productSnapshot("item-b", "222222")}
	lastItems := []map[string]interface{}{productSnapshot("item-c", "333333")}
	h.statusRequest("POST", "/api/agent/sync/jobs/"+jobID+"/batches", map[string]interface{}{"id": firstBatch, "batch_number": 1, "total_items": 3, "is_last": false, "items": firstItems}, originAgent, "", 201)
	h.statusRequest("POST", "/api/agent/sync/jobs/"+jobID+"/batches", map[string]interface{}{"id": lastBatch, "batch_number": 2, "total_items": 3, "is_last": true, "items": lastItems}, originAgent, "", 201)
	// Sending the same immutable batch again is idempotent and does not enqueue twice.
	h.statusRequest("POST", "/api/agent/sync/jobs/"+jobID+"/batches", map[string]interface{}{"id": lastBatch, "batch_number": 2, "total_items": 3, "is_last": true, "items": lastItems}, originAgent, "", 200)
	batches := claim(h, destinationAgent, 10)
	if len(batches) != 2 {
		t.Fatalf("destino deveria receber exatamente dois lotes, recebeu %d", len(batches))
	}
	for _, batch := range batches {
		var payload struct {
			BatchID string                   `json:"batchId"`
			Items   []map[string]interface{} `json:"items"`
		}
		raw, _ := json.Marshal(batch.Payload)
		_ = json.Unmarshal(raw, &payload)
		switch payload.BatchID {
		case firstBatch.String():
			productID := payload.Items[0]["productId"].(string)
			var storedProduct models.Product
			if err := h.db.First(&storedProduct, "id = ?", productID).Error; err != nil {
				t.Fatalf("canonical product was not persisted: %v", err)
			}
			if storedProduct.Name != "Product item-a" || !storedProduct.Active || storedProduct.TracksSerial || storedProduct.GroupName != "General" || storedProduct.SubgroupName != "Misc" {
				t.Fatalf("product contract fields were not mapped: %#v", storedProduct)
			}
			var barcodeCount int64
			if err := h.db.Model(&models.ProductBarcode{}).Where("id_produto = ?", storedProduct.ID).Count(&barcodeCount).Error; err != nil || barcodeCount != 2 {
				t.Fatalf("expected both product barcodes, count=%d err=%v", barcodeCount, err)
			}
			var localMapping models.ProductStoreMapping
			if err := h.db.Where("id_loja = ? AND codigo_local = ?", mustUUID(t, origin), "item-a").First(&localMapping).Error; err != nil || localMapping.ProductID != storedProduct.ID {
				t.Fatalf("store-scoped local product code was not mapped: %#v err=%v", localMapping, err)
			}
			missingProductID := payload.Items[1]["productId"].(string)
			ack(h, destinationAgent, batch, 1, []models.ItemError{{ItemKey: missingProductID, ErrorCode: "PRODUCT_NOT_FOUND", Error: "product is absent from destination catalog"}})
		case lastBatch.String():
			ack(h, destinationAgent, batch, 1, nil)
		default:
			t.Fatalf("ID de lote inesperado: %s", payload.BatchID)
		}
	}
	jobStatus, job := h.request("GET", "/api/sync/jobs/"+jobID, nil, admin, company)
	if jobStatus != 200 || job["status"] != "COMPLETED_WITH_ERRORS" || job["progress"].(float64) != 100 || job["processedItems"].(float64) != 2 || job["errorItems"].(float64) != 1 {
		t.Fatalf("agregaÃ§Ã£o/progresso da carga incorretos: HTTP %d %#v", jobStatus, job)
	}
	_, jobErrors := h.request("GET", "/api/sync/jobs/"+jobID+"/errors", nil, admin, company)
	errorsList := jobErrors["items"].([]interface{})
	if len(errorsList) != 1 {
		t.Fatalf("esperava um erro de item, recebeu %#v", jobErrors)
	}
	if errorsList[0].(map[string]interface{})["errorCode"] != "PRODUCT_NOT_FOUND" {
		t.Fatalf("product resolution code should be queryable: %#v", errorsList[0])
	}

	transferID := uuid.New()
	transferItemID := uuid.New()
	transferMessageID := uuid.New()
	transfer := map[string]interface{}{"id": transferID, "number": "T-1001", "items": []map[string]interface{}{{"id": transferItemID, "product_id": uuid.New(), "barcode": "7891234567890", "name": "Produto teste", "unit": "UN", "quantity": 2}}}
	transferRequest := map[string]interface{}{"id": transferMessageID, "type": "EVENT", "operation": "transfer.created", "destination_store_id": destination, "payload": transfer}
	status, _ = h.request("POST", "/api/agent/messages", transferRequest, originAgent, "")
	if status != 201 {
		t.Fatalf("criaÃ§Ã£o de transferÃªncia HTTP %d", status)
	}
	transferListStatus, transferList := h.request("GET", "/api/transferencias?page=1&page_size=10", nil, admin, company)
	if transferListStatus != http.StatusOK || transferList["total"].(float64) != 1 {
		t.Fatalf("contexto de transferÃªncias nÃ£o listou a transferÃªncia criada: HTTP %d %#v", transferListStatus, transferList)
	}
	status, _ = h.request("POST", "/api/agent/messages", transferRequest, originAgent, "")
	if status != 200 {
		t.Fatalf("same transfer event must be idempotent; HTTP %d", status)
	}
	deliveredTransfer := claim(h, destinationAgent, 10)
	if len(deliveredTransfer) != 1 || deliveredTransfer[0].TransferID == nil {
		t.Fatalf("transferÃªncia nÃ£o foi encaminhada ao destino: %#v", deliveredTransfer)
	}
	ack(h, destinationAgent, deliveredTransfer[0], 1, nil)
	_, transferView := h.request("GET", "/api/transferencias/"+transferID.String(), nil, admin, company)
	savedTransfer := transferView["transfer"].(map[string]interface{})
	if savedTransfer["status"] != "SYNCED" || savedTransfer["receivedAt"] != nil {
		t.Fatalf("ACK de sincronizaÃ§Ã£o nÃ£o deve marcar recebimento fÃ­sico: %#v", savedTransfer)
	}
	transferItems := transferView["items"].([]interface{})
	if len(transferItems) != 1 || transferItems[0].(map[string]interface{})["status"] != "STORED" {
		t.Fatalf("ACK should store the per-item result: %#v", transferItems)
	}
	status, _ = h.request("POST", "/api/agent/messages", map[string]interface{}{"id": uuid.NewString(), "type": "EVENT", "operation": "transfer.received", "destination_store_id": origin, "payload": map[string]interface{}{"id": transferID}}, destinationAgent, "")
	if status != 201 {
		t.Fatalf("confirmaÃ§Ã£o fÃ­sica HTTP %d", status)
	}
	returned := claim(h, originAgent, 10)
	if len(returned) != 1 || returned[0].Operation != "transfer.received" {
		t.Fatalf("confirmaÃ§Ã£o nÃ£o retornou Ã  origem: %#v", returned)
	}
	ack(h, originAgent, returned[0], 0, nil)
	_, transferView = h.request("GET", "/api/transferencias/"+transferID.String(), nil, admin, company)
	savedTransfer = transferView["transfer"].(map[string]interface{})
	if savedTransfer["status"] != "RECEIVED" || savedTransfer["receivedAt"] == nil {
		t.Fatalf("evento fÃ­sico explÃ­cito deveria concluir a transferÃªncia: %#v", savedTransfer)
	}

	failedTransferID, failedTransferItemID := uuid.New(), uuid.New()
	failedTransfer := map[string]interface{}{"id": failedTransferID, "number": "T-1002", "items": []map[string]interface{}{{"id": failedTransferItemID, "barcode": "0000000000000", "name": "Produto ausente", "quantity": 1}}}
	h.statusRequest("POST", "/api/agent/messages", map[string]interface{}{"id": uuid.NewString(), "type": "EVENT", "operation": "transfer.created", "destination_store_id": destination, "payload": failedTransfer}, originAgent, "", 201)
	failedDelivery := claim(h, destinationAgent, 10)
	if len(failedDelivery) != 1 {
		t.Fatalf("expected one transfer with an unresolved product: %#v", failedDelivery)
	}
	ack(h, destinationAgent, failedDelivery[0], 0, []models.ItemError{{ItemKey: failedTransferItemID.String(), ErrorCode: "PRODUCT_NOT_FOUND", Error: "no destination product matches the transfer barcode"}})
	_, failedTransferView := h.request("GET", "/api/transferencias/"+failedTransferID.String(), nil, admin, company)
	failedTransferItems := failedTransferView["items"].([]interface{})
	if failedTransferView["transfer"].(map[string]interface{})["status"] != "FAILED" || failedTransferItems[0].(map[string]interface{})["errorCode"] != "PRODUCT_NOT_FOUND" {
		t.Fatalf("product resolution failure is not visible in transfer detail: %#v", failedTransferView)
	}
	h.statusRequest("POST", "/api/transferencias/"+failedTransferID.String()+"/retry", nil, admin, company, 200)
	retriedDelivery := claim(h, destinationAgent, 10)
	if len(retriedDelivery) != 1 || retriedDelivery[0].Operation != "transfer.created" {
		t.Fatalf("failed transfer retry was not delivered: %#v", retriedDelivery)
	}
	ack(h, destinationAgent, retriedDelivery[0], 1, nil)
	_, failedTransferView = h.request("GET", "/api/transferencias/"+failedTransferID.String(), nil, admin, company)
	failedTransferItems = failedTransferView["items"].([]interface{})
	if failedTransferView["transfer"].(map[string]interface{})["status"] != "SYNCED" || failedTransferItems[0].(map[string]interface{})["status"] != "STORED" {
		t.Fatalf("successful retry did not clear transfer failure: %#v", failedTransferView)
	}

	failedID := uuid.NewString()
	h.statusRequest("POST", "/api/agent/messages", map[string]interface{}{"id": failedID, "type": "EVENT", "operation": "customer.updated", "destination_store_id": destination, "payload": map[string]string{"id": "customer-5"}}, originAgent, "", 201)
	pending := claim(h, destinationAgent, 1)
	if len(pending) != 1 || pending[0].ID.String() != failedID {
		t.Fatalf("falha deveria reservar mensagem nova: %#v", pending)
	}
	h.statusRequest("POST", "/api/agent/messages/"+failedID+"/fail", models.FailDTO{ClaimToken: *pending[0].ClaimToken, Error: "temporÃ¡rio"}, destinationAgent, "", 200)
	time.Sleep(300 * time.Millisecond)
	pending = claim(h, destinationAgent, 1)
	if len(pending) != 1 {
		t.Fatalf("retry apÃ³s backoff nÃ£o apareceu: %#v", pending)
	}
	h.statusRequest("POST", "/api/agent/messages/"+failedID+"/fail", models.FailDTO{ClaimToken: *pending[0].ClaimToken, Error: "limite atingido"}, destinationAgent, "", 200)
	_, dead := h.request("GET", "/api/messages/"+failedID, nil, admin, company)
	deadMessage := dead["message"].(map[string]interface{})
	if deadMessage["status"] != "DEAD_LETTER" {
		t.Fatalf("mÃ¡ximo de tentativas deveria usar dead-letter: %#v", deadMessage)
	}
	h.statusRequest("POST", "/api/messages/"+failedID+"/retry", nil, admin, company, 200)
	pending = claim(h, destinationAgent, 1)
	if len(pending) != 1 {
		t.Fatal("reprocessamento manual nÃ£o entrou na fila")
	}
	ack(h, destinationAgent, pending[0], 0, nil)

	// Two simultaneous consumers reserve two rows exactly once with SKIP LOCKED.
	for n := 0; n < 2; n++ {
		h.statusRequest("POST", "/api/agent/messages", map[string]interface{}{"id": uuid.NewString(), "type": "EVENT", "operation": "customer.updated", "destination_store_id": destination, "payload": map[string]string{"id": fmt.Sprintf("parallel-%d", n)}}, originAgent, "", 201)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	var claimed []models.Message
	var errs []error
	for n := 0; n < 2; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req, _ := http.NewRequest("GET", h.server.URL+"/api/agent/messages/pending?limit=1", nil)

			req.Header.Set("X-API-Key", h.agentKeys[destinationAgent])
			req.Header.Set("X-Agent-ID", destinationAgent)
			response, e := h.client.Do(req)
			if e != nil {
				mu.Lock()
				errs = append(errs, e)
				mu.Unlock()
				return
			}
			defer response.Body.Close()
			var result struct {
				Items []models.Message `json:"items"`
			}
			e = json.NewDecoder(response.Body).Decode(&result)
			mu.Lock()
			defer mu.Unlock()
			if e != nil {
				errs = append(errs, e)
				return
			}
			claimed = append(claimed, result.Items...)
		}()
	}
	wg.Wait()
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	if len(claimed) != 2 || claimed[0].ID == claimed[1].ID {
		t.Fatalf("concorrÃªncia reservou incorretamente: %#v", claimed)
	}
	for _, m := range claimed {
		ack(h, destinationAgent, m, 0, nil)
	}

	secondCompany := bodyID(h, "POST", "/api/companies", models.CompanyDTO{Nome: "Outra empresa", CNPJ: "TEST-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:10]}, admin, company)
	secondGroup := bodyID(h, "POST", "/api/groups", models.GroupDTO{Nome: "Grupo privado"}, admin, secondCompany)
	secondStore := bodyID(h, "POST", "/api/stores", models.StoreDTO{GroupID: mustUUID(t, secondGroup), Codigo: "001", Nome: "Privada", Tipo: "MATRIZ"}, admin, secondCompany)
	h.statusRequest("GET", "/api/messages/"+messageID, nil, admin, secondCompany, 404)
	h.statusRequest("POST", "/api/agent/messages", map[string]interface{}{"id": uuid.NewString(), "type": "EVENT", "operation": "product.updated", "destination_store_id": secondStore, "payload": map[string]string{"id": "tenant-leak"}}, originAgent, secondCompany, 404)
	// An injected company header is ignored for agents: identity comes from the signed claim.
	status, _ = h.request("POST", "/api/agent/heartbeat", models.HeartbeatDTO{Version: "1.0.0", Status: "ONLINE", DatabaseOK: true, Products: 22}, originAgent, secondCompany)
	if status != 200 {
		t.Fatalf("heartbeat deve derivar tenant do token, HTTP %d", status)
	}
	var heartbeat models.Heartbeat
	if e := h.db.Where("id_empresa = ? AND id_loja = ?", company, origin).Order("data_hora_criacao DESC").First(&heartbeat).Error; e != nil || heartbeat.Products != 22 {
		t.Fatalf("identidade do agente foi sobrescrita por idempresa: %+v %v", heartbeat, e)
	}
	_, dashboard := h.request("GET", "/api/dashboard", nil, admin, company)
	if dashboard["messagesToday"] == nil || dashboard["online"].(float64) < 1 {
		t.Fatalf("dashboard nÃ£o refletiu heartbeat: %#v", dashboard)
	}
	_, audits := h.request("GET", "/api/audit", nil, admin, company)
	if audits["total"].(float64) < 3 {
		t.Fatalf("auditoria de aÃ§Ãµes nÃ£o foi persistida: %#v", audits)
	}
}

func TestGroupIntegrationSettingsGateMessagesAndSyncJobs(t *testing.T) {
	h := newHarness(t)
	var user models.User
	if err := h.db.First(&user, "id = ?", h.user).Error; err != nil {
		t.Fatal(err)
	}
	_, login := h.request("POST", "/api/auth/login", models.LoginDTO{Email: user.Email, Senha: "password-de-teste-segura"}, "", "")
	admin := login["token"].(string)
	company := bodyID(h, "POST", "/api/companies", models.CompanyDTO{Nome: "Empresa de polÃ­tica", CNPJ: "TEST-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:10]}, admin, "")
	group := bodyID(h, "POST", "/api/groups", models.GroupDTO{Nome: "Rede com configuraÃ§Ã£o"}, admin, company)
	origin := bodyID(h, "POST", "/api/stores", models.StoreDTO{GroupID: mustUUID(t, group), Codigo: "101", Nome: "Origem", Tipo: "MATRIZ"}, admin, company)
	destination := bodyID(h, "POST", "/api/stores", models.StoreDTO{GroupID: mustUUID(t, group), Codigo: "102", Nome: "Destino", Tipo: "FILIAL"}, admin, company)
	h.statusRequest("PUT", "/api/stores/"+origin, models.StoreDTO{GroupID: mustUUID(t, group), Codigo: "101", Nome: "Origem atualizada", CNPJ: "99000000000101", Tipo: "MATRIZ"}, admin, company, http.StatusOK)
	originAgent, originKey := createAgent(h, origin, admin, company)
	destinationAgent, _ := createAgent(h, destination, admin, company)

	status, current := h.request("GET", "/api/groups/"+group, nil, admin, company)
	for _, invalidBoolean := range []interface{}{"S", "N", 0, 1} {
		h.statusRequest("PUT", "/api/groups/"+group, map[string]interface{}{"nome": "Rede com configuraÃƒÂ§ÃƒÂ£o", "integrar_clientes": invalidBoolean}, admin, company, http.StatusBadRequest)
	}
	if status != 200 || current["integrarPrecos"] != true || current["integrarProdutos"] != true || current["integrarClientes"] != true || current["integrarFuncionarios"] != true {
		t.Fatalf("novo grupo deve preservar integraÃ§Ãµes habilitadas: HTTP %d %#v", status, current)
	}

	for _, operation := range []string{"product_group.updated", "product_subgroup.updated", "price.updated", "customer.created", "employee.created"} {
		status, result := h.request("POST", "/api/agent/messages", map[string]interface{}{"id": uuid.NewString(), "type": "EVENT", "operation": operation, "destination_store_id": destination, "payload": map[string]string{"id": "reg-" + operation}}, originAgent, "")
		if status != http.StatusCreated {
			t.Fatalf("operaÃ§Ã£o %s deveria estar liberada por padrÃ£o: HTTP %d %#v", operation, status, result)
		}
	}

	falseValue := false
	settings := models.GroupDTO{Nome: "Rede com configuraÃ§Ã£o", IntegrarPrecos: &falseValue, IntegrarProdutos: &falseValue, IntegrarClientes: &falseValue, IntegrarFuncionarios: &falseValue}
	settings.IntegrarGruposSubgrupos = &falseValue
	h.statusRequest("PUT", "/api/groups/"+group, settings, admin, company, http.StatusOK)
	price := models.ProductPriceSnapshot{LocalCode: "P-ANY", Kind: "SALE", Quantity: 1, Amount: floatPointer(10), Active: &falseValue}
	if status, _ := h.requestAgentWithKey("PUT", "/api/integracao/v1/produtos/P-ANY/precos/SALE/1", price, originAgent, originKey); status != http.StatusConflict {
		t.Fatalf("integraÃ§Ã£o direta de preÃ§os deve respeitar a configuraÃ§Ã£o do grupo, HTTP %d", status)
	}
	status, current = h.request("GET", "/api/groups/"+group, nil, admin, company)
	if status != 200 || current["integrarPrecos"] != false || current["integrarProdutos"] != false || current["integrarGruposSubgrupos"] != false || current["integrarClientes"] != false || current["integrarFuncionarios"] != false {
		t.Fatalf("configuraÃ§Ã£o atualizada nÃ£o foi persistida: HTTP %d %#v", status, current)
	}
	trueValue := true
	h.statusRequest("PUT", "/api/groups/"+group, models.GroupDTO{Nome: "Rede com configuraÃ§Ã£o", IntegrarClientes: &trueValue}, admin, company, http.StatusOK)
	status, current = h.request("GET", "/api/groups/"+group, nil, admin, company)
	if status != 200 || current["integrarPrecos"] != false || current["integrarProdutos"] != false || current["integrarClientes"] != true {
		t.Fatalf("atualizaÃ§Ã£o parcial alterou opÃ§Ãµes omitidas: HTTP %d %#v", status, current)
	}
	h.statusRequest("PUT", "/api/groups/"+group, models.GroupDTO{Nome: "Rede com configuraÃ§Ã£o", IntegrarClientes: &falseValue}, admin, company, http.StatusOK)

	for _, operation := range []string{"product.updated", "product_group.created", "product_subgroup.disabled", "price.updated", "customer.disabled", "employee.updated"} {
		status, result := h.request("POST", "/api/agent/messages", map[string]interface{}{"id": uuid.NewString(), "type": "EVENT", "operation": operation, "destination_store_id": destination, "payload": map[string]string{"id": "blocked-" + operation}}, originAgent, "")
		if status != http.StatusConflict {
			t.Fatalf("operaÃ§Ã£o %s deveria respeitar integraÃ§Ã£o desativada: HTTP %d %#v", operation, status, result)
		}
	}
	status, result := h.request("POST", "/api/agent/messages", map[string]interface{}{"id": uuid.NewString(), "type": "COMMAND", "operation": "product.update_price", "destination_store_id": destination, "payload": map[string]string{"id": "blocked-price-command"}}, originAgent, "")
	if status != http.StatusConflict {
		t.Fatalf("comando de alteraÃ§Ã£o de preÃ§o deveria respeitar a opÃ§Ã£o desligada: HTTP %d %#v", status, result)
	}
	for _, entity := range []string{"product", "customer", "employee"} {
		h.statusRequest("POST", "/api/sync/jobs", models.JobDTO{OriginStoreID: mustUUID(t, origin), DestinationStoreID: mustUUID(t, destination), Entity: entity, BatchSize: 10}, admin, company, http.StatusConflict)
	}
	if got := claim(h, destinationAgent, 10); len(got) != 0 {
		t.Fatalf("mensagens pendentes foram entregues com integraÃ§Ã£o desligada: %#v", got)
	}

	settings.IntegrarPrecos = &trueValue
	settings.IntegrarProdutos = &trueValue
	settings.IntegrarClientes = &trueValue
	settings.IntegrarFuncionarios = &trueValue
	h.statusRequest("PUT", "/api/groups/"+group, settings, admin, company, http.StatusOK)
	queued := claim(h, destinationAgent, 10)
	if len(queued) != 5 {
		t.Fatalf("mensagens devem voltar Ã  fila ao reabilitar o grupo: recebeu %d", len(queued))
	}
	for _, message := range queued {
		ack(h, destinationAgent, message, 0, nil)
	}

	job := bodyID(h, "POST", "/api/sync/jobs", models.JobDTO{OriginStoreID: mustUUID(t, origin), DestinationStoreID: mustUUID(t, destination), Entity: "product", BatchSize: 10}, admin, company)
	settings.IntegrarProdutos = &falseValue
	h.statusRequest("PUT", "/api/groups/"+group, settings, admin, company, http.StatusOK)
	if got := claim(h, originAgent, 10); len(got) != 0 {
		t.Fatalf("comando de carga de produtos foi entregue depois de desativar a integraÃ§Ã£o: %#v", got)
	}
	h.statusRequest("POST", "/api/agent/sync/jobs/"+job+"/batches", map[string]interface{}{"id": uuid.New(), "batch_number": 1, "total_items": 0, "is_last": true, "items": []map[string]string{}}, originAgent, "", http.StatusConflict)
}

func TestHealthAndAuthenticationBoundary(t *testing.T) {
	h := newHarness(t)
	status, health := h.request("GET", "/api/health", nil, "", "")
	if status != 200 || health["status"] != "ok" {
		t.Fatalf("health HTTP %d %#v", status, health)
	}
	h.statusRequest("GET", "/api/dashboard", nil, "", "", 401)
	h.statusRequest("GET", "/api/groups", nil, "", "", 401)
}

func TestDirectFNTSIntegrationResourcesAndConcurrentUpserts(t *testing.T) {
	h := newHarness(t)
	var user models.User
	if err := h.db.First(&user, "id = ?", h.user).Error; err != nil {
		t.Fatal(err)
	}
	_, login := h.request("POST", "/api/auth/login", models.LoginDTO{Email: user.Email, Senha: "password-de-teste-segura"}, "", "")
	admin := login["token"].(string)
	company := bodyID(h, "POST", "/api/companies", models.CompanyDTO{Nome: "Empresa FNTS direta", CNPJ: "TEST-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:10]}, admin, "")
	group := bodyID(h, "POST", "/api/groups", models.GroupDTO{Nome: "Grupo FNTS direto"}, admin, company)
	origin := bodyID(h, "POST", "/api/stores", models.StoreDTO{GroupID: mustUUID(t, group), Codigo: "201", Nome: "Loja de origem", Tipo: "MATRIZ"}, admin, company)
	destination := bodyID(h, "POST", "/api/stores", models.StoreDTO{GroupID: mustUUID(t, group), Codigo: "202", Nome: "Loja de destino", Tipo: "FILIAL"}, admin, company)
	originAgent, originKey := createAgent(h, origin, admin, company)
	destinationAgent, destinationKey := createAgent(h, destination, admin, company)

	active, inactive, usaGrade := true, false, true
	barcodeActive := true
	product := models.ProductSnapshot{
		LocalCode: "P-001", SKU: "MATRIX-001", Name: "Produto inicial", Description: "DescriÃ§Ã£o essencial",
		Unit: "UN", Brand: "FNTS", CodigoMarca: "MARCA-01", CodigoReceita: "REC-01",
		CodigoBarraNovartis: "7891234567890", NCM: "12345678", CEST: "1234567",
		PrecoCustoAnterior: 8.10, PrecoVendaAnterior: 12.30, PrecoFarmaPop: 13.40,
		AliquotaPIS: 1.65, AliquotaCOFINS: 7.60, PFCP: 2.10, PFCPST: 3.20,
		UsaGrade: &usaGrade, Active: &active, TracksSerial: &inactive,
		Group:    &models.ProductCategoryValue{LocalCode: "G-01", Name: "Grupo geral"},
		Subgroup: &models.ProductCategoryValue{LocalCode: "SG-01", Name: "Subgrupo geral"},
		Barcodes: []models.ProductBarcodeValue{{Value: "000000000001", Primary: &active, Active: &barcodeActive}},
	}
	status, productResult := h.requestAgentWithKey("PUT", "/api/integracao/v1/produtos/P-001", product, originAgent, originKey)
	if status != http.StatusOK {
		t.Fatalf("upsert direto de produto HTTP %d: %#v", status, productResult)
	}
	productID := mustUUID(t, productResult["id_produto"])
	product.Name = "Produto atualizado"
	product.Group = &models.ProductCategoryValue{LocalCode: "G-02", Name: "Grupo atualizado"}
	product.Subgroup = &models.ProductCategoryValue{LocalCode: "SG-02", Name: "Subgrupo atualizado"}
	status, updatedProduct := h.requestAgentWithKey("PUT", "/api/integracao/v1/produtos/P-001", product, originAgent, originKey)
	if status != http.StatusOK || updatedProduct["id_produto"] != productID.String() || updatedProduct["nome"] != "Produto atualizado" {
		t.Fatalf("update de produto deve manter o cadastro canonico: HTTP %d %#v", status, updatedProduct)
	}
	var savedProduct models.Product
	if err := h.db.First(&savedProduct, "id = ?", productID).Error; err != nil || savedProduct.Name != "Produto atualizado" || savedProduct.TracksSerial || !savedProduct.UsaGrade || savedProduct.CodigoMarca != "MARCA-01" || savedProduct.CodigoReceita != "REC-01" || savedProduct.CodigoBarraNovartis != "7891234567890" || savedProduct.PrecoCustoAnterior != 8.10 || savedProduct.PrecoVendaAnterior != 12.30 || savedProduct.PrecoFarmaPop != 13.40 || savedProduct.AliquotaPIS != 1.65 || savedProduct.AliquotaCOFINS != 7.60 || savedProduct.PFCP != 2.10 || savedProduct.PFCPST != 3.20 || savedProduct.GroupName != "Grupo atualizado" || savedProduct.SubgroupName != "Subgrupo atualizado" {
		t.Fatalf("campos do produto nao foram persistidos: %+v %v", savedProduct, err)
	}
	var savedProductMapping models.ProductStoreMapping
	if err := h.db.Where("id_empresa = ? AND id_loja = ? AND id_produto = ?", company, origin, productID).First(&savedProductMapping).Error; err != nil || savedProductMapping.GroupLocalCode != "G-02" || savedProductMapping.GroupName != "Grupo atualizado" || savedProductMapping.SubgroupLocalCode != "SG-02" || savedProductMapping.SubgroupName != "Subgrupo atualizado" {
		t.Fatalf("cÃ³digos e nomes locais de grupo/subgrupo devem ser persistidos no vÃ­nculo da loja: %+v %v", savedProductMapping, err)
	}
	// A valid EAN is the canonical identity even when the local code changes.
	// Move the existing store mapping instead of creating a second row.
	movedProduct := product
	movedProduct.LocalCode = "P-101"
	movedProduct.ProductID = productID.String()
	status, movedResult := h.requestAgentWithKey("PUT", "/api/integracao/v1/produtos/P-101", movedProduct, originAgent, originKey)
	if status != http.StatusOK || movedResult["id_produto"] != productID.String() {
		t.Fatalf("produto com EAN deve aceitar troca de codigo local: HTTP %d %#v", status, movedResult)
	}
	var movedMappings []models.ProductStoreMapping
	if err := h.db.Where("id_empresa = ? AND id_loja = ? AND id_produto = ?", company, origin, productID).Find(&movedMappings).Error; err != nil || len(movedMappings) != 1 || movedMappings[0].LocalCode != "P-101" {
		t.Fatalf("troca de codigo deve manter um unico vinculo atualizado: %+v %v", movedMappings, err)
	}
	occupant := models.ProductSnapshot{LocalCode: "P-003", SKU: "OCCUPANT-003", Name: "Produto ocupante", Unit: "UN", Active: &active, TracksSerial: &inactive, Barcodes: []models.ProductBarcodeValue{}}
	if status, _ := h.requestAgentWithKey("PUT", "/api/integracao/v1/produtos/P-003", occupant, originAgent, originKey); status != http.StatusOK {
		t.Fatalf("produto ocupante de teste nao foi criado: HTTP %d", status)
	}
	occupiedProduct := movedProduct
	occupiedProduct.LocalCode = "P-003"
	occupiedProduct.SKU = "MATRIX-001"
	if status, _ := h.requestAgentWithKey("PUT", "/api/integracao/v1/produtos/P-003", occupiedProduct, originAgent, originKey); status != http.StatusConflict {
		t.Fatalf("codigo local ocupado por outro produto deve ser rejeitado sem sobrescrever: HTTP %d", status)
	}
	var savedBarcode models.ProductBarcode
	if err := h.db.Where("id_produto = ?", productID).First(&savedBarcode).Error; err != nil || savedBarcode.Value != "000000000001" || !savedBarcode.Primary {
		t.Fatalf("codigo de barras nao foi preservado: %+v %v", savedBarcode, err)
	}
	foreignCompany := bodyID(h, "POST", "/api/companies", models.CompanyDTO{Nome: "Empresa isolada", CNPJ: "TEST-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:10]}, admin, "")
	foreignGroup := bodyID(h, "POST", "/api/groups", models.GroupDTO{Nome: "Grupo isolado"}, admin, foreignCompany)
	foreignStore := bodyID(h, "POST", "/api/stores", models.StoreDTO{GroupID: mustUUID(t, foreignGroup), Codigo: "901", Nome: "Loja isolada", Tipo: "MATRIZ"}, admin, foreignCompany)
	foreignAgent, foreignKey := createAgent(h, foreignStore, admin, foreignCompany)
	foreignProduct := models.ProductSnapshot{LocalCode: "FOREIGN-001", Name: "Produto privado de outra empresa", Unit: "UN", Active: &active, TracksSerial: &inactive, Barcodes: []models.ProductBarcodeValue{}}
	status, foreignProductResult := h.requestAgentWithKey("PUT", "/api/integracao/v1/produtos/FOREIGN-001", foreignProduct, foreignAgent, foreignKey)
	if status != http.StatusOK {
		t.Fatalf("produto de teste isolado HTTP %d: %#v", status, foreignProductResult)
	}
	foreignProductID := mustUUID(t, foreignProductResult["id_produto"])
	crossTenantProduct := models.ProductSnapshot{ProductID: foreignProductID.String(), LocalCode: "CROSS-TENANT", Name: "nao pode sobrescrever outro tenant", Unit: "UN", Active: &active, TracksSerial: &inactive, Barcodes: []models.ProductBarcodeValue{}}
	if status, _ := h.requestAgentWithKey("PUT", "/api/integracao/v1/produtos/CROSS-TENANT", crossTenantProduct, originAgent, originKey); status != http.StatusConflict {
		t.Fatalf("productId de outro tenant deve ser rejeitado no upsert individual, HTTP %d", status)
	}
	if status, _ := h.requestAgentWithKey("POST", "/api/integracao/v1/produtos/lotes", map[string]interface{}{"itens": []models.ProductSnapshot{crossTenantProduct}}, originAgent, originKey); status != http.StatusConflict {
		t.Fatalf("productId de outro tenant deve ser rejeitado no lote, HTTP %d", status)
	}
	guardedProduct := models.Product{Tenant: models.NewTenant(mustUUID(t, company)), Name: "tentativa de sobrescrita", Active: true}
	guardedProduct.ID = foreignProductID
	if err := repositories.New(h.db).Scoped(context.Background(), mustUUID(t, company)).UpsertProducts([]models.Product{guardedProduct}); err == nil {
		t.Fatal("camada de repositÃ³rio deve bloquear colisÃ£o global de id de produto")
	}
	var unchangedForeignProduct models.Product
	if err := h.db.First(&unchangedForeignProduct, "id = ?", foreignProductID).Error; err != nil || unchangedForeignProduct.Name != "Produto privado de outra empresa" {
		t.Fatalf("tentativa cross-tenant alterou produto de outra empresa: %+v %v", unchangedForeignProduct, err)
	}
	batchProduct := product
	batchProduct.LocalCode = "P-002"
	batchProduct.SKU = "MATRIX-002"
	batchProduct.Name = "Produto em lote"
	batchBarcode := true
	batchProduct.Barcodes = []models.ProductBarcodeValue{{Value: "000000000002", Primary: &active, Active: &batchBarcode}}
	status, productBatch := h.requestAgentWithKey("POST", "/api/integracao/v1/produtos/lotes", map[string]interface{}{"itens": []models.ProductSnapshot{batchProduct}}, originAgent, originKey)
	if status != http.StatusOK || productBatch["quantidade_processada"] != float64(1) {
		t.Fatalf("lote de produtos HTTP %d: %#v", status, productBatch)
	}
	var batchProductID uuid.UUID
	for _, item := range productBatch["itens"].([]interface{}) {
		batchProductID = mustUUID(t, item.(map[string]interface{})["id"])
	}
	var batchBarcodeRow models.ProductBarcode
	if err := h.db.Where("id_empresa = ? AND id_produto = ? AND codigo_barras_normalizado = ?", company, batchProductID, "000000000002").First(&batchBarcodeRow).Error; err != nil || !batchBarcodeRow.Primary || !batchBarcodeRow.Active {
		t.Fatalf("lote de produto n\u00e3o persistiu codigo de barras principal: %+v %v", batchBarcodeRow, err)
	}
	loadProducts := make([]models.ProductSnapshot, 1000)
	loadPrefix := "LOAD-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	for index := range loadProducts {
		code := fmt.Sprintf("%s-%04d", loadPrefix, index)
		loadProducts[index] = models.ProductSnapshot{
			LocalCode: code, SKU: code, Name: "Produto de carga " + code, Unit: "UN",
			Active: &active, TracksSerial: &inactive, Barcodes: []models.ProductBarcodeValue{},
		}
	}
	loadStarted := time.Now()
	status, loadResult := h.requestAgentWithKey("POST", "/api/integracao/v1/produtos/lotes", map[string]interface{}{"itens": loadProducts}, originAgent, originKey)
	loadElapsed := time.Since(loadStarted)
	if status != http.StatusOK || loadResult["quantidade_processada"] != float64(len(loadProducts)) {
		t.Fatalf("lote mÃ¡ximo de 1000 produtos HTTP %d apÃ³s %s: %#v", status, loadElapsed, loadResult)
	}
	t.Logf("lote de %d produtos aceito em %s (%.0f registros/s)", len(loadProducts), loadElapsed, float64(len(loadProducts))/loadElapsed.Seconds())
	price := models.ProductPriceSnapshot{LocalCode: "P-101", Kind: "SALE", Quantity: 1, Amount: floatPointer(19.9), Active: &active}
	if status, _ := h.requestAgentWithKey("PUT", "/api/integracao/v1/produtos/P-101/precos/SALE/not-a-number", price, originAgent, originKey); status != http.StatusBadRequest {
		t.Fatalf("quantidade invÃ¡lida na URL deve ser rejeitada, HTTP %d", status)
	}
	if status, _ := h.requestAgentWithKey("PUT", "/api/integracao/v1/produtos/P-101/precos/SALE/2", price, originAgent, originKey); status != http.StatusBadRequest {
		t.Fatalf("chave da URL e quantidade do preÃ§o devem coincidir, HTTP %d", status)
	}
	invalidPrecision := price
	invalidPrecision.Quantity = 1.00001
	if status, _ := h.requestAgentWithKey("PUT", "/api/integracao/v1/produtos/P-101/precos/SALE/1.00001", invalidPrecision, originAgent, originKey); status != http.StatusBadRequest {
		t.Fatalf("preÃ§o nÃ£o pode ultrapassar quatro casas decimais, HTTP %d", status)
	}
	if status, _ := h.requestAgentWithKey("PUT", "/api/integracao/v1/produtos/P-UNKNOWN/precos/SALE/1", models.ProductPriceSnapshot{LocalCode: "P-UNKNOWN", Kind: "SALE", Quantity: 1, Amount: floatPointer(10), Active: &active}, originAgent, originKey); status != http.StatusConflict {
		t.Fatalf("preÃ§o deve exigir produto jÃ¡ mapeado para a loja, HTTP %d", status)
	}
	if status, _ := h.requestAgentWithKey("POST", "/api/integracao/v1/precos-produtos/lotes", map[string]interface{}{"itens": []models.ProductPriceSnapshot{price, price}}, originAgent, originKey); status != http.StatusBadRequest {
		t.Fatalf("lote deve rejeitar tipos/quantidades de preÃ§o duplicados, HTTP %d", status)
	}
	status, priceResult := h.requestAgentWithKey("PUT", "/api/integracao/v1/produtos/P-101/precos/SALE/1", price, originAgent, originKey)
	if status != http.StatusOK || priceResult["tipo"] != "SALE" || priceResult["valor"] != float64(19.9) {
		t.Fatalf("upsert direto de preÃ§o HTTP %d: %#v", status, priceResult)
	}
	price.Amount = floatPointer(21.5)
	price.Active = &inactive
	status, priceResult = h.requestAgentWithKey("PUT", "/api/integracao/v1/produtos/P-101/precos/SALE/1", price, originAgent, originKey)
	if status != http.StatusOK || priceResult["ativo"] != false || priceResult["valor"] != float64(21.5) {
		t.Fatalf("reenvio deve atualizar o preÃ§o e preservar active=false, HTTP %d: %#v", status, priceResult)
	}
	loadPrices := make([]models.ProductPriceSnapshot, len(loadProducts))
	for index, productPrice := range loadProducts {
		loadPrices[index] = models.ProductPriceSnapshot{LocalCode: productPrice.LocalCode, Kind: "WHOLESALE", Quantity: 1, Amount: floatPointer(8.75), Active: &active}
	}
	status, loadPriceResult := h.requestAgentWithKey("POST", "/api/integracao/v1/precos-produtos/lotes", map[string]interface{}{"itens": loadPrices}, originAgent, originKey)
	if status != http.StatusOK || loadPriceResult["quantidade_processada"] != float64(len(loadPrices)) {
		t.Fatalf("lote de preÃ§os para 1000 produtos HTTP %d: %#v", status, loadPriceResult)
	}
	var persistedPrice models.ProductPrice
	if err := h.db.Where("id_empresa = ? AND id_loja = ? AND id_produto = ? AND tipo = ? AND quantity = ?", company, origin, productID, "SALE", 1).First(&persistedPrice).Error; err != nil || persistedPrice.Amount != 21.5 || persistedPrice.Active {
		t.Fatalf("preÃ§o atualizado nÃ£o foi persistido: %+v %v", persistedPrice, err)
	}
	var wholesalePriceCount int64
	if err := h.db.Model(&models.ProductPrice{}).Where("id_empresa = ? AND id_loja = ? AND tipo = ?", company, origin, "WHOLESALE").Count(&wholesalePriceCount).Error; err != nil || wholesalePriceCount != int64(len(loadPrices)) {
		t.Fatalf("lote de preÃ§os nÃ£o foi aplicado completamente: count=%d err=%v", wholesalePriceCount, err)
	}

	customer := models.CustomerSnapshot{LocalCode: "C-001", Name: "Cliente FNTS", TaxIdentifier: "529.982.247-25", Phone: "1133334444", Mobile: "11999998888", Email: "cliente@example.test", Address: models.Address{Street: "Rua Central", Number: "10", Complement: "Sala 2", District: "Centro", City: "Sao Paulo", State: "SP", PostalCode: "01001000"}, CreditStatus: "STANDARD"}
	status, customerResult := h.requestAgentWithKey("PUT", "/api/integracao/v1/clientes/C-001", customer, originAgent, originKey)
	if status != http.StatusOK {
		t.Fatalf("upsert direto de cliente HTTP %d: %#v", status, customerResult)
	}
	customerID := mustUUID(t, customerResult["id"])
	customer.ID = customerID.String()
	customer.Name = "Cliente FNTS atualizado"
	status, updatedCustomer := h.requestAgentWithKey("PUT", "/api/integracao/v1/clientes/C-001", customer, originAgent, originKey)
	if status != http.StatusOK || updatedCustomer["id"] != customerID.String() || updatedCustomer["nome"] != "Cliente FNTS atualizado" {
		t.Fatalf("update de cliente deve manter a identidade canonica: HTTP %d %#v", status, updatedCustomer)
	}
	status, replicaCustomer := h.requestAgentWithKey("PUT", "/api/integracao/v1/clientes/C-009", models.CustomerSnapshot{LocalCode: "C-009", Name: "Cliente FNTS", TaxIdentifier: customer.TaxIdentifier, Email: customer.Email, Address: customer.Address, CreditStatus: "STANDARD"}, destinationAgent, destinationKey)
	if status != http.StatusOK || replicaCustomer["id"] != customerID.String() {
		t.Fatalf("documento deve vincular o mesmo cliente canonico em outra loja: HTTP %d %#v", status, replicaCustomer)
	}
	var savedCustomer models.Customer
	if err := h.db.First(&savedCustomer, "id = ?", customerID).Error; err != nil || savedCustomer.Document != "52998224725" || savedCustomer.Address.City != "Sao Paulo" {
		t.Fatalf("dados de cliente nao foram normalizados/persistidos: %+v %v", savedCustomer, err)
	}
	customerBatch := models.CustomerSnapshot{LocalCode: "C-002", Name: "Cliente em lote", CreditStatus: "UNCLASSIFIED"}
	status, customerBatchResult := h.requestAgentWithKey("POST", "/api/integracao/v1/clientes/lotes", map[string]interface{}{"itens": []models.CustomerSnapshot{customerBatch}}, originAgent, originKey)
	if status != http.StatusOK || customerBatchResult["quantidade_processada"] != float64(1) {
		t.Fatalf("lote de clientes HTTP %d: %#v", status, customerBatchResult)
	}
	loadCustomers := make([]models.CustomerSnapshot, 1000)
	loadCustomerPrefix := "LOAD-C-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:8]
	for index := range loadCustomers {
		code := fmt.Sprintf("%s-%04d", loadCustomerPrefix, index)
		loadCustomers[index] = models.CustomerSnapshot{LocalCode: code, Name: "Cliente de carga " + code, CreditStatus: "UNCLASSIFIED"}
	}
	loadStarted = time.Now()
	status, loadCustomerResult := h.requestAgentWithKey("POST", "/api/integracao/v1/clientes/lotes", map[string]interface{}{"itens": loadCustomers}, originAgent, originKey)
	loadElapsed = time.Since(loadStarted)
	if status != http.StatusOK || loadCustomerResult["quantidade_processada"] != float64(len(loadCustomers)) {
		t.Fatalf("lote de 1000 clientes HTTP %d apos %s: %#v", status, loadElapsed, loadCustomerResult)
	}
	t.Logf("lote de %d clientes aceito em %s (%.0f registros/s)", len(loadCustomers), loadElapsed, float64(len(loadCustomers))/loadElapsed.Seconds())
	invalidCustomer := models.CustomerSnapshot{LocalCode: "C-INVALID", Name: "Documento invalido", TaxIdentifier: "ABC", CreditStatus: "STANDARD"}
	if status, _ = h.requestAgentWithKey("PUT", "/api/integracao/v1/clientes/C-INVALID", invalidCustomer, originAgent, originKey); status != http.StatusBadRequest {
		t.Fatalf("documento fiscal malformado deve ser rejeitado, HTTP %d", status)
	}

	canOpen, canView, blind := false, true, false
	employee := models.EmployeeSnapshot{LocalCode: "E-001", Name: "Operadora FNTS", Document: "123.456.789-01", Email: "operadora@example.test", Phone: "11900001111", Role: "CAIXA", Active: &inactive, CashierOperators: []models.CashierOperatorValue{{LocalCode: "OP-001", EmployeeLocalCode: "E-001", CanOpenGeneralCash: &canOpen, CanViewAllReports: &canView, BlindClosing: &blind}}}
	status, employeeResult := h.requestAgentWithKey("PUT", "/api/integracao/v1/funcionarios/E-001", employee, originAgent, originKey)
	if status != http.StatusOK || employeeResult["ativo"] != false {
		t.Fatalf("upsert de funcionario deve aceitar active=false: HTTP %d %#v", status, employeeResult)
	}
	employeeID := mustUUID(t, employeeResult["id"])
	employee.ID = employeeID.String()
	employee.Name = "Operadora FNTS atualizada"
	status, updatedEmployee := h.requestAgentWithKey("PUT", "/api/integracao/v1/funcionarios/E-001", employee, originAgent, originKey)
	if status != http.StatusOK || updatedEmployee["id"] != employeeID.String() || updatedEmployee["ativo"] != false {
		t.Fatalf("update de funcionario deve manter o estado booleano e a identidade: HTTP %d %#v", status, updatedEmployee)
	}
	var savedEmployee models.Employee
	if err := h.db.First(&savedEmployee, "id = ?", employeeID).Error; err != nil || savedEmployee.Active || savedEmployee.Document != "12345678901" {
		t.Fatalf("dados de funcionario nao foram preservados: %+v %v", savedEmployee, err)
	}
	var savedOperator models.CashierOperator
	if err := h.db.Where("id_empresa = ? AND id_loja = ? AND codigo_local = ?", company, origin, "OP-001").First(&savedOperator).Error; err != nil || savedOperator.CanOpenGeneralCash || !savedOperator.CanViewAllReports || savedOperator.BlindClosing {
		t.Fatalf("permissoes booleanas do operador nao foram preservadas: %+v %v", savedOperator, err)
	}
	employeeBatch := models.EmployeeSnapshot{LocalCode: "E-002", Name: "Funcionario em lote", Active: &active}
	status, employeeBatchResult := h.requestAgentWithKey("POST", "/api/integracao/v1/funcionarios/lotes", map[string]interface{}{"itens": []models.EmployeeSnapshot{employeeBatch}}, originAgent, originKey)
	if status != http.StatusOK || employeeBatchResult["quantidade_processada"] != float64(1) {
		t.Fatalf("lote de funcionarios HTTP %d: %#v", status, employeeBatchResult)
	}
	loadEmployees := make([]models.EmployeeSnapshot, 1000)
	loadEmployeePrefix := "LOAD-E-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:8]
	for index := range loadEmployees {
		code := fmt.Sprintf("%s-%04d", loadEmployeePrefix, index)
		loadEmployees[index] = models.EmployeeSnapshot{LocalCode: code, Name: "Funcionario de carga " + code, Active: &active}
	}
	loadStarted = time.Now()
	status, loadEmployeeResult := h.requestAgentWithKey("POST", "/api/integracao/v1/funcionarios/lotes", map[string]interface{}{"itens": loadEmployees}, originAgent, originKey)
	loadElapsed = time.Since(loadStarted)
	if status != http.StatusOK || loadEmployeeResult["quantidade_processada"] != float64(len(loadEmployees)) {
		t.Fatalf("lote de 1000 funcionarios HTTP %d apos %s: %#v", status, loadElapsed, loadEmployeeResult)
	}
	t.Logf("lote de %d funcionarios aceito em %s (%.0f registros/s)", len(loadEmployees), loadElapsed, float64(len(loadEmployees))/loadElapsed.Seconds())
	operator := models.CashierOperatorSnapshot{LocalCode: "OP-002", EmployeeLocalCode: "E-001", CanOpenGeneralCash: &canOpen, CanViewAllReports: &canView, BlindClosing: &blind}
	status, operatorBatch := h.requestAgentWithKey("POST", "/api/integracao/v1/operadores-caixa/lotes", map[string]interface{}{"itens": []models.CashierOperatorSnapshot{operator}}, originAgent, originKey)
	if status != http.StatusOK || operatorBatch["quantidade_processada"] != float64(1) {
		t.Fatalf("lote de operadores HTTP %d: %#v", status, operatorBatch)
	}
	loadOperators := make([]models.CashierOperatorSnapshot, 1000)
	for index, loadedEmployee := range loadEmployees {
		loadOperators[index] = models.CashierOperatorSnapshot{LocalCode: fmt.Sprintf("LOAD-O-%s-%04d", loadEmployeePrefix, index), EmployeeLocalCode: loadedEmployee.LocalCode, CanOpenGeneralCash: &canOpen, CanViewAllReports: &canView, BlindClosing: &blind}
	}
	loadStarted = time.Now()
	status, loadOperatorResult := h.requestAgentWithKey("POST", "/api/integracao/v1/operadores-caixa/lotes", map[string]interface{}{"itens": loadOperators}, originAgent, originKey)
	loadElapsed = time.Since(loadStarted)
	if status != http.StatusOK || loadOperatorResult["quantidade_processada"] != float64(len(loadOperators)) {
		t.Fatalf("lote de 1000 operadores HTTP %d apos %s: %#v", status, loadElapsed, loadOperatorResult)
	}
	t.Logf("lote de %d operadores aceito em %s (%.0f registros/s)", len(loadOperators), loadElapsed, float64(len(loadOperators))/loadElapsed.Seconds())
	operator.LocalCode = "OP-003"
	status, operatorResult := h.requestAgentWithKey("PUT", "/api/integracao/v1/operadores-caixa/OP-003", operator, originAgent, originKey)
	if status != http.StatusOK || operatorResult["codigo_local"] != "OP-003" {
		t.Fatalf("upsert individual de operador HTTP %d: %#v", status, operatorResult)
	}

	_, dashboard := h.request("GET", "/api/dashboard", nil, admin, company)
	if dashboard["products"] != float64(1003) || dashboard["customers"] != float64(1002) || dashboard["employees"] != float64(1002) {
		t.Fatalf("dashboard deve contar cadastros canonicos recebidos diretamente: %#v", dashboard)
	}

	sentAt := time.Date(2026, time.October, 2, 12, 30, 0, 0, time.UTC)
	expiresAt := time.Date(2027, time.October, 2, 0, 0, 0, 0, time.UTC)
	manufacturedAt := time.Date(2026, time.September, 2, 0, 0, 0, 0, time.UTC)
	transfer := models.IntegrationTransferDTO{ID: uuid.New(), Number: "TR-2026-001", DestinationID: mustUUID(t, destination), OriginCNPJ: "12345678000190", DestinationCNPJ: "98765432000110", SenderUsername: "FNTS\\caixa", SenderNote: "Envio completo", SentAt: sentAt, Items: []models.TransferLine{{ID: uuid.New(), LocalProductCode: "P-001", SKU: "MATRIX-001", Barcode: "000000000001", Name: "Produto transferido", Unit: "UN", Quantity: 2.5, SalePrice: 19.9, CostPrice: 9.75, BatchCode: "LOTE-1", LotSerial: "SERIE-LOTE", Lot: "LOTE", Serial: "SERIAL-1", ExpiresAt: &expiresAt, ManufacturedAt: &manufacturedAt, Controlled: &inactive, TracksBatch: &active, TracksSerial: &inactive, RegistrationMS: "MS-123", TherapeuticClass: "ANTIBIOTICO", SNGPCType: "MEDICAMENTO", SNGPCUnit: "CX"}}}
	status, transferResult := h.requestAgentWithKey("POST", "/api/integracao/v1/transferencias", transfer, originAgent, originKey)
	if status != http.StatusCreated {
		t.Fatalf("criaÃ§Ã£o de transferencia REST HTTP %d: %#v", status, transferResult)
	}
	status, duplicateTransfer := h.requestAgentWithKey("POST", "/api/integracao/v1/transferencias", transfer, originAgent, originKey)
	if status != http.StatusOK || duplicateTransfer["duplicado"] != true {
		t.Fatalf("reenvio idempotente da transferencia HTTP %d: %#v", status, duplicateTransfer)
	}
	conflictTransfer := transfer
	conflictTransfer.SenderNote = "conteudo alterado"
	h.statusRequestAgentWithKey("POST", "/api/integracao/v1/transferencias", conflictTransfer, originAgent, originKey, http.StatusConflict)
	status, transferList := h.requestAgentWithKey("GET", "/api/integracao/v1/transferencias?pagina=1&limite=10", nil, destinationAgent, destinationKey)
	if status != http.StatusOK || transferList["total"].(float64) != 1 {
		t.Fatalf("destino deve listar suas transferencias pendentes: HTTP %d %#v", status, transferList)
	}
	status, transferDetail := h.requestAgentWithKey("GET", "/api/integracao/v1/transferencias/"+transfer.ID.String(), nil, destinationAgent, destinationKey)
	if status != http.StatusOK {
		t.Fatalf("detalhe da transferencia no destino HTTP %d: %#v", status, transferDetail)
	}
	detailTransfer := transferDetail["transferencia"].(map[string]interface{})
	if detailTransfer["cnpj_origem"] != transfer.OriginCNPJ || detailTransfer["usuario_remetente"] != transfer.SenderUsername || detailTransfer["observacao_remetente"] != transfer.SenderNote || detailTransfer["data_envio"] == nil {
		t.Fatalf("metadados da transferencia foram perdidos: %#v", detailTransfer)
	}
	var savedTransfer models.Transfer
	if err := h.db.First(&savedTransfer, "id = ?", transfer.ID).Error; err != nil || savedTransfer.PayloadHash == "" {
		t.Fatalf("transferencia sem persistencia de idempotencia: %+v %v", savedTransfer, err)
	}
	var savedTransferItem models.TransferItem
	if err := h.db.Where("id_transferencia = ?", transfer.ID).First(&savedTransferItem).Error; err != nil {
		t.Fatalf("item da transferencia nao persistido: %v", err)
	}
	if savedTransferItem.ProductID == nil || *savedTransferItem.ProductID != productID || savedTransferItem.SalePrice != 19.9 || savedTransferItem.CostPrice != 9.75 || savedTransferItem.BatchCode != "LOTE-1" || savedTransferItem.LotSerial != "SERIE-LOTE" || savedTransferItem.Lot != "LOTE" || savedTransferItem.Serial != "SERIAL-1" || savedTransferItem.Controlled || !savedTransferItem.TracksBatch || savedTransferItem.TracksSerial || savedTransferItem.RegistrationMS != "MS-123" || savedTransferItem.TherapeuticClass != "ANTIBIOTICO" || savedTransferItem.SNGPCType != "MEDICAMENTO" || savedTransferItem.SNGPCUnit != "CX" {
		t.Fatalf("campos essenciais do item foram perdidos: %+v", savedTransferItem)
	}
	h.statusRequestAgentWithKey("PATCH", "/api/integracao/v1/transferencias/"+transfer.ID.String(), models.TransferStatusDTO{Status: "RECEIVED"}, destinationAgent, destinationKey, http.StatusOK)
	if err := h.db.First(&savedTransfer, "id = ?", transfer.ID).Error; err != nil || savedTransfer.Status != "RECEIVED" || savedTransfer.ReceivedAt == nil {
		t.Fatalf("PATCH de recebimento nao atualizou a transferencia: %+v %v", savedTransfer, err)
	}

	failedTransfer := transfer
	failedTransfer.ID = uuid.New()
	failedTransfer.Number = "TR-2026-002"
	failedTransfer.Items = []models.TransferLine{{ID: uuid.New(), LocalProductCode: "P-001", Barcode: "000000000001", Name: "Item com falha", Unit: "UN", Quantity: 1, Controlled: &inactive, TracksBatch: &inactive, TracksSerial: &inactive}}
	status, _ = h.requestAgentWithKey("POST", "/api/integracao/v1/transferencias", failedTransfer, originAgent, originKey)
	if status != http.StatusCreated {
		t.Fatalf("criacao de transferencia para resultado por item HTTP %d", status)
	}
	if status, _ = h.requestAgentWithKey("PATCH", "/api/integracao/v1/transferencias/"+failedTransfer.ID.String(), models.TransferStatusDTO{Status: "RECEIVED"}, originAgent, originKey); status != http.StatusForbidden {
		t.Fatalf("origem nao deve confirmar recebimento fisico; HTTP %d", status)
	}
	if status, _ = h.requestAgentWithKey("PATCH", "/api/integracao/v1/transferencias/"+failedTransfer.ID.String(), models.TransferStatusDTO{Status: "FAILED"}, destinationAgent, destinationKey); status != http.StatusBadRequest {
		t.Fatalf("FAILED deve exigir erros por item; HTTP %d", status)
	}
	itemKey := failedTransfer.Items[0].ID.String()
	failedStatus := models.TransferStatusDTO{Status: "FAILED", ItemErrors: []models.IntegrationItemError{{ItemKey: itemKey, ErrorCode: "RECEIPT_FAILED", Error: "item rejected by destination"}}}
	if status, _ = h.requestAgentWithKey("PATCH", "/api/integracao/v1/transferencias/"+failedTransfer.ID.String(), failedStatus, destinationAgent, destinationKey); status != http.StatusOK {
		t.Fatalf("destino deve registrar falha por item; HTTP %d", status)
	}
	var failedItem models.TransferItem
	if err := h.db.Where("id_transferencia = ?", failedTransfer.ID).First(&failedItem).Error; err != nil || failedItem.Status != "FAILED" || failedItem.ErrorCode != "RECEIPT_FAILED" {
		t.Fatalf("resultado de item da transferencia nao persistido: %+v %v", failedItem, err)
	}
	cancelledTransfer := transfer
	cancelledTransfer.ID = uuid.New()
	cancelledTransfer.Number = "TR-2026-003"
	cancelledTransfer.Items = []models.TransferLine{{ID: uuid.New(), LocalProductCode: "P-001", Barcode: "000000000001", Name: "Item cancelado", Unit: "UN", Quantity: 1, Controlled: &inactive, TracksBatch: &inactive, TracksSerial: &inactive}}
	if status, _ = h.requestAgentWithKey("POST", "/api/integracao/v1/transferencias", cancelledTransfer, originAgent, originKey); status != http.StatusCreated {
		t.Fatalf("criacao de transferencia para cancelamento HTTP %d", status)
	}
	if status, _ = h.requestAgentWithKey("PATCH", "/api/integracao/v1/transferencias/"+cancelledTransfer.ID.String(), models.TransferStatusDTO{Status: "CANCELLED"}, destinationAgent, destinationKey); status != http.StatusForbidden {
		t.Fatalf("somente a origem deve cancelar; HTTP %d", status)
	}
	if status, _ = h.requestAgentWithKey("PATCH", "/api/integracao/v1/transferencias/"+cancelledTransfer.ID.String(), models.TransferStatusDTO{Status: "CANCELLED"}, originAgent, originKey); status != http.StatusOK {
		t.Fatalf("origem nao conseguiu cancelar a transferencia; HTTP %d", status)
	}

	// Simultaneous writes to one external identity are serialized by transactional advisory locks.
	const concurrentRequests = 24
	var wait sync.WaitGroup
	statuses := make([]int, concurrentRequests)
	for index := range statuses {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			concurrentProduct := product
			concurrentProduct.Name = fmt.Sprintf("Concorrente %d", index)
			body, err := json.Marshal(concurrentProduct)
			if err != nil {
				statuses[index] = 0
				return
			}
			request, err := http.NewRequest("PUT", h.server.URL+"/api/integracao/v1/produtos/P-001", bytes.NewReader(body))
			if err != nil {
				statuses[index] = 0
				return
			}
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-API-Key", originKey)
			request.Header.Set("X-Agent-ID", originAgent)
			response, err := h.client.Do(request)
			if err != nil {
				statuses[index] = 0
				return
			}
			statuses[index] = response.StatusCode
			_, _ = io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
		}(index)
	}
	wait.Wait()
	for index, status := range statuses {
		if status != http.StatusOK {
			t.Fatalf("concorrencia de upsert #%d recebeu HTTP %d", index, status)
		}
	}

	message := map[string]interface{}{"id": uuid.NewString(), "type": "EVENT", "operation": "employee.updated", "destinationStoreId": destination, "payload": map[string]string{"id": "employee-event"}}
	if status, _ := h.request("POST", "/api/agent/messages", message, originAgent, ""); status != http.StatusCreated {
		t.Fatalf("nao foi possivel criar evento pendente de funcionario: %d", status)
	}
	falseValue := false
	h.statusRequest("PUT", "/api/groups/"+group, models.GroupDTO{Nome: "Grupo FNTS direto", IntegrarFuncionarios: &falseValue}, admin, company, http.StatusOK)
	blockedEmployee := models.EmployeeSnapshot{LocalCode: "E-BLOCKED", Name: "Nao publicar", Active: &active}
	if status, _ := h.requestAgentWithKey("PUT", "/api/integracao/v1/funcionarios/E-BLOCKED", blockedEmployee, originAgent, originKey); status != http.StatusConflict {
		t.Fatalf("a configuracao do grupo deveria bloquear o cadastro de funcionario: %d", status)
	}
	if pending := claim(h, destinationAgent, 10); len(pending) != 0 {
		t.Fatalf("evento pendente de funcionario ignorou a configuracao do grupo: %#v", pending)
	}
}

func TestDirectIntegrationControllersRejectUnknownFields(t *testing.T) {
	h := newHarness(t)
	var user models.User
	if err := h.db.First(&user, "id = ?", h.user).Error; err != nil {
		t.Fatal(err)
	}
	_, login := h.request("POST", "/api/auth/login", models.LoginDTO{Email: user.Email, Senha: "password-de-teste-segura"}, "", "")
	admin := login["token"].(string)
	company := bodyID(h, "POST", "/api/companies", models.CompanyDTO{Nome: "Empresa contrato estrito", CNPJ: "TEST-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:10]}, admin, "")
	group := bodyID(h, "POST", "/api/groups", models.GroupDTO{Nome: "Grupo contrato estrito"}, admin, company)
	store := bodyID(h, "POST", "/api/stores", models.StoreDTO{GroupID: mustUUID(t, group), Codigo: "STRICT", Nome: "Loja contrato estrito", Tipo: "MATRIZ"}, admin, company)
	agentID, apiKey := createAgent(h, store, admin, company)
	unknown := map[string]interface{}{"unknownField": true}
	requests := []struct {
		method, path string
		body         interface{}
	}{
		{"PUT", "/api/integracao/v1/produtos/P-1", unknown},
		{"POST", "/api/integracao/v1/produtos/lotes", unknown},
		{"PUT", "/api/integracao/v1/produtos/P-1/precos/SALE/1", unknown},
		{"POST", "/api/integracao/v1/precos-produtos/lotes", unknown},
		{"PUT", "/api/integracao/v1/clientes/C-1", unknown},
		{"POST", "/api/integracao/v1/clientes/lotes", unknown},
		{"PUT", "/api/integracao/v1/funcionarios/E-1", unknown},
		{"POST", "/api/integracao/v1/funcionarios/lotes", unknown},
		{"PUT", "/api/integracao/v1/operadores-caixa/O-1", unknown},
		{"POST", "/api/integracao/v1/operadores-caixa/lotes", unknown},
	}
	for _, request := range requests {
		t.Run(request.method+" "+request.path, func(t *testing.T) {
			status, _ := h.requestAgentWithKey(request.method, request.path, request.body, agentID, apiKey)
			if status != http.StatusBadRequest {
				t.Fatalf("unknown DTO field must be rejected with HTTP 400, got %d", status)
			}
		})
	}
}

func (h *harness) statusRequestAgentWithKey(method, path string, body interface{}, agentID, apiKey string, expected int) {
	h.t.Helper()
	status, result := h.requestAgentWithKey(method, path, body, agentID, apiKey)
	if status != expected {
		h.t.Fatalf("%s %s: recebeu HTTP %d, esperado %d: %#v", method, path, status, expected, result)
	}
}
