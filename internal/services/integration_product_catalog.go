package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/phsoftwares/storelink-backend/internal/models"
	"github.com/phsoftwares/storelink-backend/internal/repositories"
)

type agentProductSnapshot struct {
	ProductID                     string                `json:"productId,omitempty"`
	LocalCode                     string                `json:"localCode"`
	SKU                           string                `json:"sku,omitempty"`
	CamposNulos                   []string              `json:"camposNulos,omitempty"`
	EstoqueMinimo                 float64               `json:"estoqueMinimo,omitempty"`
	MargemMinima                  float64               `json:"margemMinima,omitempty"`
	FlagSis                       string                `json:"flagSis,omitempty"`
	Name                          string                `json:"name"`
	Description                   string                `json:"description,omitempty"`
	Unit                          string                `json:"unit"`
	Brand                         string                `json:"brand,omitempty"`
	CodigoMarca                   string                `json:"codigoMarca,omitempty"`
	DataCadastro                  *time.Time            `json:"dataCadastro,omitempty"`
	Aplicacao                     string                `json:"aplicacao,omitempty"`
	Origem                        string                `json:"origem,omitempty"`
	NCM                           string                `json:"ncm,omitempty"`
	CEST                          string                `json:"cest,omitempty"`
	ClassificacaoFiscal           string                `json:"classificacaoFiscal,omitempty"`
	ClasseTP                      string                `json:"classeTp,omitempty"`
	CST                           string                `json:"cst,omitempty"`
	CodigoMS                      string                `json:"codigoMs,omitempty"`
	PrincipioAtivo                string                `json:"principioAtivo,omitempty"`
	NomeLaboratorio               string                `json:"nomeLaboratorio,omitempty"`
	CodigoFornecedor              string                `json:"codigoFornecedor,omitempty"`
	NomeFornecedor                string                `json:"nomeFornecedor,omitempty"`
	CNPJFornecedor                string                `json:"cnpjFornecedor,omitempty"`
	NomeFantasiaFornecedor        string                `json:"nomeFantasiaFornecedor,omitempty"`
	Tipo                          string                `json:"tipo,omitempty"`
	FarmaciaControlado            string                `json:"farmaciaControlado,omitempty"`
	FarmaciaApresentacao          int64                 `json:"farmaciaApresentacao,omitempty"`
	FarmaciaRegistroMedicamento   string                `json:"farmaciaRegistroMedicamento,omitempty"`
	Apresentacao                  string                `json:"apresentacao,omitempty"`
	CodigoOriginal                string                `json:"codigoOriginal,omitempty"`
	CSOSN                         string                `json:"csosn,omitempty"`
	UsaLote                       string                `json:"usaLote,omitempty"`
	UsaBalanca                    string                `json:"usaBalanca,omitempty"`
	CodigoReceita                 string                `json:"codigoReceita,omitempty"`
	CodigoBarraNovartis           string                `json:"codigoBarraNovartis,omitempty"`
	SituacaoTributaria            string                `json:"situacaoTributaria,omitempty"`
	PISCOFINS                     string                `json:"pisCofins,omitempty"`
	IncidenciaPISCOFINS           string                `json:"incidenciaPisCofins,omitempty"`
	IAT                           string                `json:"iat,omitempty"`
	IPPT                          string                `json:"ippt,omitempty"`
	IndCFOPVendaDentro            string                `json:"indCfopVendaDentro,omitempty"`
	CodigoClassificacaoTributaria string                `json:"codigoClassificacaoTributaria,omitempty"`
	CSTIBS                        string                `json:"cstIbs,omitempty"`
	CSTCBS                        string                `json:"cstCbs,omitempty"`
	AliquotaIBS                   float64               `json:"aliquotaIbs,omitempty"`
	AliquotaCBS                   float64               `json:"aliquotaCbs,omitempty"`
	AliquotaEfetivaIBS            float64               `json:"aliquotaEfetivaIbs,omitempty"`
	AliquotaEfetivaCBS            float64               `json:"aliquotaEfetivaCbs,omitempty"`
	ObsReformaTributaria          string                `json:"obsReformaTributaria,omitempty"`
	PrecoVenda                    float64               `json:"precoVenda,omitempty"`
	PrecoVenda1                   float64               `json:"precoVenda1,omitempty"`
	PrecoVenda2                   float64               `json:"precoVenda2,omitempty"`
	PrecoVenda3                   float64               `json:"precoVenda3,omitempty"`
	PrecoVenda4                   float64               `json:"precoVenda4,omitempty"`
	PrecoVenda5                   float64               `json:"precoVenda5,omitempty"`
	PrecoPromocao                 float64               `json:"precoPromocao,omitempty"`
	MargemDesconto                float64               `json:"margemDesconto,omitempty"`
	Aliquota                      float64               `json:"aliquota,omitempty"`
	QtdeEmbalagem                 float64               `json:"qtdeEmbalagem,omitempty"`
	DescontoPrecoVenda            float64               `json:"descontoPrecoVenda,omitempty"`
	FarmaciaPMC                   float64               `json:"farmaciaPmc,omitempty"`
	DataPromocao                  *time.Time            `json:"dataPromocao,omitempty"`
	FimPromocao                   *time.Time            `json:"fimPromocao,omitempty"`
	UsaTabelaPreco                string                `json:"usaTabelaPreco,omitempty"`
	PMargem1                      float64               `json:"pmargem1,omitempty"`
	PMargem2                      float64               `json:"pmargem2,omitempty"`
	PMargem3                      float64               `json:"pmargem3,omitempty"`
	PMargem4                      float64               `json:"pmargem4,omitempty"`
	PMargem5                      float64               `json:"pmargem5,omitempty"`
	PrecoCustoAnterior            float64               `json:"precoCustoAnterior,omitempty"`
	PrecoVendaAnterior            float64               `json:"precoVendaAnterior,omitempty"`
	PrecoFarmaPop                 float64               `json:"precoFarmaPop,omitempty"`
	AliquotaPIS                   float64               `json:"aliquotaPis,omitempty"`
	AliquotaCOFINS                float64               `json:"aliquotaCofins,omitempty"`
	PFCP                          float64               `json:"pfcp,omitempty"`
	PFCPST                        float64               `json:"pfcpst,omitempty"`
	Active                        *bool                 `json:"active"`
	TracksSerial                  *bool                 `json:"tracksSerial"`
	UsaGrade                      *bool                 `json:"usaGrade,omitempty"`
	Group                         *agentProductCategory `json:"group,omitempty"`
	Subgroup                      *agentProductCategory `json:"subgroup,omitempty"`
	Barcodes                      []agentProductBarcode `json:"barcodes"`
}

