ALTER TABLE agente
    ALTER COLUMN id_loja DROP NOT NULL;

ALTER TABLE agente
    DROP CONSTRAINT unq_agente_loja;

CREATE UNIQUE INDEX ux_agente_empresa_compartilhado
    ON agente(id_empresa)
    WHERE id_loja IS NULL;

CREATE INDEX idx_loja_empresa_cnpj_normalizado
    ON loja(
        id_empresa,
        (regexp_replace(upper(coalesce(cnpj, '')), '[^A-Z0-9]', '', 'g'))
    )
    WHERE cnpj <> '';
