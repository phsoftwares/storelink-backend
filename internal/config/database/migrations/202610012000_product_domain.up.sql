CREATE TABLE produto (
    id UUID PRIMARY KEY,
    id_empresa UUID NOT NULL REFERENCES empresa(id) ON DELETE CASCADE,
    sku VARCHAR(100),
    nome VARCHAR(255) NOT NULL,
    descricao VARCHAR(1000) NOT NULL DEFAULT '',
    unidade VARCHAR(20) NOT NULL DEFAULT '',
    marca VARCHAR(100) NOT NULL DEFAULT '',
    ncm VARCHAR(20) NOT NULL DEFAULT '',
    cest VARCHAR(20) NOT NULL DEFAULT '',
    grupo_nome VARCHAR(100) NOT NULL DEFAULT '',
    subgrupo_nome VARCHAR(100) NOT NULL DEFAULT '',
    ativo BOOLEAN NOT NULL DEFAULT TRUE,
    tracks_serial BOOLEAN NOT NULL DEFAULT FALSE,
    data_hora_criacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    data_hora_atualizacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ux_produto_id_tenant UNIQUE (id_empresa, id)
);
CREATE UNIQUE INDEX ux_produto_empresa_sku
    ON produto(id_empresa, UPPER(sku)) WHERE sku IS NOT NULL AND BTRIM(sku) <> '';
CREATE INDEX idx_produto_empresa_nome ON produto(id_empresa, nome);

CREATE TABLE produto_codigo_barras (
    id UUID PRIMARY KEY,
    id_empresa UUID NOT NULL REFERENCES empresa(id) ON DELETE CASCADE,
    id_produto UUID NOT NULL,
    codigo_barras VARCHAR(32) NOT NULL,
    codigo_barras_normalizado VARCHAR(32) NOT NULL,
    principal BOOLEAN NOT NULL DEFAULT FALSE,
    ativo BOOLEAN NOT NULL DEFAULT TRUE,
    fraction NUMERIC(18,6),
    data_hora_criacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    data_hora_atualizacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_produto_codigo_barras_produto FOREIGN KEY (id_empresa, id_produto)
        REFERENCES produto(id_empresa, id) ON DELETE CASCADE,
    CONSTRAINT ck_produto_codigo_barras_valor CHECK (BTRIM(codigo_barras) <> ''),
    CONSTRAINT ck_produto_codigo_barras_fraction CHECK (fraction IS NULL OR fraction >= 0),
    CONSTRAINT ux_produto_codigo_barras_id_tenant UNIQUE (id_empresa, id)
);
CREATE UNIQUE INDEX ux_produto_codigo_barras_tenant_value
    ON produto_codigo_barras(id_empresa, codigo_barras_normalizado) WHERE ativo = TRUE;
CREATE UNIQUE INDEX ux_produto_codigo_barras_primary
    ON produto_codigo_barras(id_empresa, id_produto) WHERE principal = TRUE AND ativo = TRUE;
CREATE INDEX idx_produto_codigo_barras_product
    ON produto_codigo_barras(id_empresa, id_produto, ativo);

CREATE TABLE produto_loja_mapeamento (
    id UUID PRIMARY KEY,
    id_empresa UUID NOT NULL REFERENCES empresa(id) ON DELETE CASCADE,
    id_loja UUID NOT NULL,
    id_produto UUID NOT NULL,
    codigo_local VARCHAR(100) NOT NULL,
    data_hora_criacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    data_hora_atualizacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_produto_loja_mapeamento_loja FOREIGN KEY (id_empresa, id_loja)
        REFERENCES loja(id_empresa, id) ON DELETE CASCADE,
    CONSTRAINT fk_produto_loja_mapeamento_produto FOREIGN KEY (id_empresa, id_produto)
        REFERENCES produto(id_empresa, id) ON DELETE CASCADE,
    CONSTRAINT ck_produto_loja_mapeamento_codigo CHECK (BTRIM(codigo_local) <> ''),
    CONSTRAINT ux_produto_loja_mapeamento_id_tenant UNIQUE (id_empresa, id),
    CONSTRAINT unq_produto_loja_produto UNIQUE (id_empresa, id_loja, id_produto)
);
CREATE UNIQUE INDEX ux_produto_loja_codigo_normalizado
    ON produto_loja_mapeamento(id_empresa, id_loja, UPPER(BTRIM(codigo_local)));
CREATE INDEX idx_produto_loja_mapeamento_product
    ON produto_loja_mapeamento(id_empresa, id_produto);

CREATE TABLE produto_preco_loja (
    id UUID PRIMARY KEY,
    id_empresa UUID NOT NULL REFERENCES empresa(id) ON DELETE CASCADE,
    id_loja UUID NOT NULL,
    id_produto UUID NOT NULL,
    tipo VARCHAR(20) NOT NULL CHECK (tipo IN ('COST','SALE','WHOLESALE')),
    quantity NUMERIC(18,4) NOT NULL DEFAULT 1 CHECK (quantity > 0),
    amount NUMERIC(18,4) NOT NULL CHECK (amount >= 0),
    ativo BOOLEAN NOT NULL DEFAULT TRUE,
    valid_from TIMESTAMP,
    data_hora_criacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    data_hora_atualizacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_produto_preco_loja_loja FOREIGN KEY (id_empresa, id_loja)
        REFERENCES loja(id_empresa, id) ON DELETE CASCADE,
    CONSTRAINT fk_produto_preco_loja_produto FOREIGN KEY (id_empresa, id_produto)
        REFERENCES produto(id_empresa, id) ON DELETE CASCADE,
    CONSTRAINT ux_produto_preco_loja_id_tenant UNIQUE (id_empresa, id),
    CONSTRAINT unq_produto_preco_loja_tier UNIQUE (id_empresa, id_loja, id_produto, tipo, quantity)
);
CREATE INDEX idx_produto_preco_loja_product ON produto_preco_loja(id_empresa, id_loja, id_produto, ativo);

ALTER TABLE transferencia_item RENAME COLUMN product_id TO codigo_produto_origem;
ALTER TABLE transferencia_item ALTER COLUMN codigo_produto_origem DROP NOT NULL;
ALTER TABLE transferencia_item
    ADD COLUMN id_produto UUID,
    ADD COLUMN codigo_barras VARCHAR(32) NOT NULL DEFAULT '',
    ADD COLUMN sku VARCHAR(100) NOT NULL DEFAULT '',
    ADD COLUMN unit VARCHAR(20) NOT NULL DEFAULT '',
    ADD COLUMN status VARCHAR(16) NOT NULL DEFAULT 'PENDING'
        CHECK (status IN ('PENDING','STORED','FAILED')),
    ADD COLUMN error_code VARCHAR(80) NOT NULL DEFAULT '',
    ADD COLUMN erro_detalhe TEXT NOT NULL DEFAULT '';
ALTER TABLE transferencia_item ADD CONSTRAINT fk_transferencia_item_produto
    FOREIGN KEY (id_empresa, id_produto) REFERENCES produto(id_empresa, id) ON DELETE RESTRICT;
CREATE INDEX idx_transferencia_item_status ON transferencia_item(id_empresa, id_transferencia, status);

ALTER TABLE integracao_erro ADD COLUMN error_code VARCHAR(80) NOT NULL DEFAULT '';
ALTER TABLE sync_item ADD COLUMN error_code VARCHAR(80) NOT NULL DEFAULT '';
