-- Customer, employee and other domain claims coalesce by their generic local
-- identity. Keep the indexed expressions and operation prefix aligned with
-- AcknowledgeSupersededBulk so a small domain batch does not scan a large
-- pending product queue.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_mensagem_domain_coalesce_pending
    ON mensagem (
        id_empresa,
        id_loja_origem,
        id_loja_destino,
        operation varchar_pattern_ops,
        (COALESCE(
            NULLIF(BTRIM(payload->>'codigo'), ''),
            NULLIF(BTRIM(payload->>'id'), ''),
            id::text
        )),
        data_hora_criacao,
        id
    )
    WHERE status IN ('PENDING', 'RETRY');
