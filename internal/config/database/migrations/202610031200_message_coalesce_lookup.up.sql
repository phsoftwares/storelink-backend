-- Lookup used by the initial customer/employee catalog claim. The expression
-- matches the identity fallback used by the coalescing query and bulk ack.
CREATE INDEX idx_mensagem_carga_identidade
    ON mensagem(
        id_empresa,
        id_loja_origem,
        id_loja_destino,
        operation,
        status,
        (COALESCE(NULLIF(payload->>'codigo', ''), NULLIF(payload->>'id', ''), id::text)),
        data_hora_criacao,
        id
    )
    WHERE status IN ('PENDING', 'RETRY');
