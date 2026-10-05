DROP INDEX IF EXISTS idx_loja_empresa_cnpj;

ALTER TABLE loja
    DROP COLUMN IF EXISTS cnpj;
