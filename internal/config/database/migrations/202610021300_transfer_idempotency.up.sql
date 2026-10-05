ALTER TABLE transferencia
    ADD COLUMN payload_hash VARCHAR(64) NOT NULL DEFAULT '';

ALTER TABLE operador_caixa
    DROP CONSTRAINT IF EXISTS unq_operador_caixa_loja_codigo;

CREATE UNIQUE INDEX ux_operador_caixa_loja_codigo_normalizado
    ON operador_caixa(id_empresa, id_loja, UPPER(BTRIM(codigo_local)));
