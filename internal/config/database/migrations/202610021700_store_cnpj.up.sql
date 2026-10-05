ALTER TABLE loja
    ADD COLUMN cnpj VARCHAR(20) NOT NULL DEFAULT '';

CREATE INDEX idx_loja_empresa_cnpj
    ON loja(id_empresa, cnpj)
    WHERE cnpj <> '';
