CREATE INDEX idx_produto_sku_normalized_lookup
    ON produto(id_empresa, UPPER(BTRIM(sku)))
    WHERE sku IS NOT NULL AND BTRIM(sku) <> '';

CREATE INDEX idx_produto_codigo_barras_normalized_lookup
    ON produto_codigo_barras(id_empresa, codigo_barras_normalizado, id_produto);
