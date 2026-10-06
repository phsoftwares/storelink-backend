package models

import (
	"time"

	"github.com/google/uuid"
)

// ProductSnapshot is the versioned business contract accepted from an Agent.
// LocalCode belongs to one store; ProductID is the canonical StoreLink id.
type ProductSnapshot struct {
	ProductID                     string                `json:"id_produto,omitempty"`
	LocalCode                     string                `json:"codigo_local" binding:"required,max=100"`
	SKU                           string                `json:"sku,omitempty"`
	CamposNulos                   []string              `json:"campos_nulos,omitempty"`
	EstoqueMinimo                 float64               `json:"estoque_minimo,omitempty"`
	MargemMinima                  float64               `json:"margem_minima,omitempty"`
	FlagSis                       string                `json:"flag_sis,omitempty"`
	Name                          string                `json:"nome" binding:"required,max=255"`
	Description                   string                `json:"descricao,omitempty"`
	Unit                          string                `json:"unidade"`
	Brand                         string                `json:"marca,omitempty"`
	CodigoMarca                   string                `json:"codigo_marca,omitempty"`
	DataCadastro                  *time.Time            `json:"data_cadastro,omitempty"`
	Aplicacao                     string                `json:"aplicacao,omitempty"`
	Origem                        string                `json:"origem,omitempty"`
	NCM                           string                `json:"ncm,omitempty"`
	CEST                          string                `json:"cest,omitempty"`
	ClassificacaoFiscal           string                `json:"classificacao_fiscal,omitempty"`
	ClasseTP                      string                `json:"classe_tp,omitempty"`
	CST                           string                `json:"cst,omitempty"`
	CodigoMS                      string                `json:"codigo_ms,omitempty"`
	PrincipioAtivo                string                `json:"principio_ativo,omitempty"`
	NomeLaboratorio               string                `json:"nome_laboratorio,omitempty"`
	CodigoFornecedor              string                `json:"codigo_fornecedor,omitempty"`
	NomeFornecedor                string                `json:"nome_fornecedor,omitempty"`
	CNPJFornecedor                string                `json:"cnpj_fornecedor,omitempty"`
	NomeFantasiaFornecedor        string                `json:"nome_fantasia_fornecedor,omitempty"`
	Tipo                          string                `json:"tipo,omitempty"`
	FarmaciaControlado            string                `json:"farmacia_controlado,omitempty"`
	FarmaciaApresentacao          int64                 `json:"farmacia_apresentacao"`
	FarmaciaRegistroMedicamento   string                `json:"farmacia_registro_medicamento,omitempty"`
	Apresentacao                  string                `json:"apresentacao,omitempty"`
	CodigoOriginal                string                `json:"codigo_original,omitempty"`
	CSOSN                         string                `json:"csosn,omitempty"`
	UsaLote                       string                `json:"usa_lote,omitempty"`
	UsaBalanca                    string                `json:"usa_balanca,omitempty"`
	CodigoReceita                 string                `json:"codigo_receita,omitempty"`
	CodigoBarraNovartis           string                `json:"codigo_barra_novartis,omitempty"`
	SituacaoTributaria            string                `json:"situacao_tributaria,omitempty"`
	PISCOFINS                     string                `json:"piscofins,omitempty"`
	IncidenciaPISCOFINS           string                `json:"incidencia_piscofins,omitempty"`
	IAT                           string                `json:"iat,omitempty"`
	IPPT                          string                `json:"ippt,omitempty"`
	IndCFOPVendaDentro            string                `json:"ind_cfop_venda_dentro,omitempty"`
	CodigoClassificacaoTributaria string                `json:"codigo_classificacao_tributaria,omitempty"`
	CSTIBS                        string                `json:"cst_ibs,omitempty"`
	CSTCBS                        string                `json:"cst_cbs,omitempty"`
	AliquotaIBS                   float64               `json:"aliquota_ibs,omitempty"`
	AliquotaCBS                   float64               `json:"aliquota_cbs,omitempty"`
	AliquotaEfetivaIBS            float64               `json:"aliquota_efetiva_ibs,omitempty"`
	AliquotaEfetivaCBS            float64               `json:"aliquota_efetiva_cbs,omitempty"`
	ObsReformaTributaria          string                `json:"obs_reforma_tributaria,omitempty"`
	PrecoVenda                    float64               `json:"preco_venda,omitempty"`
	PrecoVenda1                   float64               `json:"preco_venda1,omitempty"`
	PrecoVenda2                   float64               `json:"preco_venda2,omitempty"`
	PrecoVenda3                   float64               `json:"preco_venda3,omitempty"`
	PrecoVenda4                   float64               `json:"preco_venda4,omitempty"`
	PrecoVenda5                   float64               `json:"preco_venda5,omitempty"`
	PrecoPromocao                 float64               `json:"preco_promocao,omitempty"`
	MargemDesconto                float64               `json:"margem_desconto,omitempty"`
	Aliquota                      float64               `json:"aliquota,omitempty"`
	QtdeEmbalagem                 float64               `json:"qtde_embalagem,omitempty"`
	DescontoPrecoVenda            float64               `json:"desconto_preco_venda,omitempty"`
	FarmaciaPMC                   float64               `json:"farmacia_pmc,omitempty"`
	DataPromocao                  *time.Time            `json:"data_promocao,omitempty"`
	FimPromocao                   *time.Time            `json:"fim_promocao,omitempty"`
	UsaTabelaPreco                string                `json:"usa_tabela_preco,omitempty"`
	PMargem1                      float64               `json:"pmargem1,omitempty"`
	PMargem2                      float64               `json:"pmargem2,omitempty"`
	PMargem3                      float64               `json:"pmargem3,omitempty"`
	PMargem4                      float64               `json:"pmargem4,omitempty"`
	PMargem5                      float64               `json:"pmargem5,omitempty"`
	PrecoCustoAnterior            float64               `json:"preco_custo_anterior,omitempty"`
	PrecoVendaAnterior            float64               `json:"preco_venda_anterior,omitempty"`
	PrecoFarmaPop                 float64               `json:"preco_farma_pop,omitempty"`
	AliquotaPIS                   float64               `json:"aliquota_pis,omitempty"`
	AliquotaCOFINS                float64               `json:"aliquota_cofins,omitempty"`
	PFCP                          float64               `json:"pfcp,omitempty"`
	PFCPST                        float64               `json:"pfcpst,omitempty"`
	Active                        *bool                 `json:"ativo" binding:"required"`
	TracksSerial                  *bool                 `json:"usa_serial" binding:"required"`
	UsaGrade                      *bool                 `json:"usa_grade,omitempty"`
	Group                         *ProductCategoryValue `json:"grupo,omitempty"`
	Subgroup                      *ProductCategoryValue `json:"subgrupo,omitempty"`
	Barcodes                      []ProductBarcodeValue `json:"codigos_barras"`
}

