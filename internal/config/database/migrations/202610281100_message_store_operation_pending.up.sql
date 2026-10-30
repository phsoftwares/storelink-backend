-- Claims are scoped to one destination store and an operation prefix. Without
-- this index, a customer/employee claim scans every pending product event for
-- that store during a large initial catalog load.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_mensagem_store_operation_pending
    ON mensagem (
        id_empresa,
        id_loja_destino,
        operation text_pattern_ops,
        data_hora_criacao,
        id
    )
    WHERE status IN ('PENDING', 'RETRY');
