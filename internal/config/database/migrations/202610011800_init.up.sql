CREATE TABLE empresa (
    id UUID PRIMARY KEY,
    nome VARCHAR(255) NOT NULL,
    cnpj VARCHAR(20) NOT NULL UNIQUE,
    ativo BOOLEAN NOT NULL DEFAULT TRUE,
    data_hora_criacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    data_hora_atualizacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE usuario (
    id UUID PRIMARY KEY,
    nome VARCHAR(255) NOT NULL,
    email VARCHAR(100) NOT NULL UNIQUE,
    senha VARCHAR(255) NOT NULL,
    ativo BOOLEAN NOT NULL DEFAULT TRUE,
    data_hora_criacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    data_hora_atualizacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE usuario_empresa (
    id UUID PRIMARY KEY,
    id_empresa UUID NOT NULL,
    id_usuario UUID NOT NULL REFERENCES usuario(id) ON DELETE CASCADE,
    data_hora_criacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    data_hora_atualizacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_usuario_empresa_empresa FOREIGN KEY (id_empresa) REFERENCES empresa(id) ON DELETE CASCADE,
    CONSTRAINT unq_usuario_empresa UNIQUE (id_empresa, id_usuario),
    CONSTRAINT ux_usuario_empresa_id_tenant UNIQUE (id_empresa, id)
);
CREATE INDEX idx_usuario_empresa_usuario ON usuario_empresa(id_usuario, id_empresa);

CREATE TABLE grupo_loja (
    id UUID PRIMARY KEY,
    id_empresa UUID NOT NULL REFERENCES empresa(id) ON DELETE CASCADE,
    nome VARCHAR(255) NOT NULL,
    ativo BOOLEAN NOT NULL DEFAULT TRUE,
    data_hora_criacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    data_hora_atualizacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT unq_grupo_loja_empresa_nome UNIQUE (id_empresa, nome),
    CONSTRAINT ux_grupo_loja_id_tenant UNIQUE (id_empresa, id)
);
CREATE INDEX idx_grupo_loja_empresa ON grupo_loja(id_empresa, ativo);

CREATE TABLE loja (
    id UUID PRIMARY KEY,
    id_empresa UUID NOT NULL REFERENCES empresa(id) ON DELETE CASCADE,
    id_grupo UUID NOT NULL,
    codigo VARCHAR(50) NOT NULL,
    nome VARCHAR(255) NOT NULL,
    tipo VARCHAR(12) NOT NULL CHECK (tipo IN ('MATRIZ','FILIAL')),
    ativo BOOLEAN NOT NULL DEFAULT TRUE,
    data_hora_criacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    data_hora_atualizacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_loja_grupo FOREIGN KEY (id_empresa, id_grupo) REFERENCES grupo_loja(id_empresa, id) ON DELETE RESTRICT,
    CONSTRAINT unq_loja_grupo_codigo UNIQUE (id_empresa, id_grupo, codigo),
    CONSTRAINT ux_loja_id_tenant UNIQUE (id_empresa, id)
);
CREATE INDEX idx_loja_empresa ON loja(id_empresa, ativo);
CREATE INDEX idx_loja_grupo ON loja(id_empresa, id_grupo, ativo);

CREATE TABLE agente (
    id UUID PRIMARY KEY,
    id_empresa UUID NOT NULL REFERENCES empresa(id) ON DELETE CASCADE,
    id_loja UUID NOT NULL,
    nome VARCHAR(255) NOT NULL,
    secret_hash VARCHAR(255) NOT NULL,
    ativo BOOLEAN NOT NULL DEFAULT TRUE,
    data_hora_criacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    data_hora_atualizacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_agente_loja FOREIGN KEY (id_empresa, id_loja) REFERENCES loja(id_empresa, id) ON DELETE CASCADE,
    CONSTRAINT unq_agente_loja UNIQUE (id_empresa, id_loja),
    CONSTRAINT ux_agente_id_tenant UNIQUE (id_empresa, id)
);
CREATE INDEX idx_agente_empresa ON agente(id_empresa, ativo);
CREATE INDEX idx_agente_loja ON agente(id_empresa, id_loja);

CREATE TABLE heartbeat (
    id UUID PRIMARY KEY,
    id_empresa UUID NOT NULL,
    id_agente UUID NOT NULL,
    id_loja UUID NOT NULL,
    version VARCHAR(50) NOT NULL,
    status VARCHAR(12) NOT NULL CHECK (status IN ('ONLINE','OFFLINE','DEGRADED')),
    database_ok BOOLEAN NOT NULL,
    pending_count INTEGER NOT NULL CHECK (pending_count >= 0),
    error_count INTEGER NOT NULL CHECK (error_count >= 0),
    products INTEGER NOT NULL DEFAULT 0 CHECK (products >= 0),
    customers INTEGER NOT NULL DEFAULT 0 CHECK (customers >= 0),
    employees INTEGER NOT NULL DEFAULT 0 CHECK (employees >= 0),
    suppliers INTEGER NOT NULL DEFAULT 0 CHECK (suppliers >= 0),
    data_hora_criacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    data_hora_atualizacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_heartbeat_agente FOREIGN KEY (id_empresa, id_agente) REFERENCES agente(id_empresa, id) ON DELETE CASCADE,
    CONSTRAINT fk_heartbeat_loja FOREIGN KEY (id_empresa, id_loja) REFERENCES loja(id_empresa, id) ON DELETE CASCADE,
    CONSTRAINT ux_heartbeat_id_tenant UNIQUE (id_empresa, id)
);
CREATE INDEX idx_heartbeat_loja_latest ON heartbeat(id_empresa, id_loja, data_hora_criacao DESC);
CREATE INDEX idx_heartbeat_agente_latest ON heartbeat(id_empresa, id_agente, data_hora_criacao DESC);

CREATE TABLE sync_job (
    id UUID PRIMARY KEY,
    id_empresa UUID NOT NULL REFERENCES empresa(id) ON DELETE CASCADE,
    id_loja_origem UUID NOT NULL,
    id_loja_destino UUID NOT NULL,
    entity VARCHAR(30) NOT NULL CHECK (entity IN ('product','customer','employee','supplier')),
    batch_size INTEGER NOT NULL CHECK (batch_size BETWEEN 1 AND 1000),
    status VARCHAR(30) NOT NULL CHECK (status IN ('PENDING','RUNNING','COMPLETED','COMPLETED_WITH_ERRORS','FAILED','CANCELLED')),
    total_items INTEGER NOT NULL DEFAULT 0 CHECK (total_items >= 0),
    received_items INTEGER NOT NULL DEFAULT 0 CHECK (received_items >= 0),
    processed_items INTEGER NOT NULL DEFAULT 0 CHECK (processed_items >= 0),
    error_items INTEGER NOT NULL DEFAULT 0 CHECK (error_items >= 0),
    source_finished BOOLEAN NOT NULL DEFAULT FALSE,
    finished_at TIMESTAMP,
    data_hora_criacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    data_hora_atualizacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_sync_job_origem FOREIGN KEY (id_empresa, id_loja_origem) REFERENCES loja(id_empresa, id) ON DELETE RESTRICT,
    CONSTRAINT fk_sync_job_destino FOREIGN KEY (id_empresa, id_loja_destino) REFERENCES loja(id_empresa, id) ON DELETE RESTRICT,
    CONSTRAINT ck_sync_job_distintas CHECK (id_loja_origem <> id_loja_destino),
    CONSTRAINT ck_sync_job_received CHECK (received_items <= total_items),
    CONSTRAINT ux_sync_job_id_tenant UNIQUE (id_empresa, id)
);
CREATE INDEX idx_sync_job_empresa_status ON sync_job(id_empresa, status, data_hora_criacao DESC);
CREATE INDEX idx_sync_job_origem ON sync_job(id_empresa, id_loja_origem, status);
CREATE INDEX idx_sync_job_destino ON sync_job(id_empresa, id_loja_destino, status);

CREATE TABLE transferencia (
    id UUID PRIMARY KEY,
    id_empresa UUID NOT NULL REFERENCES empresa(id) ON DELETE CASCADE,
    id_loja_origem UUID NOT NULL,
    id_loja_destino UUID NOT NULL,
    numero VARCHAR(100) NOT NULL,
    status VARCHAR(12) NOT NULL CHECK (status IN ('PENDING','SYNCED','RECEIVED','CANCELLED','FAILED')),
    item_count INTEGER NOT NULL CHECK (item_count BETWEEN 1 AND 1000),
    synced_at TIMESTAMP,
    received_at TIMESTAMP,
    data_hora_criacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    data_hora_atualizacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_transferencia_origem FOREIGN KEY (id_empresa, id_loja_origem) REFERENCES loja(id_empresa, id) ON DELETE RESTRICT,
    CONSTRAINT fk_transferencia_destino FOREIGN KEY (id_empresa, id_loja_destino) REFERENCES loja(id_empresa, id) ON DELETE RESTRICT,
    CONSTRAINT ck_transferencia_distintas CHECK (id_loja_origem <> id_loja_destino),
    CONSTRAINT unq_transferencia_numero UNIQUE (id_empresa, id_loja_origem, numero),
    CONSTRAINT ux_transferencia_id_tenant UNIQUE (id_empresa, id)
);
CREATE INDEX idx_transferencia_empresa_status ON transferencia(id_empresa, status, data_hora_criacao DESC);
CREATE INDEX idx_transferencia_origem ON transferencia(id_empresa, id_loja_origem, data_hora_criacao DESC);
CREATE INDEX idx_transferencia_destino ON transferencia(id_empresa, id_loja_destino, status);

CREATE TABLE mensagem (
    id UUID PRIMARY KEY,
    id_empresa UUID NOT NULL REFERENCES empresa(id) ON DELETE CASCADE,
    type VARCHAR(12) NOT NULL CHECK (type IN ('EVENT','COMMAND')),
    operation VARCHAR(100) NOT NULL,
    id_loja_origem UUID NOT NULL,
    id_loja_destino UUID NOT NULL,
    payload JSONB NOT NULL,
    status VARCHAR(20) NOT NULL CHECK (status IN ('PENDING','PROCESSING','ACKNOWLEDGED','RETRY','FAILED','DEAD_LETTER')),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    claim_token UUID,
    lease_until TIMESTAMP,
    next_attempt_at TIMESTAMP,
    last_attempt_at TIMESTAMP,
    last_error VARCHAR(2000) NOT NULL DEFAULT '',
    acknowledged_at TIMESTAMP,
    id_sync_job UUID,
    id_sync_batch UUID,
    id_transferencia UUID,
    data_hora_criacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    data_hora_atualizacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_mensagem_origem FOREIGN KEY (id_empresa, id_loja_origem) REFERENCES loja(id_empresa, id) ON DELETE RESTRICT,
    CONSTRAINT fk_mensagem_destino FOREIGN KEY (id_empresa, id_loja_destino) REFERENCES loja(id_empresa, id) ON DELETE RESTRICT,
    CONSTRAINT fk_mensagem_sync_job FOREIGN KEY (id_empresa, id_sync_job) REFERENCES sync_job(id_empresa, id) ON DELETE RESTRICT,
    CONSTRAINT fk_mensagem_transferencia FOREIGN KEY (id_empresa, id_transferencia) REFERENCES transferencia(id_empresa, id) ON DELETE RESTRICT,
    CONSTRAINT ck_mensagem_distintas CHECK (id_loja_origem <> id_loja_destino),
    CONSTRAINT ck_mensagem_claim CHECK ((status = 'PROCESSING' AND claim_token IS NOT NULL AND lease_until IS NOT NULL) OR (status <> 'PROCESSING' AND lease_until IS NULL)),
    CONSTRAINT ck_mensagem_ack CHECK ((status = 'ACKNOWLEDGED' AND acknowledged_at IS NOT NULL) OR (status <> 'ACKNOWLEDGED' AND acknowledged_at IS NULL)),
    CONSTRAINT ux_mensagem_id_tenant UNIQUE (id_empresa, id)
);
CREATE INDEX idx_mensagem_queue ON mensagem(id_empresa, id_loja_destino, status, next_attempt_at, data_hora_criacao);
CREATE INDEX idx_mensagem_claim_lease ON mensagem(id_empresa, id_loja_destino, lease_until) WHERE status = 'PROCESSING';
CREATE INDEX idx_mensagem_origem ON mensagem(id_empresa, id_loja_origem, data_hora_criacao DESC);
CREATE INDEX idx_mensagem_operation ON mensagem(id_empresa, operation, data_hora_criacao DESC);
CREATE INDEX idx_mensagem_job ON mensagem(id_empresa, id_sync_job, status);
CREATE INDEX idx_mensagem_transfer ON mensagem(id_empresa, id_transferencia);

CREATE TABLE sync_batch (
    id UUID PRIMARY KEY,
    id_empresa UUID NOT NULL REFERENCES empresa(id) ON DELETE CASCADE,
    id_sync_job UUID NOT NULL,
    id_mensagem UUID NOT NULL,
    batch_number INTEGER NOT NULL CHECK (batch_number >= 1),
    item_count INTEGER NOT NULL CHECK (item_count >= 0),
    processed_count INTEGER NOT NULL DEFAULT 0 CHECK (processed_count >= 0),
    error_count INTEGER NOT NULL DEFAULT 0 CHECK (error_count >= 0),
    status VARCHAR(30) NOT NULL CHECK (status IN ('PENDING','COMPLETED','COMPLETED_WITH_ERRORS','FAILED')),
    payload_hash VARCHAR(64) NOT NULL,
    total_items INTEGER NOT NULL CHECK (total_items >= 0),
    is_last BOOLEAN NOT NULL DEFAULT FALSE,
    data_hora_criacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    data_hora_atualizacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_sync_batch_job FOREIGN KEY (id_empresa, id_sync_job) REFERENCES sync_job(id_empresa, id) ON DELETE CASCADE,
    CONSTRAINT fk_sync_batch_mensagem FOREIGN KEY (id_empresa, id_mensagem) REFERENCES mensagem(id_empresa, id) ON DELETE CASCADE,
    CONSTRAINT unq_sync_batch_number UNIQUE (id_empresa, id_sync_job, batch_number),
    CONSTRAINT unq_sync_batch_message UNIQUE (id_empresa, id_mensagem),
    CONSTRAINT ck_sync_batch_result CHECK (processed_count + error_count <= item_count),
    CONSTRAINT ux_sync_batch_id_tenant UNIQUE (id_empresa, id)
);
CREATE INDEX idx_sync_batch_job ON sync_batch(id_empresa, id_sync_job, batch_number);
CREATE INDEX idx_sync_batch_status ON sync_batch(id_empresa, status, data_hora_criacao);

ALTER TABLE mensagem ADD CONSTRAINT fk_mensagem_sync_batch
    FOREIGN KEY (id_empresa, id_sync_batch) REFERENCES sync_batch(id_empresa, id)
    ON DELETE RESTRICT DEFERRABLE INITIALLY DEFERRED;

CREATE TABLE mensagem_tentativa (
    id UUID PRIMARY KEY,
    id_empresa UUID NOT NULL REFERENCES empresa(id) ON DELETE CASCADE,
    id_mensagem UUID NOT NULL,
    id_agente UUID NOT NULL,
    claim_token UUID NOT NULL,
    status VARCHAR(20) NOT NULL CHECK (status IN ('PROCESSING','ACKNOWLEDGED','RETRY','DEAD_LETTER')),
    error VARCHAR(2000) NOT NULL DEFAULT '',
    started_at TIMESTAMP NOT NULL,
    finished_at TIMESTAMP,
    data_hora_criacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    data_hora_atualizacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_mensagem_tentativa_mensagem FOREIGN KEY (id_empresa, id_mensagem) REFERENCES mensagem(id_empresa, id) ON DELETE CASCADE,
    CONSTRAINT fk_mensagem_tentativa_agente FOREIGN KEY (id_empresa, id_agente) REFERENCES agente(id_empresa, id) ON DELETE CASCADE,
    CONSTRAINT unq_mensagem_tentativa_claim UNIQUE (id_empresa, id_mensagem, claim_token),
    CONSTRAINT ux_mensagem_tentativa_id_tenant UNIQUE (id_empresa, id)
);
CREATE INDEX idx_mensagem_tentativa_message ON mensagem_tentativa(id_empresa, id_mensagem, started_at);

CREATE TABLE sync_item (
    id UUID PRIMARY KEY,
    id_empresa UUID NOT NULL REFERENCES empresa(id) ON DELETE CASCADE,
    id_sync_job UUID NOT NULL,
    id_sync_batch UUID NOT NULL,
    item_key VARCHAR(255) NOT NULL,
    error VARCHAR(2000) NOT NULL,
    status VARCHAR(12) NOT NULL CHECK (status IN ('FAILED','RESOLVED')),
    data_hora_criacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    data_hora_atualizacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_sync_item_job FOREIGN KEY (id_empresa, id_sync_job) REFERENCES sync_job(id_empresa, id) ON DELETE CASCADE,
    CONSTRAINT fk_sync_item_batch FOREIGN KEY (id_empresa, id_sync_batch) REFERENCES sync_batch(id_empresa, id) ON DELETE CASCADE,
    CONSTRAINT unq_sync_item_item UNIQUE (id_empresa, id_sync_batch, item_key),
    CONSTRAINT ux_sync_item_id_tenant UNIQUE (id_empresa, id)
);
CREATE INDEX idx_sync_item_job ON sync_item(id_empresa, id_sync_job, status);

CREATE TABLE integracao_erro (
    id UUID PRIMARY KEY,
    id_empresa UUID NOT NULL REFERENCES empresa(id) ON DELETE CASCADE,
    id_loja UUID NOT NULL,
    id_mensagem UUID,
    id_sync_job UUID,
    id_sync_batch UUID,
    operation VARCHAR(100) NOT NULL,
    error VARCHAR(2000) NOT NULL,
    item_key VARCHAR(255) NOT NULL DEFAULT '',
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    status VARCHAR(12) NOT NULL CHECK (status IN ('OPEN','RESOLVED')),
    resolved_at TIMESTAMP,
    data_hora_criacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    data_hora_atualizacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_integracao_erro_loja FOREIGN KEY (id_empresa, id_loja) REFERENCES loja(id_empresa, id) ON DELETE CASCADE,
    CONSTRAINT fk_integracao_erro_mensagem FOREIGN KEY (id_empresa, id_mensagem) REFERENCES mensagem(id_empresa, id) ON DELETE CASCADE,
    CONSTRAINT fk_integracao_erro_job FOREIGN KEY (id_empresa, id_sync_job) REFERENCES sync_job(id_empresa, id) ON DELETE CASCADE,
    CONSTRAINT fk_integracao_erro_batch FOREIGN KEY (id_empresa, id_sync_batch) REFERENCES sync_batch(id_empresa, id) ON DELETE CASCADE,
    CONSTRAINT ck_integracao_erro_resolution CHECK ((status = 'RESOLVED' AND resolved_at IS NOT NULL) OR (status = 'OPEN' AND resolved_at IS NULL)),
    CONSTRAINT ux_integracao_erro_id_tenant UNIQUE (id_empresa, id)
);
CREATE INDEX idx_integracao_erro_empresa_status ON integracao_erro(id_empresa, status, data_hora_criacao DESC);
CREATE INDEX idx_integracao_erro_loja ON integracao_erro(id_empresa, id_loja, status);
CREATE INDEX idx_integracao_erro_operation ON integracao_erro(id_empresa, operation, data_hora_criacao DESC);
CREATE INDEX idx_integracao_erro_job ON integracao_erro(id_empresa, id_sync_job, id_sync_batch);

CREATE TABLE transferencia_item (
    id UUID PRIMARY KEY,
    id_empresa UUID NOT NULL REFERENCES empresa(id) ON DELETE CASCADE,
    id_transferencia UUID NOT NULL,
    product_id VARCHAR(255) NOT NULL,
    description VARCHAR(500) NOT NULL DEFAULT '',
    quantity NUMERIC(18,4) NOT NULL CHECK (quantity > 0),
    data_hora_criacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    data_hora_atualizacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_transferencia_item_transfer FOREIGN KEY (id_empresa, id_transferencia) REFERENCES transferencia(id_empresa, id) ON DELETE CASCADE,
    CONSTRAINT ux_transferencia_item_id_tenant UNIQUE (id_empresa, id)
);
CREATE INDEX idx_transferencia_item_transfer ON transferencia_item(id_empresa, id_transferencia);

CREATE TABLE audit_log (
    id UUID PRIMARY KEY,
    id_empresa UUID NOT NULL REFERENCES empresa(id) ON DELETE CASCADE,
    id_usuario UUID REFERENCES usuario(id) ON DELETE SET NULL,
    id_loja UUID,
    action VARCHAR(100) NOT NULL,
    details JSONB NOT NULL DEFAULT '{}'::jsonb,
    data_hora_criacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    data_hora_atualizacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_audit_loja FOREIGN KEY (id_empresa, id_loja) REFERENCES loja(id_empresa, id) ON DELETE CASCADE,
    CONSTRAINT ux_audit_log_id_tenant UNIQUE (id_empresa, id)
);
CREATE INDEX idx_audit_empresa_created ON audit_log(id_empresa, data_hora_criacao DESC);
CREATE INDEX idx_audit_empresa_action ON audit_log(id_empresa, action, data_hora_criacao DESC);
CREATE INDEX idx_audit_empresa_loja ON audit_log(id_empresa, id_loja, data_hora_criacao DESC);

