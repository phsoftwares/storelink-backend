ALTER TABLE grupo_loja
    ADD COLUMN integrar_funcionarios BOOLEAN NOT NULL DEFAULT TRUE;

CREATE TABLE cliente (
    id UUID PRIMARY KEY,
    id_empresa UUID NOT NULL REFERENCES empresa(id) ON DELETE CASCADE,
    nome VARCHAR(255) NOT NULL,
    documento VARCHAR(20) NOT NULL DEFAULT '',
    telefone VARCHAR(50) NOT NULL DEFAULT '',
    celular VARCHAR(50) NOT NULL DEFAULT '',
    email VARCHAR(254) NOT NULL DEFAULT '',
    endereco JSONB NOT NULL DEFAULT '{}'::jsonb,
    situacao_credito VARCHAR(20) NOT NULL CHECK (situacao_credito IN ('STANDARD','UNDER_REVIEW','UNCLASSIFIED')),
    data_hora_criacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    data_hora_atualizacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ux_cliente_id_tenant UNIQUE (id_empresa, id)
);
CREATE INDEX idx_cliente_empresa_nome ON cliente(id_empresa, nome, id);
CREATE UNIQUE INDEX ux_cliente_empresa_documento ON cliente(id_empresa, regexp_replace(documento, '[^0-9]', '', 'g'))
    WHERE documento <> '';

CREATE TABLE cliente_loja_mapeamento (
    id UUID PRIMARY KEY,
    id_empresa UUID NOT NULL REFERENCES empresa(id) ON DELETE CASCADE,
    id_loja UUID NOT NULL,
    id_cliente UUID NOT NULL,
    codigo_local VARCHAR(100) NOT NULL,
    data_hora_criacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    data_hora_atualizacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_cliente_mapeamento_loja FOREIGN KEY (id_empresa, id_loja)
        REFERENCES loja(id_empresa, id) ON DELETE CASCADE,
    CONSTRAINT fk_cliente_mapeamento_cliente FOREIGN KEY (id_empresa, id_cliente)
        REFERENCES cliente(id_empresa, id) ON DELETE CASCADE,
    CONSTRAINT ck_cliente_mapeamento_codigo CHECK (BTRIM(codigo_local) <> ''),
    CONSTRAINT ux_cliente_mapeamento_id_tenant UNIQUE (id_empresa, id),
    CONSTRAINT unq_cliente_loja_cliente UNIQUE (id_empresa, id_loja, id_cliente)
);
CREATE UNIQUE INDEX ux_cliente_loja_codigo_normalizado
    ON cliente_loja_mapeamento(id_empresa, id_loja, UPPER(BTRIM(codigo_local)));
CREATE INDEX idx_cliente_loja_cliente ON cliente_loja_mapeamento(id_empresa, id_cliente);

CREATE TABLE funcionario (
    id UUID PRIMARY KEY,
    id_empresa UUID NOT NULL REFERENCES empresa(id) ON DELETE CASCADE,
    nome VARCHAR(255) NOT NULL,
    documento VARCHAR(20) NOT NULL DEFAULT '',
    email VARCHAR(254) NOT NULL DEFAULT '',
    telefone VARCHAR(50) NOT NULL DEFAULT '',
    funcao VARCHAR(100) NOT NULL DEFAULT '',
    ativo BOOLEAN NOT NULL,
    data_hora_criacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    data_hora_atualizacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ux_funcionario_id_tenant UNIQUE (id_empresa, id)
);
CREATE INDEX idx_funcionario_empresa_nome ON funcionario(id_empresa, nome, id);
CREATE UNIQUE INDEX ux_funcionario_empresa_documento ON funcionario(id_empresa, regexp_replace(documento, '[^0-9]', '', 'g'))
    WHERE documento <> '';

CREATE TABLE funcionario_loja_mapeamento (
    id UUID PRIMARY KEY,
    id_empresa UUID NOT NULL REFERENCES empresa(id) ON DELETE CASCADE,
    id_loja UUID NOT NULL,
    id_funcionario UUID NOT NULL,
    codigo_local VARCHAR(100) NOT NULL,
    data_hora_criacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    data_hora_atualizacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_funcionario_mapeamento_loja FOREIGN KEY (id_empresa, id_loja)
        REFERENCES loja(id_empresa, id) ON DELETE CASCADE,
    CONSTRAINT fk_funcionario_mapeamento_funcionario FOREIGN KEY (id_empresa, id_funcionario)
        REFERENCES funcionario(id_empresa, id) ON DELETE CASCADE,
    CONSTRAINT ck_funcionario_mapeamento_codigo CHECK (BTRIM(codigo_local) <> ''),
    CONSTRAINT ux_funcionario_mapeamento_id_tenant UNIQUE (id_empresa, id),
    CONSTRAINT unq_funcionario_loja_funcionario UNIQUE (id_empresa, id_loja, id_funcionario)
);
CREATE UNIQUE INDEX ux_funcionario_loja_codigo_normalizado
    ON funcionario_loja_mapeamento(id_empresa, id_loja, UPPER(BTRIM(codigo_local)));
CREATE INDEX idx_funcionario_loja_funcionario ON funcionario_loja_mapeamento(id_empresa, id_funcionario);

CREATE TABLE operador_caixa (
    id UUID PRIMARY KEY,
    id_empresa UUID NOT NULL REFERENCES empresa(id) ON DELETE CASCADE,
    id_loja UUID NOT NULL,
    id_funcionario UUID NOT NULL,
    codigo_local VARCHAR(100) NOT NULL,
    pode_abrir_caixa_geral BOOLEAN NOT NULL,
    pode_visualizar_todos_relatorios BOOLEAN NOT NULL,
    fechamento_cego BOOLEAN NOT NULL,
    data_hora_criacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    data_hora_atualizacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_operador_caixa_loja FOREIGN KEY (id_empresa, id_loja)
        REFERENCES loja(id_empresa, id) ON DELETE CASCADE,
    CONSTRAINT fk_operador_caixa_funcionario FOREIGN KEY (id_empresa, id_funcionario)
        REFERENCES funcionario(id_empresa, id) ON DELETE CASCADE,
    CONSTRAINT ck_operador_caixa_codigo CHECK (BTRIM(codigo_local) <> ''),
    CONSTRAINT ux_operador_caixa_id_tenant UNIQUE (id_empresa, id),
    CONSTRAINT unq_operador_caixa_loja_codigo UNIQUE (id_empresa, id_loja, codigo_local)
);
CREATE INDEX idx_operador_caixa_funcionario ON operador_caixa(id_empresa, id_funcionario);
