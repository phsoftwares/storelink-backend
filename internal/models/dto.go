package models

import (
	"github.com/google/uuid"
	"time"
)

type LoginDTO struct {
	Email string `json:"email" binding:"required,email,max=100"`
	Senha string `json:"senha" binding:"required,max=72"`
}
type LoginResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiraEm"`
	User      User      `json:"user"`
	Companies []Company `json:"companies"`
}
type CompanyDTO struct {
	Nome  string `json:"nome" binding:"required,max=255"`
	CNPJ  string `json:"cnpj" binding:"required,min=14,max=20"`
	Ativo *bool  `json:"ativo"`
}
type GroupDTO struct {
	Nome                    string `json:"nome" binding:"required,max=255"`
	Ativo                   *bool  `json:"ativo"`
	IntegrarPrecos          *bool  `json:"integrarPrecos"`
	IntegrarProdutos        *bool  `json:"integrarProdutos"`
	IntegrarGruposSubgrupos *bool  `json:"integrarGruposSubgrupos"`
	IntegrarClientes        *bool  `json:"integrarClientes"`
	IntegrarFuncionarios    *bool  `json:"integrarFuncionarios"`
	IntegrarUsuarios        *bool  `json:"integrarUsuarios"`
}
type StoreDTO struct {
	GroupID uuid.UUID `json:"groupId" binding:"required"`
	Codigo  string    `json:"codigo" binding:"required,max=50"`
	Nome    string    `json:"nome" binding:"required,max=255"`
	CNPJ    string    `json:"cnpj" binding:"required,min=14,max=20"`
	Tipo    string    `json:"tipo" binding:"required,oneof=MATRIZ FILIAL"`
	Ativo   *bool     `json:"ativo"`
}
type AgentDTO struct {
	StoreID *uuid.UUID `json:"storeId,omitempty"`
	Nome    string     `json:"nome" binding:"required,max=255"`
}
type AgentProvisionResponse struct {
	Agent  Agent  `json:"agent"`
	APIKey string `json:"apiKey,omitempty"`
}
type AgentUpdateDTO struct {
	Ativo *bool `json:"ativo" binding:"required"`
}
type HeartbeatDTO struct {
	Version      string `json:"version" binding:"required,max=50"`
	Status       string `json:"status" binding:"required,oneof=ONLINE OFFLINE DEGRADED"`
	DatabaseOK   bool   `json:"databaseOk"`
	PendingCount int    `json:"pendingCount" binding:"gte=0"`
	ErrorCount   int    `json:"errorCount" binding:"gte=0"`
	Products     int    `json:"products" binding:"gte=0"`
	Customers    int    `json:"customers" binding:"gte=0"`
	Employees    int    `json:"employees" binding:"gte=0"`
	Suppliers    int    `json:"suppliers" binding:"gte=0"`
}
type MessageDTO struct {
	ID                 uuid.UUID `json:"id" binding:"required"`
	Type               string    `json:"type" binding:"required,oneof=EVENT COMMAND"`
	Operation          string    `json:"operation" binding:"required,max=100"`
	DestinationStoreID uuid.UUID `json:"destinationStoreId" binding:"required"`
	Payload            JSON      `json:"payload" binding:"required" swaggertype:"object"`
	CreatedAt          time.Time `json:"createdAt"`
}

// DomainPublicationDTO is the common contract for non-product master data.
// The payload keeps the local C000xxx snapshot extensible without creating a
// new transport contract for every added column.
type DomainPublicationDTO struct {
	ID                  *uuid.UUID  `json:"id,omitempty"`
	Entity              string      `json:"entidade" binding:"required,max=50"`
	Event               string      `json:"evento" binding:"required,oneof=CREATED UPDATED DISABLED"`
	Payload             JSON        `json:"payload" binding:"required" swaggertype:"object"`
	DestinationStoreIDs []uuid.UUID `json:"ids_lojas_destino,omitempty"`
}

type DomainPublicationResult struct {
	ID                   uuid.UUID `json:"id"`
	CanonicalCode        string    `json:"codigo_canonico,omitempty"`
	RelatedCanonicalCode string    `json:"codigo_relacionado_canonico,omitempty"`
	RelatedID            string    `json:"id_relacionado,omitempty"`
	Version              int64     `json:"versao"`
	HashPayload          string    `json:"hash_payload"`
	DestinationsQueued   int       `json:"destinos_enfileirados"`
}

