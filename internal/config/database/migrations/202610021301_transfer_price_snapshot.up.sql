ALTER TABLE transferencia_item
    ADD COLUMN tabela_preco_informada BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN tabela_preco_payload TEXT NOT NULL DEFAULT '',
    ADD COLUMN usa_tb_pc_informada BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN usa_tb_pc VARCHAR(20) NOT NULL DEFAULT '';
