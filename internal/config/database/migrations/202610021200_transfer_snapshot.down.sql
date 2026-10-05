ALTER TABLE transferencia_item
    DROP COLUMN IF EXISTS unidade_sngpc,
    DROP COLUMN IF EXISTS tipo_produto_sngpc,
    DROP COLUMN IF EXISTS classe_terapeutica,
    DROP COLUMN IF EXISTS registro_ms,
    DROP COLUMN IF EXISTS tracks_serial,
    DROP COLUMN IF EXISTS tracks_batch,
    DROP COLUMN IF EXISTS controlled,
    DROP COLUMN IF EXISTS data_fabricacao,
    DROP COLUMN IF EXISTS data_validade,
    DROP COLUMN IF EXISTS serial,
    DROP COLUMN IF EXISTS lot,
    DROP COLUMN IF EXISTS lote_serial,
    DROP COLUMN IF EXISTS codigo_lote,
    DROP COLUMN IF EXISTS preco_custo,
    DROP COLUMN IF EXISTS preco_venda;

ALTER TABLE transferencia
    DROP COLUMN IF EXISTS data_envio,
    DROP COLUMN IF EXISTS observacao_remetente,
    DROP COLUMN IF EXISTS usuario_remetente,
    DROP COLUMN IF EXISTS cnpj_destino,
    DROP COLUMN IF EXISTS cnpj_origem;