type ProductBatchDTO struct {
	Items []ProductSnapshot `json:"itens" binding:"required,min=1,max=1000,dive"`
}

type ProductCategoryValue struct {
	LocalCode string `json:"codigo_local,omitempty"`
	Name      string `json:"nome" binding:"required,max=100"`
}

type ProductBarcodeValue struct {
	Value    string   `json:"valor" binding:"required,max=32"`
	Primary  *bool    `json:"principal" binding:"required"`
	Active   *bool    `json:"ativo" binding:"required"`
	Fraction *float64 `json:"fracao,omitempty"`
	C000055  *bool    `json:"c00055,omitempty"`
}

type Product struct {
	Tenant
	SKU                           string     `json:"sku"`
	EstoqueMinimo                 float64    `json:"estoqueMinimo" gorm:"column:estoque_minimo"`
	MargemMinima                  float64    `json:"margemMinima" gorm:"column:margem_minima"`
	FlagSis                       string     `json:"flagSis" gorm:"column:flag_sis"`
	Name                          string     `json:"name" gorm:"column:nome"`
	Description                   string     `json:"description" gorm:"column:descricao"`
	Unit                          string     `json:"unit" gorm:"column:unidade"`
	Brand                         string     `json:"brand" gorm:"column:marca"`
	CodigoMarca                   string     `json:"codigoMarca" gorm:"column:codigo_marca"`
	DataCadastro                  *time.Time `json:"dataCadastro" gorm:"column:data_cadastro"`
	Aplicacao                     string     `json:"aplicacao" gorm:"column:aplicacao"`
	Origem                        string     `json:"origem" gorm:"column:origem"`
	NCM                           string     `json:"ncm"`
	CEST                          string     `json:"cest"`
	ClassificacaoFiscal           string     `json:"classificacaoFiscal" gorm:"column:classificacao_fiscal"`
	ClasseTP                      string     `json:"classeTp" gorm:"column:classe_tp"`
	CST                           string     `json:"cst"`
	CodigoMS                      string     `json:"codigoMs" gorm:"column:codigo_ms"`
	PrincipioAtivo                string     `json:"principioAtivo" gorm:"column:principio_ativo"`
	NomeLaboratorio               string     `json:"nomeLaboratorio" gorm:"column:nome_laboratorio"`
	CodigoFornecedor              string     `json:"codigoFornecedor" gorm:"column:codigo_fornecedor"`
	NomeFornecedor                string     `json:"nomeFornecedor" gorm:"column:nome_fornecedor"`
	CNPJFornecedor                string     `json:"cnpjFornecedor" gorm:"column:cnpj_fornecedor"`
	NomeFantasiaFornecedor        string     `json:"nomeFantasiaFornecedor" gorm:"column:nome_fantasia_fornecedor"`
	Tipo                          string     `json:"tipo"`
	FarmaciaControlado            string     `json:"farmaciaControlado" gorm:"column:farmacia_controlado"`
	FarmaciaApresentacao          int64      `json:"farmaciaApresentacao" gorm:"column:farmacia_apresentacao"`
	FarmaciaRegistroMedicamento   string     `json:"farmaciaRegistroMedicamento" gorm:"column:farmacia_registro_medicamento"`
	Apresentacao                  string     `json:"apresentacao"`
	CodigoOriginal                string     `json:"codigoOriginal" gorm:"column:codigo_original"`
	CSOSN                         string     `json:"csosn"`
	UsaLote                       string     `json:"usaLote" gorm:"column:usa_lote"`
	UsaBalanca                    string     `json:"usaBalanca" gorm:"column:usa_balanca"`
	CodigoReceita                 string     `json:"codigoReceita" gorm:"column:codigo_receita"`
	CodigoBarraNovartis           string     `json:"codigoBarraNovartis" gorm:"column:codigo_barra_novartis"`
	SituacaoTributaria            string     `json:"situacaoTributaria" gorm:"column:situacao_tributaria"`
	PISCOFINS                     string     `json:"pisCofins" gorm:"column:piscofins"`
	IncidenciaPISCOFINS           string     `json:"incidenciaPisCofins" gorm:"column:incidencia_piscofins"`
	IAT                           string     `json:"iat"`
	IPPT                          string     `json:"ippt" gorm:"column:ippt"`
	IndCFOPVendaDentro            string     `json:"indCfopVendaDentro" gorm:"column:ind_cfop_venda_dentro"`
	CodigoClassificacaoTributaria string     `json:"codigoClassificacaoTributaria" gorm:"column:codigo_classificacao_tributaria"`
	CSTIBS                        string     `json:"cstIbs" gorm:"column:cst_ibs"`
	CSTCBS                        string     `json:"cstCbs" gorm:"column:cst_cbs"`
	AliquotaIBS                   float64    `json:"aliquotaIbs" gorm:"column:aliquota_ibs"`
	AliquotaCBS                   float64    `json:"aliquotaCbs" gorm:"column:aliquota_cbs"`
	AliquotaEfetivaIBS            float64    `json:"aliquotaEfetivaIbs" gorm:"column:aliquota_efetiva_ibs"`
	AliquotaEfetivaCBS            float64    `json:"aliquotaEfetivaCbs" gorm:"column:aliquota_efetiva_cbs"`
	ObsReformaTributaria          string     `json:"obsReformaTributaria" gorm:"column:obs_reforma_tributaria"`
	PrecoVenda                    float64    `json:"precoVenda" gorm:"column:preco_venda"`
	PrecoVenda1                   float64    `json:"precoVenda1" gorm:"column:preco_venda1"`
	PrecoVenda2                   float64    `json:"precoVenda2" gorm:"column:preco_venda2"`
	PrecoVenda3                   float64    `json:"precoVenda3" gorm:"column:preco_venda3"`
	PrecoVenda4                   float64    `json:"precoVenda4" gorm:"column:preco_venda4"`
	PrecoVenda5                   float64    `json:"precoVenda5" gorm:"column:preco_venda5"`
	PrecoPromocao                 float64    `json:"precoPromocao" gorm:"column:preco_promocao"`
	MargemDesconto                float64    `json:"margemDesconto" gorm:"column:margem_desconto"`
	Aliquota                      float64    `json:"aliquota" gorm:"column:aliquota"`
	QtdeEmbalagem                 float64    `json:"qtdeEmbalagem" gorm:"column:qtde_embalagem"`
	DescontoPrecoVenda            float64    `json:"descontoPrecoVenda" gorm:"column:desconto_preco_venda"`
	FarmaciaPMC                   float64    `json:"farmaciaPmc" gorm:"column:farmacia_pmc"`
	DataPromocao                  *time.Time `json:"dataPromocao" gorm:"column:data_promocao"`
	FimPromocao                   *time.Time `json:"fimPromocao" gorm:"column:fim_promocao"`
	UsaTabelaPreco                string     `json:"usaTabelaPreco" gorm:"column:usa_tabela_preco"`
	PMargem1                      float64    `json:"pmargem1" gorm:"column:pmargem1"`
	PMargem2                      float64    `json:"pmargem2" gorm:"column:pmargem2"`
	PMargem3                      float64    `json:"pmargem3" gorm:"column:pmargem3"`
	PMargem4                      float64    `json:"pmargem4" gorm:"column:pmargem4"`
	PMargem5                      float64    `json:"pmargem5" gorm:"column:pmargem5"`
	PrecoCustoAnterior            float64    `json:"precoCustoAnterior" gorm:"column:preco_custo_anterior"`
	PrecoVendaAnterior            float64    `json:"precoVendaAnterior" gorm:"column:preco_venda_anterior"`
	PrecoFarmaPop                 float64    `json:"precoFarmaPop" gorm:"column:preco_farma_pop"`
	AliquotaPIS                   float64    `json:"aliquotaPis" gorm:"column:aliquota_pis"`
	AliquotaCOFINS                float64    `json:"aliquotaCofins" gorm:"column:aliquota_cofins"`
	PFCP                          float64    `json:"pfcp" gorm:"column:pfcp"`
	PFCPST                        float64    `json:"pfcpst" gorm:"column:pfcpst"`
	GroupName                     string     `json:"groupName" gorm:"column:grupo_nome"`
	SubgroupName                  string     `json:"subgroupName" gorm:"column:subgrupo_nome"`
	Active                        bool       `json:"active" gorm:"column:ativo"`
	TracksSerial                  bool       `json:"tracksSerial"`
	UsaGrade                      bool       `json:"usaGrade" gorm:"column:usa_grade"`
}