type agentProductCategory struct {
	LocalCode string `json:"localCode,omitempty"`
	Name      string `json:"name"`
}

type agentProductBarcode struct {
	Value    string   `json:"value"`
	Primary  *bool    `json:"isPrimary"`
	Active   *bool    `json:"active"`
	Fraction *float64 `json:"fraction,omitempty"`
	C000055  *bool    `json:"c00055,omitempty"`
}

func decodeProductSnapshot(raw models.JSON) (models.ProductSnapshot, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		if err == nil {
			err = fmt.Errorf("produto deve ser um objeto JSON")
		}
		return models.ProductSnapshot{}, err
	}

	legacy := false
	for _, key := range []string{"productId", "localCode", "name", "description", "unit", "brand", "active", "tracksSerial", "group", "subgroup", "barcodes"} {
		if _, found := fields[key]; found {
			legacy = true
			break
		}
	}
	if !legacy {
		var snapshot models.ProductSnapshot
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&snapshot); err != nil {
			return models.ProductSnapshot{}, err
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			if err == nil {
				return models.ProductSnapshot{}, fmt.Errorf("conteudo JSON adicional")
			}
			return models.ProductSnapshot{}, err
		}
		return snapshot, nil
	}

	var value agentProductSnapshot
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return models.ProductSnapshot{}, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		if err == nil {
			return models.ProductSnapshot{}, fmt.Errorf("conteudo JSON adicional")
		}
		return models.ProductSnapshot{}, err
	}

	snapshot := models.ProductSnapshot{
		ProductID: value.ProductID, LocalCode: value.LocalCode, SKU: value.SKU,
		CamposNulos:   value.CamposNulos,
		EstoqueMinimo: value.EstoqueMinimo, MargemMinima: value.MargemMinima, FlagSis: value.FlagSis,
		Name: value.Name, Description: value.Description, Unit: value.Unit,
		Brand: value.Brand, CodigoMarca: value.CodigoMarca, DataCadastro: value.DataCadastro,
		Aplicacao: value.Aplicacao, Origem: value.Origem,
		NCM: value.NCM, CEST: value.CEST,
		ClassificacaoFiscal: value.ClassificacaoFiscal, ClasseTP: value.ClasseTP,
		CST: value.CST, CodigoMS: value.CodigoMS, PrincipioAtivo: value.PrincipioAtivo,
		NomeLaboratorio: value.NomeLaboratorio, CodigoFornecedor: value.CodigoFornecedor,
		NomeFornecedor: value.NomeFornecedor, CNPJFornecedor: value.CNPJFornecedor,
		NomeFantasiaFornecedor: value.NomeFantasiaFornecedor, Tipo: value.Tipo,
		FarmaciaControlado:          value.FarmaciaControlado,
		FarmaciaApresentacao:        value.FarmaciaApresentacao,
		FarmaciaRegistroMedicamento: value.FarmaciaRegistroMedicamento,
		Apresentacao:                value.Apresentacao, CodigoOriginal: value.CodigoOriginal,
		CSOSN: value.CSOSN, UsaLote: value.UsaLote, UsaBalanca: value.UsaBalanca,
		CodigoReceita: value.CodigoReceita, CodigoBarraNovartis: value.CodigoBarraNovartis,
		SituacaoTributaria: value.SituacaoTributaria, PISCOFINS: value.PISCOFINS,
		IncidenciaPISCOFINS: value.IncidenciaPISCOFINS, IAT: value.IAT, IPPT: value.IPPT,
		IndCFOPVendaDentro:            value.IndCFOPVendaDentro,
		CodigoClassificacaoTributaria: value.CodigoClassificacaoTributaria,
		CSTIBS:                        value.CSTIBS, CSTCBS: value.CSTCBS,
		AliquotaIBS: value.AliquotaIBS, AliquotaCBS: value.AliquotaCBS,
		AliquotaEfetivaIBS:   value.AliquotaEfetivaIBS,
		AliquotaEfetivaCBS:   value.AliquotaEfetivaCBS,
		ObsReformaTributaria: value.ObsReformaTributaria,
		PrecoVenda:           value.PrecoVenda,
		PrecoVenda1:          value.PrecoVenda1, PrecoVenda2: value.PrecoVenda2,
		PrecoVenda3: value.PrecoVenda3, PrecoVenda4: value.PrecoVenda4,
		PrecoVenda5: value.PrecoVenda5, PrecoPromocao: value.PrecoPromocao,
		MargemDesconto: value.MargemDesconto, Aliquota: value.Aliquota,
		QtdeEmbalagem:      value.QtdeEmbalagem,
		DescontoPrecoVenda: value.DescontoPrecoVenda,
		FarmaciaPMC:        value.FarmaciaPMC, DataPromocao: value.DataPromocao,
		FimPromocao: value.FimPromocao, UsaTabelaPreco: value.UsaTabelaPreco,
		PMargem1: value.PMargem1,
		PMargem2: value.PMargem2, PMargem3: value.PMargem3,
		PMargem4: value.PMargem4, PMargem5: value.PMargem5,
		PrecoCustoAnterior: value.PrecoCustoAnterior, PrecoVendaAnterior: value.PrecoVendaAnterior,
		PrecoFarmaPop: value.PrecoFarmaPop, AliquotaPIS: value.AliquotaPIS,
		AliquotaCOFINS: value.AliquotaCOFINS, PFCP: value.PFCP, PFCPST: value.PFCPST,
		Active: value.Active, TracksSerial: value.TracksSerial, UsaGrade: value.UsaGrade,
		Barcodes: make([]models.ProductBarcodeValue, 0, len(value.Barcodes)),
	}
	if value.Group != nil {
		snapshot.Group = &models.ProductCategoryValue{LocalCode: value.Group.LocalCode, Name: value.Group.Name}
	}
	if value.Subgroup != nil {
		snapshot.Subgroup = &models.ProductCategoryValue{LocalCode: value.Subgroup.LocalCode, Name: value.Subgroup.Name}
	}
	for _, barcode := range value.Barcodes {
		snapshot.Barcodes = append(snapshot.Barcodes, models.ProductBarcodeValue{
			Value: barcode.Value, Primary: barcode.Primary, Active: barcode.Active, Fraction: barcode.Fraction, C000055: barcode.C000055,
		})
	}
	return snapshot, nil
}