// DomainPublicationBatchDTO keeps the canonical master-data publication
// contract identical to the single-item endpoint while allowing FNTS_Server
// to publish up to 1000 records in one HTTP request.
type DomainPublicationBatchDTO struct {
	Items []DomainPublicationDTO `json:"itens" binding:"required,min=1,max=1000,dive"`
}

type DomainPublicationBatchResult struct {
	ProcessedCount int                       `json:"quantidade_processada"`
	Items          []DomainPublicationResult `json:"itens"`
}

type IntegrationExceptionDTO struct {
	IdempotencyKey string     `json:"chave_idempotencia,omitempty"`
	StoreCNPJ      string     `json:"cnpj_loja,omitempty"`
	System         string     `json:"sistema" binding:"required,max=100"`
	Form           string     `json:"formulario" binding:"max=255"`
	User           string     `json:"usuario" binding:"max=100"`
	Caption        string     `json:"caption_formulario" binding:"max=255"`
	Version        string     `json:"versao_sistema" binding:"max=50"`
	CompanyName    string     `json:"razao_social" binding:"max=255"`
	Terminal       string     `json:"nome_terminal" binding:"max=100"`
	Message        string     `json:"mensagem" binding:"required,max=2000"`
	OccurredAt     *time.Time `json:"data_hora_ocorrencia,omitempty"`
	TransmittedAt  *time.Time `json:"data_hora_transmissao,omitempty"`
}

type IntegrationExceptionResult struct {
	ID             uuid.UUID `json:"id"`
	IdempotencyKey string    `json:"chave_idempotencia"`
	Duplicate      bool      `json:"duplicado"`
}
type ItemError struct {
	ItemKey   string `json:"itemKey"`
	ErrorCode string `json:"errorCode,omitempty"`
	Error     string `json:"error"`
}

type IntegrationItemError struct {
	ItemKey   string `json:"chave_item"`
	ErrorCode string `json:"codigo_erro,omitempty"`
	Error     string `json:"erro"`
}
type AckDTO struct {
	ClaimToken     uuid.UUID   `json:"claimToken" binding:"required"`
	ProcessedCount int         `json:"processedCount" binding:"gte=0"`
	Errors         []ItemError `json:"errors"`
}
type FailDTO struct {
	ClaimToken uuid.UUID `json:"claimToken" binding:"required"`
	Error      string    `json:"error" binding:"required,max=2000"`
}

// IntegrationAckDTO and IntegrationFailDTO are deliberately separate from
// the Agent protocol DTOs. The FNTS contract is Portuguese and can evolve
// without changing the protocol consumed by other clients.
type IntegrationAckDTO struct {
	ClaimToken     uuid.UUID              `json:"token_confirmacao" binding:"required"`
	ProcessedCount int                    `json:"quantidade_processada" binding:"gte=0"`
	Errors         []IntegrationItemError `json:"erros"`
}

// IntegrationAckItemDTO is the compact claim proof used when a worker
// confirms a catalog page.  A page is acknowledged in one transaction so the
// transport does not need one HTTP request per product.
type IntegrationAckItemDTO struct {
	ID         uuid.UUID `json:"id" binding:"required"`
	ClaimToken uuid.UUID `json:"token_confirmacao" binding:"required"`
}

type IntegrationAckBatchDTO struct {
	Items []IntegrationAckItemDTO `json:"itens" binding:"required,min=1,max=1000"`
}

type IntegrationAckBatchResult struct {
	AcknowledgedCount int `json:"quantidade_confirmada"`
}

type IntegrationFailDTO struct {
	ClaimToken uuid.UUID `json:"token_confirmacao" binding:"required"`
	Error      string    `json:"erro" binding:"required,max=2000"`
}

// IntegrationRenewDTO keeps a long-running FNTS batch from losing its lease
// while the local Firebird transaction is still applying the page.
type IntegrationRenewItem struct {
	ID         uuid.UUID `json:"id" binding:"required"`
	ClaimToken uuid.UUID `json:"token_confirmacao" binding:"required"`
}

type IntegrationRenewDTO struct {
	Items []IntegrationRenewItem `json:"itens" binding:"required,min=1,max=1000"`
}

type IntegrationRenewResult struct {
	Renewed    int       `json:"renovadas"`
	LeaseUntil time.Time `json:"validade_confirmacao"`
}

