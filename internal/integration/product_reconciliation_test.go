package integration

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"testing"

	"github.com/phsoftwares/storelink-backend/internal/models"
)

func boolValue(value bool) *bool { return &value }

func productPublicationBatch(start, count, storeNumber int) []models.ProductPublicationDTO {
	items := make([]models.ProductPublicationDTO, 0, count)
	for index := 0; index < count; index++ {
		codeNumber := start + index
		code := fmt.Sprintf("%06d", codeNumber)
		barcode := fmt.Sprintf("789700%07d", storeNumber*1000+index)
		amount := float64(codeNumber) / 100
		active := true
		product := models.ProductSnapshot{
			LocalCode: code, SKU: code,
			Name: fmt.Sprintf("Produto loja %d %02d", storeNumber, index+1),
			Unit: "UN", Active: boolValue(true), TracksSerial: boolValue(false),
			Barcodes: []models.ProductBarcodeValue{{
				Value: barcode, Primary: boolValue(true), Active: boolValue(true),
			}},
		}
		if index == 0 && storeNumber == 1 {
			product.PrecoVenda = amount
			product.FarmaciaApresentacao = 123
			product.NCM = "12345678"
			product.CEST = "1234567"
			product.ClassificacaoFiscal = "FISCAL-01"
			product.ClasseTP = "CLASSE-01"
			product.CST = "060"
			product.CodigoMS = "7890001"
			product.PrincipioAtivo = "Principio teste"
			product.NomeLaboratorio = "Laboratorio teste"
			product.CodigoFornecedor = "FOR-01"
			product.NomeFornecedor = "Fornecedor teste"
			product.CNPJFornecedor = "12345678000199"
			product.NomeFantasiaFornecedor = "Fornecedor"
			product.Tipo = "00 - Mercadoria para Revenda"
			product.FarmaciaControlado = "S"
			product.FarmaciaRegistroMedicamento = "REG-01"
			product.Apresentacao = "Caixa teste"
			product.CodigoOriginal = "ORIG-01"
			product.CSOSN = "102"
			product.UsaLote = "S"
			product.UsaBalanca = "N"
			product.SituacaoTributaria = "TRIB-01"
			product.PISCOFINS = "01"
			product.IncidenciaPISCOFINS = "T"
			product.IAT = "A"
			product.IPPT = "T"
			product.IndCFOPVendaDentro = "5102"
			product.UsaTabelaPreco = "SIM"
			product.CodigoClassificacaoTributaria = "000001"
			product.CSTIBS = "000"
			product.CSTCBS = "000"
			product.AliquotaIBS = 0.1
			product.AliquotaCBS = 0.2
			product.AliquotaEfetivaIBS = 0.05
			product.AliquotaEfetivaCBS = 0.06
			product.ObsReformaTributaria = "Observacao"
		}
		items = append(items, models.ProductPublicationDTO{
			Event:   "CREATED",
			Product: product,
			Prices: []models.ProductPriceSnapshot{{
				LocalCode: code,
				Kind:      "SALE",
				Quantity:  1,
				Amount:    &amount,
				Active:    &active,
			}},
		})
	}
	return items
}

func resultLocalCodes(t *testing.T, raw map[string]interface{}) []int {
	t.Helper()
	items, ok := raw["itens"].([]interface{})
	if !ok {
		t.Fatalf("resposta de publicacao sem itens: %#v", raw)
	}
	codes := make([]int, 0, len(items))
	for _, item := range items {
		view, ok := item.(map[string]interface{})
		if !ok {
			t.Fatalf("item de publicacao invalido: %#v", item)
		}
		code, ok := view["codigo_local"].(string)
		if !ok {
			t.Fatalf("codigo canonico ausente: %#v", view)
		}
		value, err := strconv.Atoi(code)
		if err != nil {
			t.Fatalf("codigo canonico nao numerico %q: %v", code, err)
		}
		codes = append(codes, value)
	}
	return codes
}

