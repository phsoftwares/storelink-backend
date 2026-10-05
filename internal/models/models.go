package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type JSON json.RawMessage

func (j JSON) Value() (driver.Value, error) {
	if len(j) == 0 {
		return "{}", nil
	}
	return string(j), nil
}
func (j *JSON) Scan(v interface{}) error {
	switch x := v.(type) {
	case []byte:
		*j = append((*j)[:0], x...)
	case string:
		*j = JSON(x)
	case nil:
		*j = JSON("{}")
	default:
		return fmt.Errorf("invalid JSON storage")
	}
	return nil
}
func (j JSON) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("{}"), nil
	}
	return j, nil
}
func (j *JSON) UnmarshalJSON(b []byte) error {
	if !json.Valid(b) {
		return fmt.Errorf("invalid JSON")
	}
	*j = append((*j)[:0], b...)
	return nil
}
func ToJSON(v interface{}) JSON {
	b, err := json.Marshal(v)
	if err != nil {
		return JSON("{}")
	}
	return b
}

type Base struct {
	ID        uuid.UUID `json:"id" gorm:"column:id;type:uuid;primaryKey"`
	CreatedAt time.Time `json:"createdAt" gorm:"column:data_hora_criacao;autoCreateTime"`
	UpdatedAt time.Time `json:"updatedAt" gorm:"column:data_hora_atualizacao;autoUpdateTime"`
}

