DROP INDEX IF EXISTS idx_operador_caixa_funcionario;
DROP TABLE IF EXISTS operador_caixa;

DROP INDEX IF EXISTS idx_funcionario_loja_funcionario;
DROP INDEX IF EXISTS ux_funcionario_loja_codigo_normalizado;
DROP TABLE IF EXISTS funcionario_loja_mapeamento;
DROP INDEX IF EXISTS ux_funcionario_empresa_documento;
DROP INDEX IF EXISTS idx_funcionario_empresa_nome;
DROP TABLE IF EXISTS funcionario;

DROP INDEX IF EXISTS idx_cliente_loja_cliente;
DROP INDEX IF EXISTS ux_cliente_loja_codigo_normalizado;
DROP TABLE IF EXISTS cliente_loja_mapeamento;
DROP INDEX IF EXISTS ux_cliente_empresa_documento;
DROP INDEX IF EXISTS idx_cliente_empresa_nome;
DROP TABLE IF EXISTS cliente;

ALTER TABLE grupo_loja DROP COLUMN IF EXISTS integrar_funcionarios;
