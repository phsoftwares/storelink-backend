ALTER TABLE produto_loja_mapeamento
    DROP COLUMN IF EXISTS codigo_grupo_local,
    DROP COLUMN IF EXISTS grupo_nome,
    DROP COLUMN IF EXISTS codigo_subgrupo_local,
    DROP COLUMN IF EXISTS subgrupo_nome;