func (Product) TableName() string { return "produto" }

type ProductBarcode struct {
	Tenant
	ProductID       uuid.UUID `json:"productId" gorm:"column:id_produto"`
	Value           string    `json:"value" gorm:"column:codigo_barras"`
	NormalizedValue string    `json:"-" gorm:"column:codigo_barras_normalizado"`
	Primary         bool      `json:"isPrimary" gorm:"column:principal"`
	Active          bool      `json:"active" gorm:"column:ativo"`
	Fraction        *float64  `json:"fraction,omitempty"`
	C000055         bool      `json:"c00055" gorm:"column:c00055"`
}

func (ProductBarcode) TableName() string { return "produto_codigo_barras" }

type ProductStoreMapping struct {
	Tenant
	StoreID           uuid.UUID `json:"storeId" gorm:"column:id_loja"`
	ProductID         uuid.UUID `json:"productId" gorm:"column:id_produto"`
	LocalCode         string    `json:"localCode" gorm:"column:codigo_local"`
	GroupLocalCode    string    `json:"groupLocalCode" gorm:"column:codigo_grupo_local"`
	GroupName         string    `json:"groupName" gorm:"column:grupo_nome"`
	SubgroupLocalCode string    `json:"subgroupLocalCode" gorm:"column:codigo_subgrupo_local"`
	SubgroupName      string    `json:"subgroupName" gorm:"column:subgrupo_nome"`
}

