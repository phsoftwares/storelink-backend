ALTER TABLE produto_loja_mapeamento
    ADD COLUMN codigo_grupo_local VARCHAR(100) NOT NULL DEFAULT '',
    ADD COLUMN grupo_nome VARCHAR(100) NOT NULL DEFAULT '',
    ADD COLUMN codigo_subgrupo_local VARCHAR(100) NOT NULL DEFAULT '',
    ADD COLUMN subgrupo_nome VARCHAR(100) NOT NULL DEFAULT '';

UPDATE produto_loja_mapeamento mapping
SET grupo_nome = product.grupo_nome,
    subgrupo_nome = product.subgrupo_nome
FROM produto product
WHERE product.id_empresa = mapping.id_empresa
  AND product.id = mapping.id_produto;
