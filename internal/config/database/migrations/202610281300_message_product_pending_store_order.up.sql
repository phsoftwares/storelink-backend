-- Product claim pages are ordered by creation time after filtering to one
-- destination and the three product event operations. This partial index lets
-- the worker read the next page directly instead of sorting/scanning every
-- pending message for the destination.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_mensagem_product_pending_store_order
    ON mensagem (
        id_empresa,
        id_loja_destino,
        data_hora_criacao,
        id
    )
    WHERE status IN ('PENDING', 'RETRY')
      AND operation IN ('product.created', 'product.updated', 'product.disabled');
