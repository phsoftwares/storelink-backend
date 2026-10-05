ALTER TABLE integracao_erro DROP COLUMN error_code;
ALTER TABLE sync_item DROP COLUMN error_code;

DROP INDEX IF EXISTS idx_transferencia_item_status;
ALTER TABLE transferencia_item DROP CONSTRAINT IF EXISTS fk_transferencia_item_produto;
ALTER TABLE transferencia_item
    DROP COLUMN IF EXISTS id_produto,
    DROP COLUMN IF EXISTS codigo_barras,
    DROP COLUMN IF EXISTS sku,
    DROP COLUMN IF EXISTS unit,
    DROP COLUMN IF EXISTS status,
    DROP COLUMN IF EXISTS error_code,
    DROP COLUMN IF EXISTS erro_detalhe;
UPDATE transferencia_item
SET codigo_produto_origem = COALESCE(NULLIF(BTRIM(codigo_produto_origem), ''), 'unknown')
WHERE codigo_produto_origem IS NULL OR BTRIM(codigo_produto_origem) = '';
ALTER TABLE transferencia_item ALTER COLUMN codigo_produto_origem SET NOT NULL;
ALTER TABLE transferencia_item RENAME COLUMN codigo_produto_origem TO product_id;

DROP TABLE IF EXISTS produto_preco_loja;
DROP INDEX IF EXISTS idx_produto_loja_mapeamento_product;
DROP TABLE IF EXISTS produto_loja_mapeamento;
DROP INDEX IF EXISTS idx_produto_codigo_barras_product;
DROP INDEX IF EXISTS ux_produto_codigo_barras_primary;
DROP INDEX IF EXISTS ux_produto_codigo_barras_tenant_value;
DROP TABLE IF EXISTS produto_codigo_barras;
DROP INDEX IF EXISTS idx_produto_empresa_nome;
DROP INDEX IF EXISTS ux_produto_empresa_sku;
DROP TABLE IF EXISTS produto;