func isLegacyAgentProductSnapshot(raw models.JSON) bool {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return false
	}
	for _, key := range []string{"productId", "localCode", "name", "description", "unit", "brand", "active", "tracksSerial", "group", "subgroup", "barcodes"} {
		if _, found := fields[key]; found {
			return true
		}
	}
	return false
}

func toAgentProductSnapshot(snapshot models.ProductSnapshot) agentProductSnapshot {
	value := agentProductSnapshot{
		ProductID: snapshot.ProductID, LocalCode: snapshot.LocalCode, SKU: snapshot.SKU,
		CamposNulos:   snapshot.CamposNulos,
		EstoqueMinimo: snapshot.EstoqueMinimo, MargemMinima: snapshot.MargemMinima, FlagSis: snapshot.FlagSis,
		Name: snapshot.Name, Description: snapshot.Description, Unit: snapshot.Unit,
		Brand: snapshot.Brand, CodigoMarca: snapshot.CodigoMarca, DataCadastro: snapshot.DataCadastro,
		Aplicacao: snapshot.Aplicacao, Origem: snapshot.Origem,
		NCM: snapshot.NCM, CEST: snapshot.CEST,
		ClassificacaoFiscal: snapshot.ClassificacaoFiscal, ClasseTP: snapshot.ClasseTP,
		CST: snapshot.CST, CodigoMS: snapshot.CodigoMS, PrincipioAtivo: snapshot.PrincipioAtivo,
		NomeLaboratorio: snapshot.NomeLaboratorio, CodigoFornecedor: snapshot.CodigoFornecedor,
		NomeFornecedor: snapshot.NomeFornecedor, CNPJFornecedor: snapshot.CNPJFornecedor,
		NomeFantasiaFornecedor: snapshot.NomeFantasiaFornecedor, Tipo: snapshot.Tipo,
		FarmaciaControlado:          snapshot.FarmaciaControlado,
		FarmaciaApresentacao:        snapshot.FarmaciaApresentacao,
		FarmaciaRegistroMedicamento: snapshot.FarmaciaRegistroMedicamento,
		Apresentacao:                snapshot.Apresentacao, CodigoOriginal: snapshot.CodigoOriginal,
		CSOSN: snapshot.CSOSN, UsaLote: snapshot.UsaLote, UsaBalanca: snapshot.UsaBalanca,
		CodigoReceita: snapshot.CodigoReceita, CodigoBarraNovartis: snapshot.CodigoBarraNovartis,
		SituacaoTributaria: snapshot.SituacaoTributaria, PISCOFINS: snapshot.PISCOFINS,
		IncidenciaPISCOFINS: snapshot.IncidenciaPISCOFINS, IAT: snapshot.IAT, IPPT: snapshot.IPPT,
		IndCFOPVendaDentro:            snapshot.IndCFOPVendaDentro,
		CodigoClassificacaoTributaria: snapshot.CodigoClassificacaoTributaria,
		CSTIBS:                        snapshot.CSTIBS, CSTCBS: snapshot.CSTCBS,
		AliquotaIBS: snapshot.AliquotaIBS, AliquotaCBS: snapshot.AliquotaCBS,
		AliquotaEfetivaIBS:   snapshot.AliquotaEfetivaIBS,
		AliquotaEfetivaCBS:   snapshot.AliquotaEfetivaCBS,
		ObsReformaTributaria: snapshot.ObsReformaTributaria,
		PrecoVenda:           snapshot.PrecoVenda,
		PrecoVenda1:          snapshot.PrecoVenda1, PrecoVenda2: snapshot.PrecoVenda2,
		PrecoVenda3: snapshot.PrecoVenda3, PrecoVenda4: snapshot.PrecoVenda4,
		PrecoVenda5: snapshot.PrecoVenda5, PrecoPromocao: snapshot.PrecoPromocao,
		MargemDesconto: snapshot.MargemDesconto, Aliquota: snapshot.Aliquota,
		QtdeEmbalagem:      snapshot.QtdeEmbalagem,
		DescontoPrecoVenda: snapshot.DescontoPrecoVenda,
		FarmaciaPMC:        snapshot.FarmaciaPMC, DataPromocao: snapshot.DataPromocao,
		FimPromocao: snapshot.FimPromocao, UsaTabelaPreco: snapshot.UsaTabelaPreco,
		PMargem1: snapshot.PMargem1,
		PMargem2: snapshot.PMargem2, PMargem3: snapshot.PMargem3,
		PMargem4: snapshot.PMargem4, PMargem5: snapshot.PMargem5,
		PrecoCustoAnterior: snapshot.PrecoCustoAnterior, PrecoVendaAnterior: snapshot.PrecoVendaAnterior,
		PrecoFarmaPop: snapshot.PrecoFarmaPop, AliquotaPIS: snapshot.AliquotaPIS,
		AliquotaCOFINS: snapshot.AliquotaCOFINS, PFCP: snapshot.PFCP, PFCPST: snapshot.PFCPST,
		Active: snapshot.Active, TracksSerial: snapshot.TracksSerial, UsaGrade: snapshot.UsaGrade,
		Barcodes: make([]agentProductBarcode, 0, len(snapshot.Barcodes)),
	}
	if snapshot.Group != nil {
		value.Group = &agentProductCategory{LocalCode: snapshot.Group.LocalCode, Name: snapshot.Group.Name}
	}
	if snapshot.Subgroup != nil {
		value.Subgroup = &agentProductCategory{LocalCode: snapshot.Subgroup.LocalCode, Name: snapshot.Subgroup.Name}
	}
	for _, barcode := range snapshot.Barcodes {
		value.Barcodes = append(value.Barcodes, agentProductBarcode{
			Value: barcode.Value, Primary: barcode.Primary, Active: barcode.Active, Fraction: barcode.Fraction, C000055: barcode.C000055,
		})
	}
	return value
}