type JobDTO struct {
	OriginStoreID      uuid.UUID `json:"originStoreId" binding:"required"`
	DestinationStoreID uuid.UUID `json:"destinationStoreId" binding:"required"`
	Entity             string    `json:"entity" binding:"required,oneof=product customer employee supplier"`
	BatchSize          int       `json:"batchSize" binding:"gte=0,lte=1000"`
}
type BatchDTO struct {
	ID          uuid.UUID `json:"id" binding:"required"`
	BatchNumber int       `json:"batchNumber" binding:"gte=1"`
	Items       []JSON    `json:"items" binding:"max=1000" swaggertype:"array,object"`
	TotalItems  int       `json:"totalItems" binding:"gte=0"`
	IsLast      bool      `json:"isLast"`
}
type TransferPayload struct {
	ID              uuid.UUID      `json:"id"`
	Number          string         `json:"numero"`
	OriginCNPJ      string         `json:"cnpj_origem,omitempty"`
	DestinationCNPJ string         `json:"cnpj_destino,omitempty"`
	SenderUsername  string         `json:"usuario_remetente,omitempty"`
	SenderNote      string         `json:"observacao_remetente,omitempty"`
	SentAt          *time.Time     `json:"data_envio,omitempty"`
	Items           []TransferLine `json:"itens"`
}
type TransferLine struct {
	ID                     uuid.UUID  `json:"id" binding:"required"`
	ProductID              *uuid.UUID `json:"id_produto,omitempty"`
	LocalProductCode       string     `json:"codigo_produto,omitempty"`
	SKU                    string     `json:"sku,omitempty"`
	Barcode                string     `json:"codigo_barras"`
	Name                   string     `json:"nome" binding:"required,max=255"`
	Unit                   string     `json:"unidade,omitempty"`
	Quantity               float64    `json:"quantidade" binding:"required,gt=0"`
	SalePrice              float64    `json:"preco_venda"`
	CostPrice              float64    `json:"preco_custo"`
	BatchCode              string     `json:"codigo_lote,omitempty"`
	LotSerial              string     `json:"lote_serial,omitempty"`
	Lot                    string     `json:"lote,omitempty"`
	Serial                 string     `json:"serial,omitempty"`
	ExpiresAt              *time.Time `json:"data_validade,omitempty"`
	ManufacturedAt         *time.Time `json:"data_fabricacao,omitempty"`
	Controlled             *bool      `json:"controlado,omitempty"`
	TracksBatch            *bool      `json:"usa_lote,omitempty"`
	TracksSerial           *bool      `json:"usa_serial,omitempty"`
	RegistrationMS         string     `json:"registro_ms,omitempty"`
	TherapeuticClass       string     `json:"classe_terapeutica,omitempty"`
	SNGPCType              string     `json:"tipo_produto_sngpc,omitempty"`
	SNGPCUnit              string     `json:"unidade_sngpc,omitempty"`
	PricesInformed         *bool      `json:"tabela_preco_informada,omitempty"`
	PriceTablePayload      string     `json:"tabela_preco_payload,omitempty"`
	UsesPriceTableInformed *bool      `json:"usa_tb_pc_informada,omitempty"`
	UsesPriceTable         string     `json:"usa_tb_pc,omitempty"`
}

type IntegrationTransferDTO struct {
	ID              uuid.UUID      `json:"id" binding:"required"`
	Number          string         `json:"numero" binding:"required,max=100"`
	DestinationID   uuid.UUID      `json:"id_loja_destino"`
	OriginCNPJ      string         `json:"cnpj_origem,omitempty" binding:"omitempty,max=20"`
	DestinationCNPJ string         `json:"cnpj_destino,omitempty" binding:"omitempty,max=20"`
	SenderUsername  string         `json:"usuario_remetente,omitempty" binding:"omitempty,max=100"`
	SenderNote      string         `json:"observacao_remetente,omitempty" binding:"omitempty,max=2000"`
	SentAt          time.Time      `json:"data_envio" binding:"required"`
	Items           []TransferLine `json:"itens" binding:"required,min=1,max=1000"`
}

type TransferStatusDTO struct {
	Status     string                 `json:"status" binding:"required,oneof=RECEIVED FAILED CANCELLED"`
	ItemErrors []IntegrationItemError `json:"erros_itens,omitempty"`
	Error      string                 `json:"erro,omitempty" binding:"omitempty,max=2000"`
}

type TransferSubmission struct {
	ID        uuid.UUID `json:"id"`
	Number    string    `json:"numero"`
	Status    string    `json:"status"`
	ItemCount int       `json:"quantidade_itens"`
	Duplicate bool      `json:"duplicado"`
}

