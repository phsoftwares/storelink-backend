DROP INDEX IF EXISTS ux_operador_caixa_loja_codigo_normalizado;

ALTER TABLE operador_caixa
    ADD CONSTRAINT unq_operador_caixa_loja_codigo
    UNIQUE (id_empresa, id_loja, codigo_local);

ALTER TABLE transferencia DROP COLUMN IF EXISTS payload_hash;