func canonicalizeProductSnapshot(r *repositories.TenantRepository, storeID uuid.UUID, raw models.JSON) (models.JSON, error) {
	snapshot, err := decodeProductSnapshot(raw)
	if err != nil {
		return nil, models.Error(400, "Produto deve seguir o contrato StoreLink: "+err.Error())
	}
	if snapshot.Active == nil || snapshot.TracksSerial == nil {
		return nil, models.Error(400, "Produto requer active e tracksSerial como valores booleanos")
	}
	snapshot.LocalCode = strings.TrimSpace(snapshot.LocalCode)
	snapshot.Name = strings.TrimSpace(snapshot.Name)
	snapshot.SKU = strings.TrimSpace(snapshot.SKU)
	snapshot.Unit = strings.TrimSpace(snapshot.Unit)
	if snapshot.LocalCode == "" || len(snapshot.LocalCode) > 100 || snapshot.Name == "" || len(snapshot.Name) > 255 || len(snapshot.Description) > 1000 || len(snapshot.Unit) > 20 || len(snapshot.SKU) > 100 || len(snapshot.Brand) > 100 || len(snapshot.NCM) > 20 || len(snapshot.CEST) > 20 {
		return nil, models.Error(400, "Produto contém campos obrigatórios vazios ou fora do limite")
	}
	if len(snapshot.Barcodes) == 0 {
		snapshot.Barcodes = []models.ProductBarcodeValue{}
	}
	seen := map[string]struct{}{}
	primaryCount := 0
	for index := range snapshot.Barcodes {
		barcode := &snapshot.Barcodes[index]
		if barcode.Primary == nil || barcode.Active == nil {
			return nil, models.Error(400, "Cada código de barras requer isPrimary e active como valores booleanos")
		}
		barcode.Value = strings.TrimSpace(barcode.Value)
		if barcode.Value == "" || len(barcode.Value) > 32 {
			return nil, models.Error(400, "Código de barras deve conter de 1 a 32 caracteres")
		}
		normalized := normalizeCatalogKey(barcode.Value)
		if _, exists := seen[normalized]; exists {
			return nil, models.Error(400, "Produto contém códigos de barras duplicados")
		}
		seen[normalized] = struct{}{}
		if *barcode.Primary {
			primaryCount++
		}
		if barcode.Fraction != nil && (*barcode.Fraction < 0 || *barcode.Fraction > 1000000) {
			return nil, models.Error(400, "Fração de código de barras fora do intervalo permitido")
		}
	}
	if primaryCount > 1 {
		return nil, models.Error(400, "Produto pode ter no máximo um código de barras principal")
	}
	if snapshot.ProductID != "" {
		id, err := uuid.Parse(snapshot.ProductID)
		if err != nil || id == uuid.Nil {
			return nil, models.Error(400, "productId deve ser UUID válido")
		}
		snapshot.ProductID = id.String()
	}
	if snapshot.Group != nil {
		snapshot.Group.LocalCode = strings.TrimSpace(snapshot.Group.LocalCode)
		snapshot.Group.Name = strings.TrimSpace(snapshot.Group.Name)
		if len(snapshot.Group.LocalCode) > 100 || len(snapshot.Group.Name) > 100 {
			return nil, models.Error(400, "Grupo de produto excede os limites suportados")
		}
	}
	if snapshot.Subgroup != nil {
		snapshot.Subgroup.LocalCode = strings.TrimSpace(snapshot.Subgroup.LocalCode)
		snapshot.Subgroup.Name = strings.TrimSpace(snapshot.Subgroup.Name)
		if len(snapshot.Subgroup.LocalCode) > 100 || len(snapshot.Subgroup.Name) > 100 {
			return nil, models.Error(400, "Subgrupo de produto excede os limites suportados")
		}
	}

	productID, err := resolveSnapshotProductID(r, storeID, snapshot)
	if err != nil {
		return nil, err
	}
	product := models.Product{
		Tenant: models.NewTenant(r.CompanyID()), SKU: snapshot.SKU,
		EstoqueMinimo: snapshot.EstoqueMinimo, MargemMinima: snapshot.MargemMinima,
		FlagSis: snapshot.FlagSis, Name: snapshot.Name,
		Description: snapshot.Description, Unit: snapshot.Unit, Brand: snapshot.Brand,
		CodigoMarca: snapshot.CodigoMarca,
		NCM:         snapshot.NCM, CEST: snapshot.CEST,
		DataCadastro: snapshot.DataCadastro, Aplicacao: snapshot.Aplicacao,
		Origem:              snapshot.Origem,
		ClassificacaoFiscal: snapshot.ClassificacaoFiscal, ClasseTP: snapshot.ClasseTP,
		CST: snapshot.CST, CodigoMS: snapshot.CodigoMS, PrincipioAtivo: snapshot.PrincipioAtivo,
		NomeLaboratorio: snapshot.NomeLaboratorio, CodigoFornecedor: snapshot.CodigoFornecedor,
		NomeFornecedor: snapshot.NomeFornecedor, CNPJFornecedor: snapshot.CNPJFornecedor,
		NomeFantasiaFornecedor: snapshot.NomeFantasiaFornecedor, Tipo: snapshot.Tipo,
		FarmaciaControlado:          snapshot.FarmaciaControlado,
		FarmaciaApresentacao:        snapshot.FarmaciaApresentacao,
		FarmaciaRegistroMedicamento: snapshot.FarmaciaRegistroMedicamento,
		Apresentacao:                snapshot.Apresentacao, CodigoOriginal: snapshot.CodigoOriginal,
		CSOSN: snapshot.CSOSN, UsaLote: snapshot.UsaLote, UsaBalanca: snapshot.UsaBalanca,
		CodigoReceita: snapshot.CodigoReceita, CodigoBarraNovartis: snapshot.CodigoBarraNovartis,
		SituacaoTributaria: snapshot.SituacaoTributaria, PISCOFINS: snapshot.PISCOFINS,
		IncidenciaPISCOFINS: snapshot.IncidenciaPISCOFINS, IAT: snapshot.IAT, IPPT: snapshot.IPPT,
		IndCFOPVendaDentro:            snapshot.IndCFOPVendaDentro,
		CodigoClassificacaoTributaria: snapshot.CodigoClassificacaoTributaria,
		CSTIBS:                        snapshot.CSTIBS, CSTCBS: snapshot.CSTCBS,
		AliquotaIBS: snapshot.AliquotaIBS, AliquotaCBS: snapshot.AliquotaCBS,
		AliquotaEfetivaIBS:   snapshot.AliquotaEfetivaIBS,
		AliquotaEfetivaCBS:   snapshot.AliquotaEfetivaCBS,
		ObsReformaTributaria: snapshot.ObsReformaTributaria,
		PrecoVenda:           snapshot.PrecoVenda,
		PrecoVenda1:          snapshot.PrecoVenda1, PrecoVenda2: snapshot.PrecoVenda2,
		PrecoVenda3: snapshot.PrecoVenda3, PrecoVenda4: snapshot.PrecoVenda4,
		PrecoVenda5: snapshot.PrecoVenda5, PrecoPromocao: snapshot.PrecoPromocao,
		MargemDesconto: snapshot.MargemDesconto, Aliquota: snapshot.Aliquota,
		QtdeEmbalagem:      snapshot.QtdeEmbalagem,
		DescontoPrecoVenda: snapshot.DescontoPrecoVenda,
		FarmaciaPMC:        snapshot.FarmaciaPMC, DataPromocao: snapshot.DataPromocao,
		FimPromocao: snapshot.FimPromocao, UsaTabelaPreco: snapshot.UsaTabelaPreco,
		PMargem1: snapshot.PMargem1,
		PMargem2: snapshot.PMargem2, PMargem3: snapshot.PMargem3,
		PMargem4: snapshot.PMargem4, PMargem5: snapshot.PMargem5,
		PrecoCustoAnterior: snapshot.PrecoCustoAnterior, PrecoVendaAnterior: snapshot.PrecoVendaAnterior,
		PrecoFarmaPop: snapshot.PrecoFarmaPop, AliquotaPIS: snapshot.AliquotaPIS,
		AliquotaCOFINS: snapshot.AliquotaCOFINS, PFCP: snapshot.PFCP, PFCPST: snapshot.PFCPST,
		Active: *snapshot.Active, TracksSerial: *snapshot.TracksSerial,
		UsaGrade: snapshot.UsaGrade != nil && *snapshot.UsaGrade,
	}
	if snapshot.Group != nil {
		product.GroupName = snapshot.Group.Name
	}
	if snapshot.Subgroup != nil {
		product.SubgroupName = snapshot.Subgroup.Name
	}
	product.ID = productID
	var existing models.Product
	if err := r.Find(&existing, productID, false); err == nil {
		product.CreatedAt = existing.CreatedAt
		if strings.TrimSpace(existing.SKU) != "" {
			// Keep the canonical tenant SKU when the product was resolved by
			// EAN. The store-specific internal code is persisted in the
			// product/store mapping, not in the global product row.
			product.SKU = existing.SKU
		}
		if err := r.Save(&product, productID); err != nil {
			return nil, err
		}
	} else if repositories.IsNotFound(err) {
		if err := r.Create(&product); err != nil {
			return nil, err
		}
	} else {
		return nil, err
	}
	if err := saveProductBarcodes(r, storeID, productID, snapshot.Barcodes); err != nil {
		return nil, err
	}
	groupLocalCode, groupName := "", ""
	if snapshot.Group != nil {
		groupLocalCode, groupName = snapshot.Group.LocalCode, snapshot.Group.Name
	}
	subgroupLocalCode, subgroupName := "", ""
	if snapshot.Subgroup != nil {
		subgroupLocalCode, subgroupName = snapshot.Subgroup.LocalCode, snapshot.Subgroup.Name
	}
	if mapping, found, err := r.ProductMapping(storeID, snapshot.LocalCode); err != nil {
		return nil, err
	} else if found && mapping.ProductID != productID {
		return nil, models.Error(409, "PRODUCT_LOCAL_CODE_CONFLICT: o código local já aponta para outro produto")
	} else if found {
		changed := mapping.LocalCode != snapshot.LocalCode || mapping.GroupLocalCode != groupLocalCode || mapping.GroupName != groupName || mapping.SubgroupLocalCode != subgroupLocalCode || mapping.SubgroupName != subgroupName
		if changed {
			mapping.LocalCode = snapshot.LocalCode
			mapping.GroupLocalCode = groupLocalCode
			mapping.GroupName = groupName
			mapping.SubgroupLocalCode = subgroupLocalCode
			mapping.SubgroupName = subgroupName
			if err := r.Save(&mapping, mapping.ID); err != nil {
				return nil, err
			}
		}
	} else if mapping, found, err := r.ProductMappingForProduct(storeID, productID); err != nil {
		return nil, err
	} else if found {
		mapping.LocalCode = snapshot.LocalCode
		mapping.GroupLocalCode = groupLocalCode
		mapping.GroupName = groupName
		mapping.SubgroupLocalCode = subgroupLocalCode
		mapping.SubgroupName = subgroupName
		if err := r.Save(&mapping, mapping.ID); err != nil {
			return nil, err
		}
	} else {
		mapping := models.ProductStoreMapping{
			Tenant: models.NewTenant(r.CompanyID()), StoreID: storeID, ProductID: productID, LocalCode: snapshot.LocalCode,
			GroupLocalCode: groupLocalCode, GroupName: groupName, SubgroupLocalCode: subgroupLocalCode, SubgroupName: subgroupName,
		}
		if err := r.Create(&mapping); err != nil {
			return nil, err
		}
	}

	snapshot.ProductID = productID.String()
	var canonical []byte
	if isLegacyAgentProductSnapshot(raw) {
		canonical, err = json.Marshal(toAgentProductSnapshot(snapshot))
	} else {
		canonical, err = json.Marshal(snapshot)
	}
	if err != nil {
		return nil, fmt.Errorf("encode canonical product: %w", err)
	}
	return models.JSON(canonical), nil
}

