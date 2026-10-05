DROP INDEX IF EXISTS ux_funcionario_empresa_documento;
CREATE UNIQUE INDEX ux_funcionario_empresa_documento
    ON funcionario(id_empresa, regexp_replace(documento, '[^0-9]', '', 'g'))
    WHERE documento <> '';

DROP INDEX IF EXISTS ux_cliente_empresa_documento;
CREATE UNIQUE INDEX ux_cliente_empresa_documento
    ON cliente(id_empresa, regexp_replace(documento, '[^0-9]', '', 'g'))
    WHERE documento <> '';
