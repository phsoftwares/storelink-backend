ALTER TABLE loja DROP CONSTRAINT IF EXISTS ck_loja_cnpj_obrigatorio;
DROP INDEX IF EXISTS ux_loja_empresa_cnpj_normalizado;

CREATE INDEX IF NOT EXISTS idx_loja_empresa_cnpj
    ON loja(id_empresa, cnpj)
    WHERE cnpj <> '';

ALTER TABLE loja
    ALTER COLUMN cnpj SET DEFAULT '';
