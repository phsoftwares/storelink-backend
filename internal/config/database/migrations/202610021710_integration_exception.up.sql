CREATE TABLE integracao_excecao (
    id UUID PRIMARY KEY,
    id_empresa UUID NOT NULL REFERENCES empresa(id) ON DELETE CASCADE,
    id_loja UUID NOT NULL,
    chave_idempotencia VARCHAR(150) NOT NULL,
    sistema VARCHAR(100) NOT NULL,
    formulario VARCHAR(255) NOT NULL DEFAULT '',
    usuario VARCHAR(100) NOT NULL DEFAULT '',
    caption_formulario VARCHAR(255) NOT NULL DEFAULT '',
    versao_sistema VARCHAR(50) NOT NULL DEFAULT '',
    razao_social VARCHAR(255) NOT NULL DEFAULT '',
    nome_terminal VARCHAR(100) NOT NULL DEFAULT '',
    mensagem VARCHAR(2000) NOT NULL,
    data_hora_ocorrencia TIMESTAMP,
    data_hora_transmissao TIMESTAMP,
    hash_payload VARCHAR(64) NOT NULL,
    status VARCHAR(12) NOT NULL DEFAULT 'OPEN' CHECK (status IN ('OPEN','RESOLVED')),
    data_hora_criacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    data_hora_atualizacao TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_integracao_excecao_loja FOREIGN KEY (id_empresa, id_loja) REFERENCES loja(id_empresa, id) ON DELETE CASCADE,
    CONSTRAINT ux_integracao_excecao_id_tenant UNIQUE (id_empresa, id),
    CONSTRAINT ux_integracao_excecao_key UNIQUE (id_empresa, id_loja, chave_idempotencia)
);

CREATE INDEX idx_integracao_excecao_loja ON integracao_excecao(id_empresa, id_loja, data_hora_criacao DESC);
