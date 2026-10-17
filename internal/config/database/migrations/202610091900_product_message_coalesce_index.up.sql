-- Product coalescing uses the nested catalog identity from the real FNTS payload.
-- Build concurrently so queue publishers/consumers remain writable during rollout.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_mensagem_product_coalesce
    ON mensagem (
        id_empresa,
        id_loja_origem,
        id_loja_destino,
        (COALESCE(
            NULLIF(BTRIM(payload->'produto'->>'id_produto'), ''),
            NULLIF(BTRIM(payload->'produto'->>'codigo_local'), ''),
            NULLIF(BTRIM(payload->'product'->>'id_produto'), ''),
            NULLIF(BTRIM(payload->'product'->>'productId'), ''),
            NULLIF(BTRIM(payload->'product'->>'codigo_local'), ''),
            NULLIF(BTRIM(payload->'product'->>'localCode'), ''),
            COALESCE(
                NULLIF(BTRIM(payload->>'codigo'), ''),
                NULLIF(BTRIM(payload->>'id'), ''),
                id::text
            )
        )),
        data_hora_criacao,
        id
    )
    WHERE status IN ('PENDING', 'RETRY')
      AND operation IN ('product.created', 'product.updated', 'product.disabled');