func resolveSnapshotProductID(r *repositories.TenantRepository, storeID uuid.UUID, snapshot models.ProductSnapshot) (uuid.UUID, error) {
	// An active EAN/barcode is the strongest identity available from FNTS.
	// Local code, SKU and an old ProductID can legitimately differ between
	// stores; they must not make an otherwise unambiguous EAN fail.
	var barcodeOwner uuid.UUID
	barcodeFound := false
	for _, barcode := range snapshot.Barcodes {
		if barcode.Active == nil || !*barcode.Active {
			continue
		}
		if value, found, err := r.ProductBarcode(normalizeCatalogKey(barcode.Value)); err != nil {
			return uuid.Nil, err
		} else if found {
			if barcodeFound && barcodeOwner != value.ProductID {
				return uuid.Nil, models.Error(409, "PRODUCT_IDENTITY_CONFLICT: codigos de barras ativos apontam para produtos diferentes")
			}
			barcodeOwner = value.ProductID
			barcodeFound = true
		}
	}
	if barcodeFound {
		return barcodeOwner, nil
	}
	// An active EAN that is not known yet is still a new identity. Do not
	// silently merge it with a stale local code or SKU: two stores may reuse
	// the same internal code for different products. An explicit canonical
	// ProductID is safe to reuse because it is the identity assigned by
	// StoreLink; otherwise create a new identity and let the caller reconcile
	// its canonical code.
	if hasActiveProductBarcode(snapshot) {
		if strings.TrimSpace(snapshot.ProductID) != "" {
			id, parseErr := uuid.Parse(strings.TrimSpace(snapshot.ProductID))
			if parseErr != nil || id == uuid.Nil {
				return uuid.Nil, models.Error(400, "productId deve ser UUID valido")
			}
			var product models.Product
			if findErr := r.Find(&product, id, false); findErr == nil {
				return product.ID, nil
			} else if !repositories.IsNotFound(findErr) {
				return uuid.Nil, findErr
			}
			foreignIDs, lookupErr := r.ProductIDsOwnedByAnotherCompany([]uuid.UUID{id})
			if lookupErr != nil {
				return uuid.Nil, lookupErr
			}
			if len(foreignIDs) > 0 {
				return uuid.Nil, models.Error(409, "PRODUCT_TENANT_CONFLICT: productId pertence a outra empresa")
			}
			return id, nil
		}
		return uuid.New(), nil
	}

	var selected uuid.UUID
	selectCandidate := func(id uuid.UUID) error {
		if selected != uuid.Nil && selected != id {
			return models.Error(409, "PRODUCT_IDENTITY_CONFLICT: SKU e códigos de barras apontam para produtos diferentes")
		}
		selected = id
		return nil
	}
	if mapping, found, err := r.ProductMapping(storeID, snapshot.LocalCode); err != nil {
		return uuid.Nil, err
	} else if found {
		if err := selectCandidate(mapping.ProductID); err != nil {
			return uuid.Nil, err
		}
	}
	if snapshot.ProductID != "" {
		id, _ := uuid.Parse(snapshot.ProductID)
		var product models.Product
		if err := r.Find(&product, id, false); err == nil {
			if err := selectCandidate(product.ID); err != nil {
				return uuid.Nil, err
			}
		} else if !repositories.IsNotFound(err) {
			return uuid.Nil, err
		} else {
			foreignIDs, lookupErr := r.ProductIDsOwnedByAnotherCompany([]uuid.UUID{id})
			if lookupErr != nil {
				return uuid.Nil, lookupErr
			}
			if len(foreignIDs) > 0 {
				return uuid.Nil, models.Error(409, "PRODUCT_TENANT_CONFLICT: productId pertence a outra empresa")
			}
			if selected != uuid.Nil && selected != id {
				return uuid.Nil, models.Error(409, "PRODUCT_IDENTITY_CONFLICT: productId diverge do mapeamento já registrado")
			}
			selected = id
		}
	}
	if snapshot.SKU != "" {
		if product, found, err := r.ProductBySKU(normalizeCatalogKey(snapshot.SKU)); err != nil {
			return uuid.Nil, err
		} else if found {
			if err := selectCandidate(product.ID); err != nil {
				return uuid.Nil, err
			}
		}
	}
	for _, barcode := range snapshot.Barcodes {
		if barcode.Active == nil || !*barcode.Active {
			continue
		}
		if productBarcode, found, err := r.ProductBarcode(normalizeCatalogKey(barcode.Value)); err != nil {
			return uuid.Nil, err
		} else if found {
			if err := selectCandidate(productBarcode.ProductID); err != nil {
				return uuid.Nil, err
			}
		}
	}
	if selected == uuid.Nil {
		if snapshot.ProductID != "" {
			selected, _ = uuid.Parse(snapshot.ProductID)
		} else {
			selected = uuid.New()
		}
	}
	return selected, nil
}