func NewBase() Base {
	return Base{ID: uuid.New(), CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
}

type Tenant struct {
	Base
	CompanyID uuid.UUID `json:"idEmpresa" gorm:"column:id_empresa;type:uuid"`
}

func NewTenant(id uuid.UUID) Tenant       { return Tenant{Base: NewBase(), CompanyID: id} }
func (t *Tenant) SetCompany(id uuid.UUID) { t.CompanyID = id }

type ScopedEntity interface{ SetCompany(uuid.UUID) }

type User struct {
	Base
	Nome  string `json:"nome"`
	Email string `json:"email"`
	Senha string `json:"-"`
	Ativo bool   `json:"ativo"`
}

func (User) TableName() string { return "usuario" }

type Company struct {
	Base
	Nome  string `json:"nome"`
	CNPJ  string `json:"cnpj" gorm:"column:cnpj"`
	Ativo bool   `json:"ativo"`
}

func (Company) TableName() string { return "empresa" }

type Membership struct {
	Tenant
	UserID uuid.UUID `json:"userId" gorm:"column:id_usuario"`
}

func (Membership) TableName() string { return "usuario_empresa" }

type Group struct {
	Tenant
	Nome                    string `json:"nome"`
	Ativo                   bool   `json:"ativo"`
	IntegrarPrecos          bool   `json:"integrarPrecos" gorm:"column:integrar_precos"`
	IntegrarProdutos        bool   `json:"integrarProdutos" gorm:"column:integrar_produtos"`
	IntegrarGruposSubgrupos bool   `json:"integrarGruposSubgrupos" gorm:"column:integrar_grupos_subgrupos"`
	IntegrarClientes        bool   `json:"integrarClientes" gorm:"column:integrar_clientes"`
	IntegrarFuncionarios    bool   `json:"integrarFuncionarios" gorm:"column:integrar_funcionarios"`
}

func (Group) TableName() string { return "grupo_loja" }

type Store struct {
	Tenant
	GroupID uuid.UUID `json:"groupId" gorm:"column:id_grupo"`
	Codigo  string    `json:"codigo"`
	Nome    string    `json:"nome"`
	CNPJ    string    `json:"cnpj,omitempty" gorm:"column:cnpj"`
	Tipo    string    `json:"tipo"`
	Ativo   bool      `json:"ativo"`
}

func (Store) TableName() string { return "loja" }

type Agent struct {
	Tenant
	StoreID    *uuid.UUID `json:"storeId,omitempty" gorm:"column:id_loja"`
	Nome       string     `json:"nome"`
	APIKeyHash string     `json:"-" gorm:"column:api_key_hash"`
	Ativo      bool       `json:"ativo"`
}

func (Agent) TableName() string { return "agente" }

type Heartbeat struct {
	Tenant
	AgentID      uuid.UUID `json:"agentId" gorm:"column:id_agente"`
	StoreID      uuid.UUID `json:"storeId" gorm:"column:id_loja"`
	Version      string    `json:"version"`
	Status       string    `json:"status"`
	DatabaseOK   bool      `json:"databaseOk"`
	PendingCount int       `json:"pendingCount"`
	ErrorCount   int       `json:"errorCount"`
	Products     int       `json:"products"`
	Customers    int       `json:"customers"`
	Employees    int       `json:"employees"`
	Suppliers    int       `json:"suppliers"`
}

func (Heartbeat) TableName() string { return "heartbeat" }

type Message struct {
	Tenant
	Type               string     `json:"type"`
	Operation          string     `json:"operation"`
	OriginStoreID      uuid.UUID  `json:"originStoreId" gorm:"column:id_loja_origem"`
	DestinationStoreID uuid.UUID  `json:"destinationStoreId" gorm:"column:id_loja_destino"`
	Payload            JSON       `json:"payload" gorm:"type:jsonb" swaggertype:"object"`
	Status             string     `json:"status"`
	Attempts           int        `json:"attempts"`
	ClaimToken         *uuid.UUID `json:"claimToken" gorm:"type:uuid"`
	LeaseUntil         *time.Time `json:"leaseUntil"`
	NextAttemptAt      *time.Time `json:"nextAttemptAt"`
	LastAttemptAt      *time.Time `json:"lastAttemptAt"`
	LastError          string     `json:"lastError"`
	AcknowledgedAt     *time.Time `json:"acknowledgedAt"`
	SyncJobID          *uuid.UUID `json:"syncJobId" gorm:"column:id_sync_job"`
	SyncBatchID        *uuid.UUID `json:"syncBatchId" gorm:"column:id_sync_batch"`
	TransferID         *uuid.UUID `json:"transferId" gorm:"column:id_transferencia"`
}

func (Message) TableName() string { return "mensagem" }

type Attempt struct {
	Tenant
	MessageID  uuid.UUID  `json:"messageId" gorm:"column:id_mensagem"`
	AgentID    uuid.UUID  `json:"agentId" gorm:"column:id_agente"`
	ClaimToken uuid.UUID  `json:"claimToken"`
	Status     string     `json:"status"`
	Error      string     `json:"error"`
	StartedAt  time.Time  `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt"`
}

func (Attempt) TableName() string { return "mensagem_tentativa" }

type IntegrationError struct {
	Tenant
	StoreID     uuid.UUID  `json:"storeId" gorm:"column:id_loja"`
	MessageID   *uuid.UUID `json:"messageId" gorm:"column:id_mensagem"`
	SyncJobID   *uuid.UUID `json:"syncJobId" gorm:"column:id_sync_job"`
	SyncBatchID *uuid.UUID `json:"syncBatchId" gorm:"column:id_sync_batch"`
	Operation   string     `json:"operation"`
	ErrorCode   string     `json:"errorCode" gorm:"column:error_code"`
	Error       string     `json:"error"`
	ItemKey     string     `json:"itemKey"`
	Attempts    int        `json:"attempts"`
	Status      string     `json:"status"`
	ResolvedAt  *time.Time `json:"resolvedAt"`
}

func (IntegrationError) TableName() string { return "integracao_erro" }

type IntegrationException struct {
	Tenant
	StoreID        uuid.UUID  `json:"storeId" gorm:"column:id_loja"`
	IdempotencyKey string     `json:"idempotencyKey" gorm:"column:chave_idempotencia"`
	System         string     `json:"system" gorm:"column:sistema"`
	Form           string     `json:"form" gorm:"column:formulario"`
	User           string     `json:"user" gorm:"column:usuario"`
	Caption        string     `json:"caption" gorm:"column:caption_formulario"`
	Version        string     `json:"version" gorm:"column:versao_sistema"`
	CompanyName    string     `json:"companyName" gorm:"column:razao_social"`
	Terminal       string     `json:"terminal" gorm:"column:nome_terminal"`
	Message        string     `json:"message" gorm:"column:mensagem"`
	OccurredAt     *time.Time `json:"occurredAt" gorm:"column:data_hora_ocorrencia"`
	TransmittedAt  *time.Time `json:"transmittedAt" gorm:"column:data_hora_transmissao"`
	HashPayload    string     `json:"hashPayload" gorm:"column:hash_payload"`
	Status         string     `json:"status"`
}

func (IntegrationException) TableName() string { return "integracao_excecao" }

type SyncJob struct {
	Tenant
	OriginStoreID      uuid.UUID  `json:"originStoreId" gorm:"column:id_loja_origem"`
	DestinationStoreID uuid.UUID  `json:"destinationStoreId" gorm:"column:id_loja_destino"`
	Entity             string     `json:"entity"`
	BatchSize          int        `json:"batchSize"`
	Status             string     `json:"status"`
	TotalItems         int        `json:"totalItems"`
	ReceivedItems      int        `json:"receivedItems"`
	ProcessedItems     int        `json:"processedItems"`
	ErrorItems         int        `json:"errorItems"`
	SourceFinished     bool       `json:"sourceFinished"`
	FinishedAt         *time.Time `json:"finishedAt"`
	Progress           float64    `json:"progress" gorm:"-"`
}

func (SyncJob) TableName() string { return "sync_job" }
func (j *SyncJob) SetProgress() {
	if j.TotalItems > 0 {
		j.Progress = float64(j.ProcessedItems+j.ErrorItems) * 100 / float64(j.TotalItems)
	} else if j.Status == "COMPLETED" {
		j.Progress = 100
	}
}

type SyncBatch struct {
	Tenant
	JobID          uuid.UUID `json:"jobId" gorm:"column:id_sync_job"`
	MessageID      uuid.UUID `json:"messageId" gorm:"column:id_mensagem"`
	BatchNumber    int       `json:"batchNumber"`
	ItemCount      int       `json:"itemCount"`
	ProcessedCount int       `json:"processedCount"`
	ErrorCount     int       `json:"errorCount"`
	Status         string    `json:"status"`
	PayloadHash    string    `json:"-"`
	TotalItems     int       `json:"totalItems"`
	IsLast         bool      `json:"isLast"`
}

func (SyncBatch) TableName() string { return "sync_batch" }

type SyncItem struct {
	Tenant
	JobID     uuid.UUID `json:"jobId" gorm:"column:id_sync_job"`
	BatchID   uuid.UUID `json:"batchId" gorm:"column:id_sync_batch"`
	ItemKey   string    `json:"itemKey"`
	ErrorCode string    `json:"errorCode"`
	Error     string    `json:"error"`
	Status    string    `json:"status"`
}

func (SyncItem) TableName() string { return "sync_item" }

type Transfer struct {
	Tenant
	OriginStoreID      uuid.UUID  `json:"originStoreId" gorm:"column:id_loja_origem"`
	DestinationStoreID uuid.UUID  `json:"destinationStoreId" gorm:"column:id_loja_destino"`
	Numero             string     `json:"numero"`
	OriginCNPJ         string     `json:"originCnpj" gorm:"column:cnpj_origem"`
	DestinationCNPJ    string     `json:"destinationCnpj" gorm:"column:cnpj_destino"`
	SenderUsername     string     `json:"senderUsername" gorm:"column:usuario_remetente"`
	SenderNote         string     `json:"senderNote" gorm:"column:observacao_remetente"`
	SentAt             *time.Time `json:"sentAt" gorm:"column:data_envio"`
	PayloadHash        string     `json:"-" gorm:"column:payload_hash"`
	Status             string     `json:"status"`
	ItemCount          int        `json:"itemCount"`
	SyncedAt           *time.Time `json:"syncedAt"`
	ReceivedAt         *time.Time `json:"receivedAt"`
}

func (Transfer) TableName() string { return "transferencia" }

type TransferItem struct {
	Tenant
	TransferID             uuid.UUID  `json:"transferId" gorm:"column:id_transferencia"`
	ProductID              *uuid.UUID `json:"productId,omitempty" gorm:"column:id_produto"`
	SourceProductKey       string     `json:"localProductCode" gorm:"column:codigo_produto_origem"`
	Barcode                string     `json:"barcode" gorm:"column:codigo_barras"`
	SKU                    string     `json:"sku,omitempty"`
	Description            string     `json:"description" gorm:"column:description"`
	Unit                   string     `json:"unit"`
	Quantity               float64    `json:"quantity"`
	SalePrice              float64    `json:"salePrice" gorm:"column:preco_venda"`
	CostPrice              float64    `json:"costPrice" gorm:"column:preco_custo"`
	BatchCode              string     `json:"batchCode" gorm:"column:codigo_lote"`
	LotSerial              string     `json:"lotSerial" gorm:"column:lote_serial"`
	Lot                    string     `json:"lot"`
	Serial                 string     `json:"serial"`
	ExpiresAt              *time.Time `json:"expiresAt" gorm:"column:data_validade"`
	ManufacturedAt         *time.Time `json:"manufacturedAt" gorm:"column:data_fabricacao"`
	Controlled             bool       `json:"controlled"`
	TracksBatch            bool       `json:"tracksBatch"`
	TracksSerial           bool       `json:"tracksSerial"`
	RegistrationMS         string     `json:"registrationMs" gorm:"column:registro_ms"`
	TherapeuticClass       string     `json:"therapeuticClass" gorm:"column:classe_terapeutica"`
	SNGPCType              string     `json:"sngpcType" gorm:"column:tipo_produto_sngpc"`
	SNGPCUnit              string     `json:"sngpcUnit" gorm:"column:unidade_sngpc"`
	PricesInformed         bool       `json:"pricesInformed" gorm:"column:tabela_preco_informada"`
	PriceTablePayload      string     `json:"priceTablePayload,omitempty" gorm:"column:tabela_preco_payload"`
	UsesPriceTableInformed bool       `json:"usesPriceTableInformed" gorm:"column:usa_tb_pc_informada"`
	UsesPriceTable         string     `json:"usesPriceTable,omitempty" gorm:"column:usa_tb_pc"`
	Status                 string     `json:"status"`
	ErrorCode              string     `json:"errorCode,omitempty"`
	ErrorDetail            string     `json:"errorDetail,omitempty" gorm:"column:erro_detalhe"`
}

func (TransferItem) TableName() string { return "transferencia_item" }

type Audit struct {
	Tenant
	UserID  *uuid.UUID `json:"userId" gorm:"column:id_usuario"`
	StoreID *uuid.UUID `json:"storeId" gorm:"column:id_loja"`
	Action  string     `json:"action"`
	Details JSON       `json:"details" gorm:"type:jsonb" swaggertype:"object"`
}

func (Audit) TableName() string { return "audit_log" }

type Identity struct {
	UserID    uuid.UUID
	CompanyID uuid.UUID
	AgentID   uuid.UUID
	StoreID   uuid.UUID
	GroupID   uuid.UUID
	Nome      string
	Email     string
}

func (i Identity) IsAgent() bool { return i.AgentID != uuid.Nil }

type AppError struct {
	MensagemErro string `json:"mensagemErro"`
	Codigo       int    `json:"codigo"`
}

func (e *AppError) Error() string       { return e.MensagemErro }
func Error(code int, text string) error { return &AppError{MensagemErro: text, Codigo: code} }

type Page struct {
	Items    interface{} `json:"items"`
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"pageSize"`
}
type StoreView struct {
	Store
	Status       string     `json:"status"`
	LastSeen     *time.Time `json:"lastSeen"`
	AgentVersion string     `json:"agentVersion"`
	Products     int        `json:"products"`
	Customers    int        `json:"customers"`
	Employees    int        `json:"employees"`
	Suppliers    int        `json:"suppliers"`
	Pending      int        `json:"pending"`
	Errors       int        `json:"errors"`
}
type Dashboard struct {
	Companies        int64 `json:"companies"`
	Groups           int64 `json:"groups"`
	Stores           int64 `json:"stores"`
	Online           int64 `json:"online"`
	Offline          int64 `json:"offline"`
	Degraded         int64 `json:"degraded"`
	MessagesToday    int64 `json:"messagesToday"`
	PendingMessages  int64 `json:"pendingMessages"`
	FailedMessages   int64 `json:"failedMessages"`
	PendingTransfers int64 `json:"pendingTransfers"`
	FailedTransfers  int64 `json:"failedTransfers"`
	RunningJobs      int64 `json:"runningJobs"`
	CompletedJobs    int64 `json:"completedJobs"`
	FailedJobs       int64 `json:"failedJobs"`
	Products         int64 `json:"products"`
	Customers        int64 `json:"customers"`
	Employees        int64 `json:"employees"`
	Suppliers        int64 `json:"suppliers"`
}