func (ProductStoreMapping) TableName() string { return "produto_loja_mapeamento" }

type ProductPrice struct {
	Tenant
	StoreID   uuid.UUID  `json:"storeId" gorm:"column:id_loja"`
	ProductID uuid.UUID  `json:"productId" gorm:"column:id_produto"`
	Kind      string     `json:"kind" gorm:"column:tipo"`
	Quantity  float64    `json:"quantity"`
	Amount    float64    `json:"amount"`
	Active    bool       `json:"active" gorm:"column:ativo"`
	ValidFrom *time.Time `json:"validFrom"`
}

func (ProductPrice) TableName() string { return "produto_preco_loja" }

// ProductPriceSnapshot is the explicit FNTS-to-StoreLink price contract.
// A store's local product code plus kind and quantity identify one price tier.
type ProductPriceSnapshot struct {
	LocalCode string     `json:"codigo_local" binding:"required,max=100"`
	Kind      string     `json:"tipo" binding:"required,oneof=COST SALE WHOLESALE"`
	Quantity  float64    `json:"quantidade" binding:"required,gt=0"`
	Amount    *float64   `json:"valor" binding:"required,gte=0"`
	Active    *bool      `json:"ativo" binding:"required"`
	ValidFrom *time.Time `json:"valido_desde,omitempty"`
}