func saveProductBarcodes(r *repositories.TenantRepository, storeID, productID uuid.UUID, barcodes []models.ProductBarcodeValue) error {
	existingBarcodes, err := r.ProductBarcodesForProducts([]uuid.UUID{productID})
	if err != nil {
		return err
	}
	currentPrimary := ""
	for _, existing := range existingBarcodes {
		if existing.Active && existing.Primary {
			currentPrimary = normalizeCatalogKey(existing.NormalizedValue)
			break
		}
	}
	sourceIsMatrix, err := storeIsMatrix(r, storeID)
	if err != nil {
		return err
	}
	incomingPrimaryValue := ""
	authoritativeC000055 := false
	keepC000055 := make([]string, 0, len(barcodes))
	for _, incoming := range barcodes {
		if incoming.C000055 != nil {
			authoritativeC000055 = true
		}
		if incoming.Active != nil && *incoming.Active && incoming.Primary != nil && *incoming.Primary {
			incomingPrimaryValue = normalizeCatalogKey(incoming.Value)
		}
	}
	for _, incoming := range barcodes {
		value := strings.TrimSpace(incoming.Value)
		normalized := normalizeCatalogKey(value)
		current, found, err := r.ProductBarcode(normalized)
		if err != nil {
			return err
		}
		if found && current.ProductID != productID {
			return models.Error(409, "BARCODE_ALREADY_ASSIGNED: código de barras ativo pertence a outro produto")
		}
		if !found {
			current, found, err = r.ProductBarcodeForProduct(normalized, productID)
			if err != nil {
				return err
			}
		}
		effectivePrimary := effectiveProductBarcodePrimary(
			currentPrimary, normalized, incomingPrimaryValue,
			incoming.Active != nil && *incoming.Active,
			incoming.Primary != nil && *incoming.Primary, sourceIsMatrix,
		)
		effectiveC000055 := incoming.C000055 != nil && *incoming.C000055
		if incoming.C000055 == nil {
			// Legacy payloads represented every non-primary barcode as C000055.
			effectiveC000055 = incoming.Primary == nil || !*incoming.Primary
		}
		effectiveC000055 = effectiveC000055 && incoming.Active != nil && *incoming.Active
		if effectiveC000055 {
			keepC000055 = append(keepC000055, normalized)
		}
		if effectivePrimary {
			if err := r.Update(&models.ProductBarcode{}, map[string]interface{}{"id_produto": productID, "principal": true}, map[string]interface{}{"principal": false}); err != nil {
				return err
			}
			currentPrimary = normalized
		}
		if found {
			current.Value = value
			current.NormalizedValue = normalized
			current.Primary = effectivePrimary
			current.Active = *incoming.Active
			current.Fraction = incoming.Fraction
			current.C000055 = effectiveC000055
			if err := r.Save(&current, current.ID); err != nil {
				return err
			}
			continue
		}
		barcode := models.ProductBarcode{Tenant: models.NewTenant(r.CompanyID()), ProductID: productID, Value: value, NormalizedValue: normalized, Primary: effectivePrimary, Active: *incoming.Active, Fraction: incoming.Fraction, C000055: effectiveC000055}
		if err := r.Create(&barcode); err != nil {
			return err
		}
	}
	if authoritativeC000055 {
		if err := r.DeleteProductC00055Except(productID, keepC000055); err != nil {
			return err
		}
	}
	return nil
}

func normalizeCatalogKey(value string) string { return strings.ToUpper(strings.TrimSpace(value)) }
