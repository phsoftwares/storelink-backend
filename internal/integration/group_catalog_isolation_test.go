package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/phsoftwares/storelink-backend/internal/models"
)

type groupScenario struct {
	name, suffix                 string
	products, categories, prices bool
	customers, employees, users  bool
	groupID                      uuid.UUID
	stores                       []testGroupStore
}

type testGroupStore struct {
	id, agentID, apiKey string
}

func TestSixStoreGroupsKeepCatalogsAndPoliciesIsolated(t *testing.T) {
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
		Nome: "Matriz de teste para seis redes",
		CNPJ: "TEST-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:10],
	}, admin, "")

	scenarios := []groupScenario{
		{name: "Rede transferência", suffix: "transfer", users: false},
		{name: "Rede clientes", suffix: "clientes", customers: true},
		{name: "Rede produtos sem preço", suffix: "produto-sem-preco", products: true, categories: true},
		{name: "Rede completa A", suffix: "completa-a", products: true, categories: true, prices: true, customers: true, employees: true, users: true},
		{name: "Rede completa B", suffix: "completa-b", products: true, categories: true, prices: true, customers: true, employees: true, users: true},
		{name: "Rede produtos com preço", suffix: "produto-com-preco", products: true, categories: true, prices: true},
	}

	for index := range scenarios {
		plan := &scenarios[index]
		group := bodyID(h, "POST", "/api/groups", models.GroupDTO{
			Nome:           plan.name,
			IntegrarPrecos: boolGroup(plan.prices), IntegrarProdutos: boolGroup(plan.products),
			IntegrarGruposSubgrupos: boolGroup(plan.categories), IntegrarClientes: boolGroup(plan.customers),
			IntegrarFuncionarios: boolGroup(plan.employees), IntegrarUsuarios: boolGroup(plan.users),
		}, admin, company)
		plan.groupID = mustUUID(t, group)
		plan.stores = make([]testGroupStore, 3)
		for storeIndex := range plan.stores {
			storeCode := fmt.Sprintf("%02d-%02d", index+1, storeIndex+1)
			store := bodyID(h, "POST", "/api/stores", models.StoreDTO{
				GroupID: plan.groupID, Codigo: storeCode,
				Nome: plan.name + " loja " + fmt.Sprint(storeIndex+1),
				Tipo: map[bool]string{true: "MATRIZ", false: "FILIAL"}[storeIndex == 0],
			}, admin, company)
			agentID, apiKey := createAgent(h, store, admin, company)
			plan.stores[storeIndex] = testGroupStore{id: store, agentID: agentID, apiKey: apiKey}
		}
	}

	var productIDs []uuid.UUID
	for index := range scenarios {
		plan := &scenarios[index]
		if !plan.products {
			continue
		}
		localCode := "P-001"
		promotionStart := time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)
		promotionEnd := time.Date(2026, time.October, 31, 0, 0, 0, 0, time.UTC)
		snapshot := models.ProductSnapshot{
			LocalCode: localCode, SKU: "SKU-COMPARTILHADO",
			Name: "Produto exclusivo de " + plan.suffix, Unit: "UN",
			Active: boolGroup(true), TracksSerial: boolGroup(false),
			PrecoVenda: 29.90, PrecoVendaInformado: true,
			PrecoVenda1: 28.90, PrecoPromocao: 19.90,
			DataPromocao: &promotionStart, FimPromocao: &promotionEnd,
			MargemDesconto: 5,
			MargemMinima:   3, DescontoPrecoVenda: 2, FarmaciaPMC: 35,
			UsaTabelaPreco: "SIM", PMargem1: 10, PrecoCustoAnterior: 8,
			PrecoVendaAnterior: 25, PrecoFarmaPop: 22,
			Group:    &models.ProductCategoryValue{LocalCode: "G-01", Name: "Grupo " + plan.suffix},
			Subgroup: &models.ProductCategoryValue{LocalCode: "SG-01", Name: "Subgrupo " + plan.suffix},
			Barcodes: []models.ProductBarcodeValue{{Value: "7890000000001", Primary: boolGroup(true), Active: boolGroup(true)}},
		}
		status, result := h.requestAgentWithKey("PUT", "/api/integracao/v1/produtos/"+localCode,
			snapshot, plan.stores[0].agentID, plan.stores[0].apiKey)
		if status != http.StatusOK {
			t.Fatalf("grupo %s nao carregou produto: HTTP %d %#v", plan.name, status, result)
		}
		productID := mustUUID(t, result["id_produto"])
		productIDs = append(productIDs, productID)

		var stored models.Product
		if err := h.db.Where("id_empresa = ? AND id_grupo = ? AND id = ?", mustUUID(t, company), plan.groupID, productID).First(&stored).Error; err != nil {
			t.Fatalf("produto ausente no grupo %s: %v", plan.name, err)
		}
		if stored.Name != snapshot.Name || stored.GroupName != snapshot.Group.Name || stored.SubgroupName != snapshot.Subgroup.Name {
			t.Fatalf("produto/categorias vazaram entre redes; grupo=%s produto=%+v", plan.name, stored)
		}

		if plan.prices {
			if stored.PrecoVenda != snapshot.PrecoVenda || stored.PrecoVenda1 != snapshot.PrecoVenda1 {
				t.Fatalf("grupo %s deveria conservar os campos de preco: venda=%v venda1=%v", plan.name, stored.PrecoVenda, stored.PrecoVenda1)
			}
		} else if stored.PrecoVenda != 0 || stored.PrecoVenda1 != 0 || stored.PrecoPromocao != 0 ||
			stored.DataPromocao != nil || stored.FimPromocao != nil ||
			stored.MargemDesconto != 0 || stored.MargemMinima != 0 || stored.DescontoPrecoVenda != 0 ||
			stored.FarmaciaPMC != 0 || stored.PMargem1 != 0 || stored.PrecoCustoAnterior != 0 ||
			stored.PrecoVendaAnterior != 0 || stored.PrecoFarmaPop != 0 || stored.UsaTabelaPreco != "NAO" {
			t.Fatalf("grupo %s desativou precos mas persistiu campos de preco: venda=%v tabela=%s", plan.name, stored.PrecoVenda, stored.UsaTabelaPreco)
		}

		price := models.ProductPriceSnapshot{LocalCode: localCode, Kind: "SALE", Quantity: 1, Amount: floatPointer(12.34), Active: boolGroup(true)}
		priceStatus, priceResult := h.requestAgentWithKey("PUT", "/api/integracao/v1/produtos/"+localCode+"/precos/SALE/1", price, plan.stores[0].agentID, plan.stores[0].apiKey)
		if plan.prices && priceStatus != http.StatusOK {
			t.Fatalf("grupo %s deveria aceitar preço: HTTP %d %#v", plan.name, priceStatus, priceResult)
		}
		if !plan.prices && priceStatus != http.StatusConflict {
			t.Fatalf("grupo %s desativou preços, mas recebeu HTTP %d: %#v", plan.name, priceStatus, priceResult)
		}
		var priceCount int64
		if err := h.db.Model(&models.ProductPrice{}).
			Where("id_empresa = ? AND id_grupo = ?", mustUUID(t, company), plan.groupID).
			Count(&priceCount).Error; err != nil {
			t.Fatal(err)
		}
		wantPrices := int64(0)
		if plan.prices {
			wantPrices = 1
		}
		if priceCount != wantPrices {
			t.Fatalf("preços no grupo %s = %d, esperado %d", plan.name, priceCount, wantPrices)
		}
		if plan.products {
			catalogStatus, catalogResult := h.requestAgentWithKey(
				"GET", "/api/integracao/v1/produtos/catalogo?pagina=1&limite=100", nil,
				plan.stores[0].agentID, plan.stores[0].apiKey)
			if catalogStatus != http.StatusOK {
				t.Fatalf("grupo %s nao abriu o catalogo: HTTP %d %#v", plan.name, catalogStatus, catalogResult)
			}
			catalogItems, ok := catalogResult["itens"].([]interface{})
			if !ok || len(catalogItems) != 1 {
				t.Fatalf("catalogo do grupo %s retornou itens inesperados: %#v", plan.name, catalogResult["itens"])
			}
			catalogItem, ok := catalogItems[0].(map[string]interface{})
			if !ok {
				t.Fatalf("catalogo do grupo %s retornou item invalido: %#v", plan.name, catalogItems[0])
			}
			if noPrices, _ := catalogItem["nao_integrar_precos"].(bool); noPrices != !plan.prices {
				t.Fatalf("politica de precos no catalogo do grupo %s = %v, esperado %v", plan.name, noPrices, !plan.prices)
			}
			if authoritative, _ := catalogItem["precos_autoritativos"].(bool); authoritative != plan.prices {
				t.Fatalf("precos_autoritativos do grupo %s = %v, esperado %v", plan.name, authoritative, plan.prices)
			}
			catalogPrices, _ := catalogItem["precos"].([]interface{})
			if got := len(catalogPrices); got != int(wantPrices) {
				t.Fatalf("catalogo do grupo %s entregou %d faixas, esperado %d", plan.name, got, wantPrices)
			}
			catalogProduct, _ := catalogItem["produto"].(map[string]interface{})
			if !plan.prices {
				if value, exists := catalogProduct["preco_venda"]; exists && value != float64(0) {
					t.Fatalf("catalogo sem preco publicou preco_venda=%v", value)
				}
			}

			publication := models.ProductPublicationDTO{
				Event: "UPDATED", Product: snapshot,
				PricesAuthoritative: true,
			}
			if plan.prices {
				publication.Prices = []models.ProductPriceSnapshot{price}
			}
			pStatus, pResult := h.requestAgentWithKey(
				"POST", "/api/integracao/v1/produtos/publicacoes/lotes",
				models.ProductPublicationBatchDTO{Items: []models.ProductPublicationDTO{publication}},
				plan.stores[0].agentID, plan.stores[0].apiKey)
			if pStatus != http.StatusOK {
				t.Fatalf("grupo %s nao deve exigir preco para publicar produto: HTTP %d %#v", plan.name, pStatus, pResult)
			}
			var afterBatch models.Product
			if err := h.db.Where("id_empresa = ? AND id_grupo = ? AND id = ?", mustUUID(t, company), plan.groupID, productID).First(&afterBatch).Error; err != nil {
				t.Fatal(err)
			}
			if !plan.prices && (afterBatch.PrecoVenda != 0 || afterBatch.PrecoVenda1 != 0 || afterBatch.PrecoPromocao != 0 || afterBatch.UsaTabelaPreco != "NAO") {
				t.Fatalf("lote de grupo sem preco persistiu valores: venda=%v venda1=%v promocao=%v tabela=%s", afterBatch.PrecoVenda, afterBatch.PrecoVenda1, afterBatch.PrecoPromocao, afterBatch.UsaTabelaPreco)
			}
			var queued models.Message
			if err := h.db.Where("id_empresa = ? AND id_loja_origem = ? AND id_loja_destino = ? AND operation = ?",
				mustUUID(t, company), mustUUID(t, plan.stores[0].id),
				mustUUID(t, plan.stores[1].id), "product.updated").First(&queued).Error; err != nil {
				t.Fatalf("publicacao nao foi enfileirada para o grupo %s: %v", plan.name, err)
			}
			var wire struct {
				NaoIntegrarPrecos   bool              `json:"nao_integrar_precos"`
				Prices              []json.RawMessage `json:"precos"`
				PricesAuthoritative bool              `json:"precos_autoritativos"`
			}
			if err := json.Unmarshal([]byte(queued.Payload), &wire); err != nil {
				t.Fatalf("payload de publicacao invalido para grupo %s: %v", plan.name, err)
			}
			if wire.NaoIntegrarPrecos != !plan.prices || wire.PricesAuthoritative != plan.prices {
				t.Fatalf("politica de precos da publicacao do grupo %s: nao_integrar=%v autoritativo=%v", plan.name, wire.NaoIntegrarPrecos, wire.PricesAuthoritative)
			}
			if got := len(wire.Prices); got != int(wantPrices) {
				t.Fatalf("publicacao do grupo %s enfileirou %d faixas, esperado %d", plan.name, got, wantPrices)
			}
		}
	}

	for left := range productIDs {
		for right := left + 1; right < len(productIDs); right++ {
			if productIDs[left] == productIDs[right] {
				t.Fatalf("mesmo EAN/SKU compartilhou productId entre grupos: %s", productIDs[left])
			}
		}
	}

	for index := range scenarios {
		plan := &scenarios[index]
		if !plan.customers {
			continue
		}
		customer := models.CustomerSnapshot{
			LocalCode: "CLI-001", Name: "Cliente " + plan.suffix,
			TaxIdentifier: "12345678901", CreditStatus: "STANDARD",
		}
		status, result := h.requestAgentWithKey("PUT", "/api/integracao/v1/clientes/CLI-001", customer,
			plan.stores[0].agentID, plan.stores[0].apiKey)
		if status != http.StatusOK {
			t.Fatalf("grupo %s nao carregou cliente: HTTP %d %#v", plan.name, status, result)
		}
		var saved models.Customer
		if err := h.db.Where("id_empresa = ? AND id_grupo = ? AND regexp_replace(documento, '[^0-9]', '', 'g') = ?", mustUUID(t, company), plan.groupID, customer.TaxIdentifier).First(&saved).Error; err != nil {
			t.Fatalf("cliente ausente no grupo %s: %v", plan.name, err)
		}
		if saved.Name != customer.Name {
			t.Fatalf("dados de cliente vazaram entre grupos: grupo=%s cliente=%+v", plan.name, saved)
		}
	}

	var sameDocumentCustomers []models.Customer
	if err := h.db.Where("id_empresa = ? AND regexp_replace(documento, '[^0-9]', '', 'g') = ?", mustUUID(t, company), "12345678901").Find(&sameDocumentCustomers).Error; err != nil {
		t.Fatal(err)
	}
	if len(sameDocumentCustomers) != 3 {
		t.Fatalf("documento repetido deve existir somente nos três grupos habilitados, encontrados=%d", len(sameDocumentCustomers))
	}

	for index := range scenarios {
		plan := &scenarios[index]
		if !plan.employees {
			continue
		}
		employee := models.EmployeeSnapshot{LocalCode: "EMP-001", Name: "Funcionário " + plan.suffix, Document: "10987654321", Active: boolGroup(true)}
		status, result := h.requestAgentWithKey("PUT", "/api/integracao/v1/funcionarios/EMP-001", employee,
			plan.stores[0].agentID, plan.stores[0].apiKey)
		if status != http.StatusOK {
			t.Fatalf("grupo %s nao carregou funcionário: HTTP %d %#v", plan.name, status, result)
		}
	}
	var sameDocumentEmployees []models.Employee
	if err := h.db.Where("id_empresa = ? AND regexp_replace(documento, '[^0-9]', '', 'g') = ?", mustUUID(t, company), "10987654321").Find(&sameDocumentEmployees).Error; err != nil {
		t.Fatal(err)
	}
	if len(sameDocumentEmployees) != 2 || sameDocumentEmployees[0].ID == sameDocumentEmployees[1].ID {
		t.Fatalf("documento de funcionário deve ficar em linhas distintas nas duas redes completas: %+v", sameDocumentEmployees)
	}

	for index := range scenarios {
		plan := &scenarios[index]
		if plan.products {
			continue
		}
		snapshot := models.ProductSnapshot{LocalCode: "P-BLOQUEADO", Name: "Nao deve entrar", Unit: "UN", Active: boolGroup(true), TracksSerial: boolGroup(false), Barcodes: []models.ProductBarcodeValue{}}
		status, _ := h.requestAgentWithKey("PUT", "/api/integracao/v1/produtos/P-BLOQUEADO", snapshot, plan.stores[0].agentID, plan.stores[0].apiKey)
		if status != http.StatusConflict {
			t.Fatalf("grupo %s desativou produtos, mas aceitou cadastro (HTTP %d)", plan.name, status)
		}
	}

	transferOnly := scenarios[0]
	falseValue := false
	transfer := models.IntegrationTransferDTO{
		ID: uuid.New(), Number: "TRANSFER-ONLY-001", DestinationID: mustUUID(t, transferOnly.stores[1].id),
		SentAt: time.Now().UTC(), Items: []models.TransferLine{{
			ID: uuid.New(), LocalProductCode: "P-TRANSFER", SKU: "TRANSFER-SKU", Name: "Item sem catálogo", Unit: "UN", Quantity: 2,
			Controlled: &falseValue, TracksBatch: &falseValue, TracksSerial: &falseValue,
		}},
	}
	status, transferResult := h.requestAgentWithKey("POST", "/api/integracao/v1/transferencias", transfer,
		transferOnly.stores[0].agentID, transferOnly.stores[0].apiKey)
	if status != http.StatusCreated {
		t.Fatalf("grupo exclusivo de transferência nao aceitou envio: HTTP %d %#v", status, transferResult)
	}
	transferID := mustUUID(t, transferResult["id"])
	status, _ = h.requestAgentWithKey("GET", "/api/integracao/v1/transferencias?status=PENDING", nil,
		transferOnly.stores[1].agentID, transferOnly.stores[1].apiKey)
	if status != http.StatusOK {
		t.Fatalf("destino nao listou a transferência do próprio grupo: HTTP %d", status)
	}
	status, received := h.requestAgentWithKey("PATCH", "/api/integracao/v1/transferencias/"+transferID.String(),
		models.TransferStatusDTO{Status: "RECEIVED"}, transferOnly.stores[1].agentID, transferOnly.stores[1].apiKey)
	if status != http.StatusOK || received["status"] != "RECEIVED" {
		t.Fatalf("destino nao confirmou a transferência: HTTP %d %#v", status, received)
	}

	crossGroupTransfer := transfer
	crossGroupTransfer.ID = uuid.New()
	crossGroupTransfer.DestinationID = mustUUID(t, scenarios[1].stores[0].id)
	status, _ = h.requestAgentWithKey("POST", "/api/integracao/v1/transferencias", crossGroupTransfer,
		transferOnly.stores[0].agentID, transferOnly.stores[0].apiKey)
	if status != http.StatusForbidden {
		t.Fatalf("transferência entre redes deve ser bloqueada, HTTP %d", status)
	}

	var groupCount, storeCount int64
	if err := h.db.Model(&models.Group{}).Where("id_empresa = ?", mustUUID(t, company)).Count(&groupCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := h.db.Model(&models.Store{}).Where("id_empresa = ?", mustUUID(t, company)).Count(&storeCount).Error; err != nil {
		t.Fatal(err)
	}
	if groupCount != 6 || storeCount != 18 || len(h.agentKeys) != 18 {
		t.Fatalf("topologia E2E incorreta: grupos=%d lojas=%d agentes=%d", groupCount, storeCount, len(h.agentKeys))
	}
}

func boolGroup(value bool) *bool { return &value }
