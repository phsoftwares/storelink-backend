ALTER TABLE transferencia_item
    DROP COLUMN IF EXISTS usa_tb_pc,
    DROP COLUMN IF EXISTS usa_tb_pc_informada,
    DROP COLUMN IF EXISTS tabela_preco_payload,
    DROP COLUMN IF EXISTS tabela_preco_informada;