type ProductPriceBatchDTO struct {
	Items []ProductPriceSnapshot `json:"itens" binding:"required,min=1,max=1000,dive"`
}

type ProductPriceBatchResult struct {
	ProcessedCount int                    `json:"quantidade_processada"`
	Items          []ProductPriceSnapshot `json:"itens"`
}

// ProductPublicationDTO is the normalized write contract used by FNTS_Server.
// It is deliberately separate from the direct catalog upsert contract: a
// publication updates the canonical source product and fans out messages to
// the existing mensagem queue. No second/legacy queue is involved.
type ProductPublicationDTO struct {
	ID      *uuid.UUID             `json:"id,omitempty"`
	Event   string                 `json:"evento" binding:"required,oneof=CREATED UPDATED DISABLED"`
	Product ProductSnapshot        `json:"produto" binding:"required"`
	Prices  []ProductPriceSnapshot `json:"precos,omitempty"`
	// PricesAuthoritative tells the receiver that an empty prices array is
	// intentional and must clear the source store's current price tiers.
	// Without this bit, an omitted price snapshot is indistinguishable from a
	// product publication that simply did not carry pricing.
	PricesAuthoritative bool `json:"precos_autoritativos,omitempty"`
	// RequestNewLocalCode is used when a receiving store found that the
	// canonical product code is occupied by another local product. The server
	// moves this product mapping atomically to LocalCodeCandidate before it
	// fans the publication out, instead of rejecting the whole synchronization.
	LocalCodeCandidate  string      `json:"codigo_local_candidato,omitempty"`
	RequestNewLocalCode bool        `json:"solicitar_novo_codigo_local,omitempty"`
	DestinationStoreIDs []uuid.UUID `json:"ids_lojas_destino,omitempty"`
}