// These views keep the direct FNTS contract in Portuguese without changing
// the JSON returned by the administration and Agent protocol endpoints.
type IntegrationMessageView struct {
	ID                 uuid.UUID  `json:"id"`
	Type               string     `json:"tipo"`
	Operation          string     `json:"operacao"`
	OriginStoreID      uuid.UUID  `json:"id_loja_origem"`
	DestinationStoreID uuid.UUID  `json:"id_loja_destino"`
	Payload            JSON       `json:"payload" swaggertype:"object"`
	Status             string     `json:"status"`
	Attempts           int        `json:"tentativas"`
	ClaimToken         *uuid.UUID `json:"token_confirmacao,omitempty"`
	LeaseUntil         *time.Time `json:"validade_confirmacao,omitempty"`
	NextAttemptAt      *time.Time `json:"proxima_tentativa,omitempty"`
	LastAttemptAt      *time.Time `json:"ultima_tentativa,omitempty"`
	LastError          string     `json:"ultimo_erro,omitempty"`
	AcknowledgedAt     *time.Time `json:"confirmado_em,omitempty"`
	TransferID         *uuid.UUID `json:"id_transferencia,omitempty"`
}

type IntegrationMessagePage struct {
	Items []IntegrationMessageView `json:"itens"`
}

type IntegrationTransferView struct {
	ID                 uuid.UUID  `json:"id"`
	OriginStoreID      uuid.UUID  `json:"id_loja_origem"`
	DestinationStoreID uuid.UUID  `json:"id_loja_destino"`
	Number             string     `json:"numero"`
	OriginCNPJ         string     `json:"cnpj_origem"`
	DestinationCNPJ    string     `json:"cnpj_destino"`
	SenderUsername     string     `json:"usuario_remetente"`
	SenderNote         string     `json:"observacao_remetente"`
	SentAt             *time.Time `json:"data_envio,omitempty"`
	Status             string     `json:"status"`
	ItemCount          int        `json:"quantidade_itens"`
	SyncedAt           *time.Time `json:"sincronizado_em,omitempty"`
	ReceivedAt         *time.Time `json:"recebido_em,omitempty"`
}

type IntegrationTransferItemView struct {
	ID                     uuid.UUID  `json:"id"`
	TransferID             uuid.UUID  `json:"id_transferencia"`
	ProductID              *uuid.UUID `json:"id_produto,omitempty"`
	SourceProductKey       string     `json:"codigo_produto"`
	Barcode                string     `json:"codigo_barras"`
	SKU                    string     `json:"sku,omitempty"`
	Description            string     `json:"descricao"`
	Unit                   string     `json:"unidade"`
	Quantity               float64    `json:"quantidade"`
	SalePrice              float64    `json:"preco_venda"`
	CostPrice              float64    `json:"preco_custo"`
	BatchCode              string     `json:"codigo_lote,omitempty"`
	LotSerial              string     `json:"lote_serial,omitempty"`
	Lot                    string     `json:"lote,omitempty"`
	Serial                 string     `json:"serial,omitempty"`
	ExpiresAt              *time.Time `json:"data_validade,omitempty"`
	ManufacturedAt         *time.Time `json:"data_fabricacao,omitempty"`
	Controlled             bool       `json:"controlado"`
	TracksBatch            bool       `json:"usa_lote"`
	TracksSerial           bool       `json:"usa_serial"`
	RegistrationMS         string     `json:"registro_ms,omitempty"`
	TherapeuticClass       string     `json:"classe_terapeutica,omitempty"`
	SNGPCType              string     `json:"tipo_produto_sngpc,omitempty"`
	SNGPCUnit              string     `json:"unidade_sngpc,omitempty"`
	PricesInformed         bool       `json:"tabela_preco_informada"`
	PriceTablePayload      string     `json:"tabela_preco_payload,omitempty"`
	UsesPriceTableInformed bool       `json:"usa_tb_pc_informada"`
	UsesPriceTable         string     `json:"usa_tb_pc,omitempty"`
	Status                 string     `json:"status"`
	ErrorCode              string     `json:"codigo_erro,omitempty"`
	ErrorDetail            string     `json:"detalhe_erro,omitempty"`
}

type IntegrationTransferDetail struct {
	Transfer IntegrationTransferView       `json:"transferencia"`
	Items    []IntegrationTransferItemView `json:"itens"`
	Timeline []map[string]interface{}      `json:"linha_do_tempo"`
}

type IntegrationTransferPage struct {
	Items    []IntegrationTransferView `json:"itens"`
	Total    int64                     `json:"total"`
	Page     int                       `json:"pagina"`
	PageSize int                       `json:"limite"`
}
