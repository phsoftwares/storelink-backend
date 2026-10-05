ALTER TABLE produto
    DROP COLUMN IF EXISTS pfcpst,
    DROP COLUMN IF EXISTS pfcp,
    DROP COLUMN IF EXISTS aliquota_cofins,
    DROP COLUMN IF EXISTS aliquota_pis,
    DROP COLUMN IF EXISTS preco_farma_pop,
    DROP COLUMN IF EXISTS preco_venda_anterior,
    DROP COLUMN IF EXISTS preco_custo_anterior,
    DROP COLUMN IF EXISTS usa_grade,
    DROP COLUMN IF EXISTS codigo_barra_novartis,
    DROP COLUMN IF EXISTS codigo_receita,
    DROP COLUMN IF EXISTS codigo_marca;