type ProductPublicationBatchDTO struct {
	Items []ProductPublicationDTO `json:"itens" binding:"required,min=1,max=1000,dive"`
}

type ProductPublicationResult struct {
	ID                 uuid.UUID `json:"id"`
	ProductID          uuid.UUID `json:"id_produto"`
	LocalCode          string    `json:"codigo_local"`
	Event              string    `json:"evento"`
	Version            int64     `json:"versao"`
	HashPayload        string    `json:"hash_payload"`
	DestinationsQueued int       `json:"destinos_enfileirados"`
}

type ProductPublicationBatchResult struct {
	ProcessedCount int                        `json:"quantidade_processada"`
	Items          []ProductPublicationResult `json:"itens"`
}

type ProductCatalogItem struct {
	Product             ProductSnapshot        `json:"produto"`
	Prices              []ProductPriceSnapshot `json:"precos,omitempty"`
	PricesAuthoritative bool                   `json:"precos_autoritativos,omitempty"`
}

type ProductCatalogPage struct {
	Items    []ProductCatalogItem `json:"itens"`
	Total    int64                `json:"total"`
	Page     int                  `json:"pagina"`
	PageSize int                  `json:"limite"`
	HasMore  bool                 `json:"tem_mais"`
}

// Portable people contracts intentionally omit passwords, payroll, cash
// balances, and session state. Employees are separate from API users.
type CustomerSnapshot struct {
	ID            string  `json:"id,omitempty"`
	LocalCode     string  `json:"codigo_local" binding:"required,max=100"`
	Name          string  `json:"nome" binding:"required,max=255"`
	TaxIdentifier string  `json:"documento,omitempty"`
	Phone         string  `json:"telefone,omitempty"`
	Mobile        string  `json:"celular,omitempty"`
	Email         string  `json:"email,omitempty"`
	Address       Address `json:"endereco"`
	CreditStatus  string  `json:"situacao_credito" binding:"required,oneof=STANDARD UNDER_REVIEW UNCLASSIFIED"`
}

type CustomerBatchDTO struct {
	Items []CustomerSnapshot `json:"itens" binding:"required,min=1,max=1000,dive"`
}

type Address struct {
	Street     string `json:"logradouro,omitempty"`
	Number     string `json:"numero,omitempty"`
	Complement string `json:"complemento,omitempty"`
	District   string `json:"bairro,omitempty"`
	City       string `json:"cidade,omitempty"`
	State      string `json:"uf,omitempty"`
	PostalCode string `json:"cep,omitempty"`
}

type Customer struct {
	Tenant
	Name         string  `json:"name" gorm:"column:nome"`
	Document     string  `json:"document" gorm:"column:documento"`
	Phone        string  `json:"phone" gorm:"column:telefone"`
	Mobile       string  `json:"mobile" gorm:"column:celular"`
	Email        string  `json:"email"`
	Address      Address `json:"address" gorm:"column:endereco;type:jsonb;serializer:json"`
	CreditStatus string  `json:"creditStatus" gorm:"column:situacao_credito"`
}

func (Customer) TableName() string { return "cliente" }

type CustomerStoreMapping struct {
	Tenant
	StoreID    uuid.UUID `json:"storeId" gorm:"column:id_loja"`
	CustomerID uuid.UUID `json:"customerId" gorm:"column:id_cliente"`
	LocalCode  string    `json:"localCode" gorm:"column:codigo_local"`
}

func (CustomerStoreMapping) TableName() string { return "cliente_loja_mapeamento" }