// TestProductCodeReconciliationFromIndependentStores proves that a store can
// publish without an initial load and that the first confirmed batch keeps the
// disputed canonical codes. The later batch is moved to the next free codes;
// EANs remain the product identity and prices/mappings follow the remap.
func TestProductCodeReconciliationFromIndependentStores(t *testing.T) {
	h := newHarness(t)
	var user models.User
	if err := h.db.First(&user, "id = ?", h.user).Error; err != nil {
		t.Fatal(err)
	}
	_, login := h.request("POST", "/api/auth/login", models.LoginDTO{Email: user.Email, Senha: "password-de-teste-segura"}, "", "")
	admin, ok := login["token"].(string)
	if !ok || admin == "" {
		t.Fatalf("login de teste invalido: %#v", login)
	}
	company := bodyID(h, "POST", "/api/companies", models.CompanyDTO{Nome: "Empresa reconciliacao", CNPJ: "99999999000199"}, admin, "")
	group := bodyID(h, "POST", "/api/groups", models.GroupDTO{Nome: "Grupo reconciliacao"}, admin, company)
	storeOne := bodyID(h, "POST", "/api/stores", models.StoreDTO{
		GroupID: mustUUID(t, group), Codigo: "001", Nome: "Loja um", CNPJ: "11111111000101", Tipo: "FILIAL",
	}, admin, company)
	storeTwo := bodyID(h, "POST", "/api/stores", models.StoreDTO{
		GroupID: mustUUID(t, group), Codigo: "002", Nome: "Loja dois", CNPJ: "22222222000102", Tipo: "FILIAL",
	}, admin, company)
	agentOne, keyOne := createAgent(h, storeOne, admin, company)
	agentTwo, keyTwo := createAgent(h, storeTwo, admin, company)

	firstBatch := productPublicationBatch(700, 25, 1)
	secondBatch := productPublicationBatch(700, 30, 2)
	status, firstResult := h.requestAgentWithKey("POST", "/api/integracao/v1/produtos/publicacoes/lotes", models.ProductPublicationBatchDTO{Items: firstBatch}, agentOne, keyOne)
	if status != http.StatusOK {
		t.Fatalf("primeira publicacao HTTP %d: %#v", status, firstResult)
	}
	firstCodes := resultLocalCodes(t, firstResult)
	if len(firstCodes) != 25 {
		t.Fatalf("primeiro lote processou %d itens", len(firstCodes))
	}

	status, secondResult := h.requestAgentWithKey("POST", "/api/integracao/v1/produtos/publicacoes/lotes", models.ProductPublicationBatchDTO{Items: secondBatch}, agentTwo, keyTwo)
	if status != http.StatusOK {
		t.Fatalf("segunda publicacao HTTP %d: %#v", status, secondResult)
	}
	secondCodes := resultLocalCodes(t, secondResult)
	if len(secondCodes) != 30 {
		t.Fatalf("segundo lote processou %d itens", len(secondCodes))
	}
	for index, code := range firstCodes {
		if code != 700+index {
			t.Fatalf("primeiro lote perdeu a preferencia no codigo %d: %v", index, firstCodes)
		}
	}
	sort.Ints(secondCodes)
	for index, code := range secondCodes {
		if code != 725+index {
			t.Fatalf("segundo lote nao foi remapeado para o proximo intervalo livre: %v", secondCodes)
		}
	}

	var productCount, barcodeCount, mappingCount, priceCount int64
	if err := h.db.Model(&models.Product{}).Count(&productCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := h.db.Model(&models.ProductBarcode{}).Where("ativo = TRUE").Count(&barcodeCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := h.db.Model(&models.ProductStoreMapping{}).Count(&mappingCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := h.db.Model(&models.ProductPrice{}).Count(&priceCount).Error; err != nil {
		t.Fatal(err)
	}
	if productCount != 55 || barcodeCount != 55 || mappingCount != 55 || priceCount != 55 {
		t.Fatalf("reconciliacao nao propagou entidades relacionadas: produtos=%d eans=%d mapeamentos=%d precos=%d", productCount, barcodeCount, mappingCount, priceCount)
	}
	var persisted models.Product
	if err := h.db.Where("nome = ?", "Produto loja 1 01").First(&persisted).Error; err != nil {
		t.Fatal(err)
	}
	if persisted.NCM != "12345678" || persisted.CEST != "1234567" || persisted.ClassificacaoFiscal != "FISCAL-01" || persisted.CST != "060" || persisted.FarmaciaControlado != "S" || persisted.FarmaciaApresentacao != 123 || persisted.CodigoOriginal != "ORIG-01" || persisted.AliquotaIBS != 0.1 || persisted.ObsReformaTributaria != "Observacao" || persisted.UsaTabelaPreco != "SIM" || persisted.PrecoVenda != float64(7) {
		t.Fatalf("atributos complementares do produto nao foram persistidos: %+v", persisted)
	}

	var duplicateEANs int64
	if err := h.db.Raw("SELECT COUNT(*) FROM (SELECT codigo_barras_normalizado FROM produto_codigo_barras WHERE ativo = TRUE GROUP BY codigo_barras_normalizado HAVING COUNT(*) > 1) duplicados").Scan(&duplicateEANs).Error; err != nil {
		t.Fatal(err)
	}
	if duplicateEANs != 0 {
		t.Fatalf("EANs ativos duplicados apos reconciliacao: %d", duplicateEANs)
	}

	var storeOneMappings, storeTwoMappings int64
	if err := h.db.Model(&models.ProductStoreMapping{}).Where("id_loja = ?", storeOne).Count(&storeOneMappings).Error; err != nil {
		t.Fatal(err)
	}
	if err := h.db.Model(&models.ProductStoreMapping{}).Where("id_loja = ?", storeTwo).Count(&storeTwoMappings).Error; err != nil {
		t.Fatal(err)
	}
	if storeOneMappings != 25 || storeTwoMappings != 30 {
		t.Fatalf("mapeamentos por loja incorretos: loja1=%d loja2=%d", storeOneMappings, storeTwoMappings)
	}

	// The first source must have queued the second store's product events, and
	// the second source must also queue events back to the first one. This is
	// the no-initial-load, bidirectional replication guarantee.
	var pendingMessages int64
	if err := h.db.Model(&models.Message{}).Where("status = ?", "PENDING").Count(&pendingMessages).Error; err != nil {
		t.Fatal(err)
	}
	if pendingMessages != 55 {
		t.Fatalf("eventos de replicacao pendentes incorretos: %d", pendingMessages)
	}

}
