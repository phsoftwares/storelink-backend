DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM agente WHERE id_loja IS NULL) THEN
        RAISE EXCEPTION 'nao e possivel remover o agente compartilhado enquanto houver credenciais sem loja';
    END IF;
END $$;

DROP INDEX IF EXISTS idx_loja_empresa_cnpj_normalizado;
DROP INDEX IF EXISTS ux_agente_empresa_compartilhado;

ALTER TABLE agente
    ADD CONSTRAINT unq_agente_loja UNIQUE (id_empresa, id_loja);

ALTER TABLE agente
    ALTER COLUMN id_loja SET NOT NULL;
