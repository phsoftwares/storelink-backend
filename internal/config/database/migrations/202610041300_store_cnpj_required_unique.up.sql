-- New stores must identify the physical FNTS installation by its CNPJ.
-- Existing empty rows are intentionally left for an explicit data repair;
-- NOT VALID keeps deployment non-destructive while enforcing all new rows.
ALTER TABLE loja
    ALTER COLUMN cnpj DROP DEFAULT;

DROP INDEX IF EXISTS idx_loja_empresa_cnpj;
DROP INDEX IF EXISTS idx_loja_empresa_cnpj_normalizado;

CREATE UNIQUE INDEX ux_loja_empresa_cnpj_normalizado
    ON loja(
        id_empresa,
        (regexp_replace(upper(coalesce(cnpj, '')), '[^0-9]', '', 'g'))
    )
    WHERE cnpj <> '';

ALTER TABLE loja
    ADD CONSTRAINT ck_loja_cnpj_obrigatorio
    CHECK (cnpj ~ '^[0-9]{14}$') NOT VALID;