type EmployeeSnapshot struct {
	ID               string                 `json:"id,omitempty"`
	LocalCode        string                 `json:"codigo_local" binding:"required,max=100"`
	Name             string                 `json:"nome" binding:"required,max=255"`
	Document         string                 `json:"documento,omitempty"`
	Email            string                 `json:"email,omitempty"`
	Phone            string                 `json:"telefone,omitempty"`
	Role             string                 `json:"funcao,omitempty"`
	Active           *bool                  `json:"ativo" binding:"required"`
	CashierOperators []CashierOperatorValue `json:"operadores_caixa,omitempty"`
}

type Employee struct {
	Tenant
	Name     string `json:"name" gorm:"column:nome"`
	Document string `json:"document" gorm:"column:documento"`
	Email    string `json:"email"`
	Phone    string `json:"phone" gorm:"column:telefone"`
	Role     string `json:"role" gorm:"column:funcao"`
	Active   bool   `json:"active" gorm:"column:ativo"`
}

func (Employee) TableName() string { return "funcionario" }

type EmployeeStoreMapping struct {
	Tenant
	StoreID    uuid.UUID `json:"storeId" gorm:"column:id_loja"`
	EmployeeID uuid.UUID `json:"employeeId" gorm:"column:id_funcionario"`
	LocalCode  string    `json:"localCode" gorm:"column:codigo_local"`
}

func (EmployeeStoreMapping) TableName() string { return "funcionario_loja_mapeamento" }

type EmployeeBatchDTO struct {
	Items []EmployeeSnapshot `json:"itens" binding:"required,min=1,max=1000,dive"`
}

type CashierOperatorValue struct {
	LocalCode          string `json:"codigo_local" binding:"required,max=100"`
	EmployeeLocalCode  string `json:"codigo_funcionario,omitempty"`
	CanOpenGeneralCash *bool  `json:"pode_abrir_caixa_geral" binding:"required"`
	CanViewAllReports  *bool  `json:"pode_visualizar_todos_relatorios" binding:"required"`
	BlindClosing       *bool  `json:"fechamento_cego" binding:"required"`
}

type CashierOperator struct {
	Tenant
	StoreID            uuid.UUID `json:"storeId" gorm:"column:id_loja"`
	EmployeeID         uuid.UUID `json:"employeeId" gorm:"column:id_funcionario"`
	LocalCode          string    `json:"localCode" gorm:"column:codigo_local"`
	CanOpenGeneralCash bool      `json:"canOpenGeneralCash" gorm:"column:pode_abrir_caixa_geral"`
	CanViewAllReports  bool      `json:"canViewAllReports" gorm:"column:pode_visualizar_todos_relatorios"`
	BlindClosing       bool      `json:"blindClosing" gorm:"column:fechamento_cego"`
}

func (CashierOperator) TableName() string { return "operador_caixa" }

type CashierOperatorSnapshot struct {
	LocalCode          string `json:"codigo_local" binding:"required,max=100"`
	EmployeeLocalCode  string `json:"codigo_funcionario" binding:"required,max=100"`
	CanOpenGeneralCash *bool  `json:"pode_abrir_caixa_geral" binding:"required"`
	CanViewAllReports  *bool  `json:"pode_visualizar_todos_relatorios" binding:"required"`
	BlindClosing       *bool  `json:"fechamento_cego" binding:"required"`
}

type CashierOperatorSnapshotBatchDTO struct {
	Items []CashierOperatorSnapshot `json:"itens" binding:"required,min=1,max=1000,dive"`
}

type CashierOperatorBatchDTO struct {
	Items []CashierOperatorValue `json:"itens" binding:"required,min=1,max=1000,dive"`
}

type CatalogWriteResult struct {
	ID        uuid.UUID `json:"id"`
	LocalCode string    `json:"codigo_local"`
	SKU       string    `json:"sku,omitempty"`
}

type CatalogBatchResult struct {
	ProcessedCount int                  `json:"quantidade_processada"`
	Items          []CatalogWriteResult `json:"itens"`
}
