-- Catalog identity belongs to a store network (grupo_loja), not the whole
-- company. Existing canonical rows are copied once for each group that uses
-- them; store mappings, prices, transfers and people links are then redirected.

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM transferencia t
        JOIN loja origem ON origem.id_empresa = t.id_empresa AND origem.id = t.id_loja_origem
        JOIN loja destino ON destino.id_empresa = t.id_empresa AND destino.id = t.id_loja_destino
        WHERE origem.id_grupo <> destino.id_grupo
    ) THEN
        RAISE EXCEPTION 'Group catalog migration stopped: existing transfers cross store groups; resolve those transfers first';
    END IF;
END $$;

ALTER TABLE produto ADD COLUMN id_grupo UUID;
ALTER TABLE produto_codigo_barras ADD COLUMN id_grupo UUID;
ALTER TABLE produto_loja_mapeamento ADD COLUMN id_grupo UUID;
ALTER TABLE produto_preco_loja ADD COLUMN id_grupo UUID;
ALTER TABLE cliente ADD COLUMN id_grupo UUID;
ALTER TABLE cliente_loja_mapeamento ADD COLUMN id_grupo UUID;
ALTER TABLE funcionario ADD COLUMN id_grupo UUID;
ALTER TABLE funcionario_loja_mapeamento ADD COLUMN id_grupo UUID;
ALTER TABLE operador_caixa ADD COLUMN id_grupo UUID;
ALTER TABLE transferencia ADD COLUMN id_grupo UUID;
ALTER TABLE transferencia_item ADD COLUMN id_grupo UUID;

UPDATE produto_loja_mapeamento m
SET id_grupo = l.id_grupo
FROM loja l
WHERE l.id_empresa = m.id_empresa AND l.id = m.id_loja;

UPDATE produto_preco_loja p
SET id_grupo = l.id_grupo
FROM loja l
WHERE l.id_empresa = p.id_empresa AND l.id = p.id_loja;

UPDATE cliente_loja_mapeamento m
SET id_grupo = l.id_grupo
FROM loja l
WHERE l.id_empresa = m.id_empresa AND l.id = m.id_loja;

UPDATE funcionario_loja_mapeamento m
SET id_grupo = l.id_grupo
FROM loja l
WHERE l.id_empresa = m.id_empresa AND l.id = m.id_loja;

UPDATE operador_caixa o
SET id_grupo = l.id_grupo
FROM loja l
WHERE l.id_empresa = o.id_empresa AND l.id = o.id_loja;

UPDATE transferencia t
SET id_grupo = l.id_grupo
FROM loja l
WHERE l.id_empresa = t.id_empresa AND l.id = t.id_loja_origem;

UPDATE transferencia_item i
SET id_grupo = t.id_grupo
FROM transferencia t
WHERE t.id_empresa = i.id_empresa AND t.id = i.id_transferencia;

CREATE TABLE storelink_migration_group_product_map (
    id_empresa UUID NOT NULL,
    id_grupo UUID NOT NULL,
    id_original UUID NOT NULL,
    id_produto UUID NOT NULL,
    PRIMARY KEY (id_empresa, id_grupo, id_original),
    UNIQUE (id_empresa, id_grupo, id_produto)
);

CREATE TEMP TABLE group_product_refs AS
    SELECT m.id_empresa, m.id_produto, m.id_grupo
    FROM produto_loja_mapeamento m
    UNION
    SELECT p.id_empresa, p.id_produto, p.id_grupo
    FROM produto_preco_loja p
    UNION
    SELECT i.id_empresa, i.id_produto, t.id_grupo
    FROM transferencia_item i
    JOIN transferencia t ON t.id_empresa = i.id_empresa AND t.id = i.id_transferencia
    WHERE i.id_produto IS NOT NULL
    UNION
    SELECT msg.id_empresa,
           CASE WHEN (msg.payload #>> '{produto,id_produto}') ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
                THEN (msg.payload #>> '{produto,id_produto}')::UUID END,
           s.id_grupo
    FROM mensagem msg
    JOIN loja s ON s.id_empresa = msg.id_empresa AND s.id = msg.id_loja_origem
    WHERE msg.operation IN ('product.created', 'product.updated', 'product.disabled')
      AND (msg.payload #>> '{produto,id_produto}') ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$';

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM produto p
        WHERE NOT EXISTS (
            SELECT 1 FROM group_product_refs r
            WHERE r.id_empresa = p.id_empresa AND r.id_produto = p.id
        )
        AND (SELECT COUNT(*) FROM grupo_loja g WHERE g.id_empresa = p.id_empresa) <> 1
    ) THEN
        RAISE EXCEPTION 'Group catalog migration stopped: an unused product belongs to a company with zero or multiple groups; assign it to a group before retrying';
    END IF;
END $$;

INSERT INTO group_product_refs (id_empresa, id_produto, id_grupo)
SELECT p.id_empresa, p.id, (array_agg(g.id))[1]
FROM produto p
JOIN grupo_loja g ON g.id_empresa = p.id_empresa
WHERE NOT EXISTS (
    SELECT 1 FROM group_product_refs r
    WHERE r.id_empresa = p.id_empresa AND r.id_produto = p.id
)
GROUP BY p.id_empresa, p.id
HAVING COUNT(g.id) = 1;

INSERT INTO storelink_migration_group_product_map (id_empresa, id_grupo, id_original, id_produto)
SELECT DISTINCT id_empresa, id_grupo, id_produto, id_produto
FROM group_product_refs;

DROP INDEX IF EXISTS ux_produto_empresa_sku;
DROP INDEX IF EXISTS ux_produto_codigo_barras_tenant_value;
DROP INDEX IF EXISTS ux_produto_codigo_barras_primary;

ALTER TABLE produto_codigo_barras DROP CONSTRAINT IF EXISTS fk_produto_codigo_barras_produto;
ALTER TABLE produto_loja_mapeamento DROP CONSTRAINT IF EXISTS fk_produto_loja_mapeamento_produto;
ALTER TABLE produto_preco_loja DROP CONSTRAINT IF EXISTS fk_produto_preco_loja_produto;
ALTER TABLE transferencia_item DROP CONSTRAINT IF EXISTS fk_transferencia_item_produto;

UPDATE produto p
SET id_grupo = groups.primary_group
FROM (
    SELECT id_empresa, id_original, (array_agg(id_grupo ORDER BY id_grupo))[1] AS primary_group
    FROM storelink_migration_group_product_map
    GROUP BY id_empresa, id_original
) groups
WHERE groups.id_empresa = p.id_empresa AND groups.id_original = p.id;

ALTER TABLE produto ALTER COLUMN id_grupo SET NOT NULL;
ALTER TABLE produto DROP CONSTRAINT produto_pkey;
ALTER TABLE produto DROP CONSTRAINT ux_produto_id_tenant;
ALTER TABLE produto ADD CONSTRAINT pk_produto_group PRIMARY KEY (id, id_grupo);
ALTER TABLE produto ADD CONSTRAINT ux_produto_group_id UNIQUE (id_empresa, id_grupo, id);

INSERT INTO produto
SELECT (jsonb_populate_record(NULL::produto,
        to_jsonb(p) || jsonb_build_object('id', pm.id_produto, 'id_grupo', pm.id_grupo))).*
FROM produto p
JOIN storelink_migration_group_product_map pm
  ON pm.id_empresa = p.id_empresa AND pm.id_original = p.id
WHERE pm.id_grupo <> (
    SELECT (array_agg(pm2.id_grupo ORDER BY pm2.id_grupo))[1] FROM storelink_migration_group_product_map pm2
    WHERE pm2.id_empresa = pm.id_empresa AND pm2.id_original = pm.id_original
);

UPDATE produto_codigo_barras b
SET id_grupo = groups.primary_group
FROM (
    SELECT id_empresa, id_original, (array_agg(id_grupo ORDER BY id_grupo))[1] AS primary_group
    FROM storelink_migration_group_product_map
    GROUP BY id_empresa, id_original
) groups
WHERE groups.id_empresa = b.id_empresa AND groups.id_original = b.id_produto;

INSERT INTO produto_codigo_barras
SELECT (jsonb_populate_record(NULL::produto_codigo_barras,
        to_jsonb(b) || jsonb_build_object('id', gen_random_uuid(), 'id_grupo', pm.id_grupo, 'id_produto', pm.id_produto))).*
FROM produto_codigo_barras b
JOIN storelink_migration_group_product_map pm
  ON pm.id_empresa = b.id_empresa AND pm.id_original = b.id_produto
WHERE pm.id_grupo <> (
    SELECT (array_agg(pm2.id_grupo ORDER BY pm2.id_grupo))[1] FROM storelink_migration_group_product_map pm2
    WHERE pm2.id_empresa = pm.id_empresa AND pm2.id_original = pm.id_original
);

ALTER TABLE produto_codigo_barras ALTER COLUMN id_grupo SET NOT NULL;

CREATE TABLE storelink_migration_group_customer_map (
    id_empresa UUID NOT NULL,
    id_grupo UUID NOT NULL,
    id_original UUID NOT NULL,
    id_cliente UUID NOT NULL,
    PRIMARY KEY (id_empresa, id_grupo, id_original),
    UNIQUE (id_empresa, id_grupo, id_cliente)
);

CREATE TEMP TABLE group_customer_refs AS
    SELECT m.id_empresa, m.id_cliente, m.id_grupo FROM cliente_loja_mapeamento m;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM cliente c
        WHERE NOT EXISTS (SELECT 1 FROM group_customer_refs r WHERE r.id_empresa = c.id_empresa AND r.id_cliente = c.id)
          AND (SELECT COUNT(*) FROM grupo_loja g WHERE g.id_empresa = c.id_empresa) <> 1
    ) THEN
        RAISE EXCEPTION 'Group catalog migration stopped: an unmapped customer belongs to a company with zero or multiple groups; assign it to a group before retrying';
    END IF;
END $$;

INSERT INTO group_customer_refs (id_empresa, id_cliente, id_grupo)
SELECT c.id_empresa, c.id, (array_agg(g.id))[1]
FROM cliente c
JOIN grupo_loja g ON g.id_empresa = c.id_empresa
WHERE NOT EXISTS (SELECT 1 FROM group_customer_refs r WHERE r.id_empresa = c.id_empresa AND r.id_cliente = c.id)
GROUP BY c.id_empresa, c.id
HAVING COUNT(g.id) = 1;

INSERT INTO storelink_migration_group_customer_map (id_empresa, id_grupo, id_original, id_cliente)
SELECT DISTINCT id_empresa, id_grupo, id_cliente, id_cliente
FROM group_customer_refs;

DROP INDEX IF EXISTS ux_cliente_empresa_documento;
ALTER TABLE cliente_loja_mapeamento DROP CONSTRAINT IF EXISTS fk_cliente_mapeamento_cliente;
UPDATE cliente c
SET id_grupo = groups.primary_group
FROM (
    SELECT id_empresa, id_original, (array_agg(id_grupo ORDER BY id_grupo))[1] AS primary_group
    FROM storelink_migration_group_customer_map
    GROUP BY id_empresa, id_original
) groups
WHERE groups.id_empresa = c.id_empresa AND groups.id_original = c.id;

ALTER TABLE cliente ALTER COLUMN id_grupo SET NOT NULL;
ALTER TABLE cliente DROP CONSTRAINT cliente_pkey;
ALTER TABLE cliente DROP CONSTRAINT ux_cliente_id_tenant;
ALTER TABLE cliente ADD CONSTRAINT pk_cliente_group PRIMARY KEY (id, id_grupo);
ALTER TABLE cliente ADD CONSTRAINT ux_cliente_group_id UNIQUE (id_empresa, id_grupo, id);

INSERT INTO cliente
SELECT (jsonb_populate_record(NULL::cliente,
        to_jsonb(c) || jsonb_build_object('id', cm.id_cliente, 'id_grupo', cm.id_grupo))).*
FROM cliente c
JOIN storelink_migration_group_customer_map cm
  ON cm.id_empresa = c.id_empresa AND cm.id_original = c.id
WHERE cm.id_grupo <> (
    SELECT (array_agg(cm2.id_grupo ORDER BY cm2.id_grupo))[1] FROM storelink_migration_group_customer_map cm2
    WHERE cm2.id_empresa = cm.id_empresa AND cm2.id_original = cm.id_original
);

CREATE TABLE storelink_migration_group_employee_map (
    id_empresa UUID NOT NULL,
    id_grupo UUID NOT NULL,
    id_original UUID NOT NULL,
    id_funcionario UUID NOT NULL,
    PRIMARY KEY (id_empresa, id_grupo, id_original),
    UNIQUE (id_empresa, id_grupo, id_funcionario)
);

CREATE TEMP TABLE group_employee_refs AS
    SELECT m.id_empresa, m.id_funcionario, m.id_grupo FROM funcionario_loja_mapeamento m
    UNION
    SELECT o.id_empresa, o.id_funcionario, o.id_grupo FROM operador_caixa o;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM funcionario e
        WHERE NOT EXISTS (SELECT 1 FROM group_employee_refs r WHERE r.id_empresa = e.id_empresa AND r.id_funcionario = e.id)
          AND (SELECT COUNT(*) FROM grupo_loja g WHERE g.id_empresa = e.id_empresa) <> 1
    ) THEN
        RAISE EXCEPTION 'Group catalog migration stopped: an unmapped employee belongs to a company with zero or multiple groups; assign it to a group before retrying';
    END IF;
END $$;

INSERT INTO group_employee_refs (id_empresa, id_funcionario, id_grupo)
SELECT e.id_empresa, e.id, (array_agg(g.id))[1]
FROM funcionario e
JOIN grupo_loja g ON g.id_empresa = e.id_empresa
WHERE NOT EXISTS (SELECT 1 FROM group_employee_refs r WHERE r.id_empresa = e.id_empresa AND r.id_funcionario = e.id)
GROUP BY e.id_empresa, e.id
HAVING COUNT(g.id) = 1;

INSERT INTO storelink_migration_group_employee_map (id_empresa, id_grupo, id_original, id_funcionario)
SELECT DISTINCT id_empresa, id_grupo, id_funcionario, id_funcionario
FROM group_employee_refs;

DROP INDEX IF EXISTS ux_funcionario_empresa_documento;
ALTER TABLE funcionario_loja_mapeamento DROP CONSTRAINT IF EXISTS fk_funcionario_mapeamento_funcionario;
ALTER TABLE operador_caixa DROP CONSTRAINT IF EXISTS fk_operador_caixa_funcionario;
UPDATE funcionario e
SET id_grupo = groups.primary_group
FROM (
    SELECT id_empresa, id_original, (array_agg(id_grupo ORDER BY id_grupo))[1] AS primary_group
    FROM storelink_migration_group_employee_map
    GROUP BY id_empresa, id_original
) groups
WHERE groups.id_empresa = e.id_empresa AND groups.id_original = e.id;

ALTER TABLE funcionario ALTER COLUMN id_grupo SET NOT NULL;
ALTER TABLE funcionario DROP CONSTRAINT funcionario_pkey;
ALTER TABLE funcionario DROP CONSTRAINT ux_funcionario_id_tenant;
ALTER TABLE funcionario ADD CONSTRAINT pk_funcionario_group PRIMARY KEY (id, id_grupo);
ALTER TABLE funcionario ADD CONSTRAINT ux_funcionario_group_id UNIQUE (id_empresa, id_grupo, id);

INSERT INTO funcionario
SELECT (jsonb_populate_record(NULL::funcionario,
        to_jsonb(e) || jsonb_build_object('id', em.id_funcionario, 'id_grupo', em.id_grupo))).*
FROM funcionario e
JOIN storelink_migration_group_employee_map em
  ON em.id_empresa = e.id_empresa AND em.id_original = e.id
WHERE em.id_grupo <> (
    SELECT (array_agg(em2.id_grupo ORDER BY em2.id_grupo))[1] FROM storelink_migration_group_employee_map em2
    WHERE em2.id_empresa = em.id_empresa AND em2.id_original = em.id_original
);

ALTER TABLE loja
    ADD CONSTRAINT ux_loja_group_scope UNIQUE (id_empresa, id_grupo, id);

ALTER TABLE produto ADD CONSTRAINT fk_produto_group FOREIGN KEY (id_empresa, id_grupo)
    REFERENCES grupo_loja(id_empresa, id) ON DELETE CASCADE;
CREATE UNIQUE INDEX ux_produto_group_sku ON produto(id_empresa, id_grupo, UPPER(sku))
    WHERE sku IS NOT NULL AND BTRIM(sku) <> '';
CREATE INDEX idx_produto_group_nome ON produto(id_empresa, id_grupo, nome, id);

ALTER TABLE produto_codigo_barras DROP CONSTRAINT produto_codigo_barras_pkey;
ALTER TABLE produto_codigo_barras ADD CONSTRAINT pk_produto_codigo_barras_group PRIMARY KEY (id, id_grupo);
ALTER TABLE produto_codigo_barras ADD CONSTRAINT fk_produto_codigo_barras_group FOREIGN KEY (id_empresa, id_grupo)
    REFERENCES grupo_loja(id_empresa, id) ON DELETE CASCADE;
ALTER TABLE produto_codigo_barras ADD CONSTRAINT fk_produto_codigo_barras_product_group
    FOREIGN KEY (id_empresa, id_grupo, id_produto) REFERENCES produto(id_empresa, id_grupo, id) ON DELETE CASCADE;
CREATE UNIQUE INDEX ux_produto_codigo_barras_group_value
    ON produto_codigo_barras(id_empresa, id_grupo, codigo_barras_normalizado) WHERE ativo = TRUE;
CREATE UNIQUE INDEX ux_produto_codigo_barras_group_primary
    ON produto_codigo_barras(id_empresa, id_grupo, id_produto) WHERE principal = TRUE AND ativo = TRUE;
CREATE INDEX idx_produto_codigo_barras_group_product
    ON produto_codigo_barras(id_empresa, id_grupo, id_produto, ativo);

ALTER TABLE produto_loja_mapeamento ALTER COLUMN id_grupo SET NOT NULL;
ALTER TABLE produto_loja_mapeamento DROP CONSTRAINT produto_loja_mapeamento_pkey;
ALTER TABLE produto_loja_mapeamento ADD CONSTRAINT pk_produto_loja_mapeamento_group PRIMARY KEY (id, id_grupo);
ALTER TABLE produto_loja_mapeamento ADD CONSTRAINT fk_produto_mapping_group FOREIGN KEY (id_empresa, id_grupo)
    REFERENCES grupo_loja(id_empresa, id) ON DELETE CASCADE;
ALTER TABLE produto_loja_mapeamento ADD CONSTRAINT fk_produto_mapping_store_group
    FOREIGN KEY (id_empresa, id_grupo, id_loja) REFERENCES loja(id_empresa, id_grupo, id) ON DELETE CASCADE;
ALTER TABLE produto_loja_mapeamento ADD CONSTRAINT fk_produto_mapping_product_group
    FOREIGN KEY (id_empresa, id_grupo, id_produto) REFERENCES produto(id_empresa, id_grupo, id) ON DELETE CASCADE;
CREATE INDEX idx_produto_loja_mapeamento_group_product ON produto_loja_mapeamento(id_empresa, id_grupo, id_produto);

ALTER TABLE produto_preco_loja ALTER COLUMN id_grupo SET NOT NULL;
ALTER TABLE produto_preco_loja DROP CONSTRAINT produto_preco_loja_pkey;
ALTER TABLE produto_preco_loja ADD CONSTRAINT pk_produto_preco_loja_group PRIMARY KEY (id, id_grupo);
ALTER TABLE produto_preco_loja DROP CONSTRAINT unq_produto_preco_loja_tier;
ALTER TABLE produto_preco_loja ADD CONSTRAINT fk_produto_price_group FOREIGN KEY (id_empresa, id_grupo)
    REFERENCES grupo_loja(id_empresa, id) ON DELETE CASCADE;
ALTER TABLE produto_preco_loja ADD CONSTRAINT fk_produto_price_store_group
    FOREIGN KEY (id_empresa, id_grupo, id_loja) REFERENCES loja(id_empresa, id_grupo, id) ON DELETE CASCADE;
ALTER TABLE produto_preco_loja ADD CONSTRAINT fk_produto_price_product_group
    FOREIGN KEY (id_empresa, id_grupo, id_produto) REFERENCES produto(id_empresa, id_grupo, id) ON DELETE CASCADE;
ALTER TABLE produto_preco_loja ADD CONSTRAINT unq_produto_preco_loja_tier
    UNIQUE (id_empresa, id_grupo, id_loja, id_produto, tipo, quantity);
CREATE INDEX idx_produto_preco_loja_group_product ON produto_preco_loja(id_empresa, id_grupo, id_loja, id_produto, ativo);

ALTER TABLE cliente ADD CONSTRAINT fk_cliente_group FOREIGN KEY (id_empresa, id_grupo)
    REFERENCES grupo_loja(id_empresa, id) ON DELETE CASCADE;
CREATE UNIQUE INDEX ux_cliente_group_documento ON cliente(id_empresa, id_grupo, regexp_replace(documento, '[^0-9]', '', 'g'))
    WHERE documento <> '';
CREATE INDEX idx_cliente_group_nome ON cliente(id_empresa, id_grupo, nome, id);

ALTER TABLE cliente_loja_mapeamento ALTER COLUMN id_grupo SET NOT NULL;
ALTER TABLE cliente_loja_mapeamento DROP CONSTRAINT cliente_loja_mapeamento_pkey;
ALTER TABLE cliente_loja_mapeamento ADD CONSTRAINT pk_cliente_loja_mapeamento_group PRIMARY KEY (id, id_grupo);
ALTER TABLE cliente_loja_mapeamento ADD CONSTRAINT fk_cliente_mapping_group FOREIGN KEY (id_empresa, id_grupo)
    REFERENCES grupo_loja(id_empresa, id) ON DELETE CASCADE;
ALTER TABLE cliente_loja_mapeamento ADD CONSTRAINT fk_cliente_mapping_store_group
    FOREIGN KEY (id_empresa, id_grupo, id_loja) REFERENCES loja(id_empresa, id_grupo, id) ON DELETE CASCADE;
ALTER TABLE cliente_loja_mapeamento ADD CONSTRAINT fk_cliente_mapping_customer_group
    FOREIGN KEY (id_empresa, id_grupo, id_cliente) REFERENCES cliente(id_empresa, id_grupo, id) ON DELETE CASCADE;
CREATE INDEX idx_cliente_loja_group_cliente ON cliente_loja_mapeamento(id_empresa, id_grupo, id_cliente);

ALTER TABLE funcionario ADD CONSTRAINT fk_funcionario_group FOREIGN KEY (id_empresa, id_grupo)
    REFERENCES grupo_loja(id_empresa, id) ON DELETE CASCADE;
CREATE UNIQUE INDEX ux_funcionario_group_documento ON funcionario(id_empresa, id_grupo, regexp_replace(documento, '[^0-9]', '', 'g'))
    WHERE documento <> '';
CREATE INDEX idx_funcionario_group_nome ON funcionario(id_empresa, id_grupo, nome, id);

ALTER TABLE funcionario_loja_mapeamento ALTER COLUMN id_grupo SET NOT NULL;
ALTER TABLE funcionario_loja_mapeamento DROP CONSTRAINT funcionario_loja_mapeamento_pkey;
ALTER TABLE funcionario_loja_mapeamento ADD CONSTRAINT pk_funcionario_loja_mapeamento_group PRIMARY KEY (id, id_grupo);
ALTER TABLE funcionario_loja_mapeamento ADD CONSTRAINT fk_funcionario_mapping_group FOREIGN KEY (id_empresa, id_grupo)
    REFERENCES grupo_loja(id_empresa, id) ON DELETE CASCADE;
ALTER TABLE funcionario_loja_mapeamento ADD CONSTRAINT fk_funcionario_mapping_store_group
    FOREIGN KEY (id_empresa, id_grupo, id_loja) REFERENCES loja(id_empresa, id_grupo, id) ON DELETE CASCADE;
ALTER TABLE funcionario_loja_mapeamento ADD CONSTRAINT fk_funcionario_mapping_employee_group
    FOREIGN KEY (id_empresa, id_grupo, id_funcionario) REFERENCES funcionario(id_empresa, id_grupo, id) ON DELETE CASCADE;
CREATE INDEX idx_funcionario_loja_group_funcionario ON funcionario_loja_mapeamento(id_empresa, id_grupo, id_funcionario);

ALTER TABLE operador_caixa ALTER COLUMN id_grupo SET NOT NULL;
ALTER TABLE operador_caixa DROP CONSTRAINT operador_caixa_pkey;
ALTER TABLE operador_caixa ADD CONSTRAINT pk_operador_caixa_group PRIMARY KEY (id, id_grupo);
ALTER TABLE operador_caixa ADD CONSTRAINT fk_operador_caixa_group FOREIGN KEY (id_empresa, id_grupo)
    REFERENCES grupo_loja(id_empresa, id) ON DELETE CASCADE;
ALTER TABLE operador_caixa ADD CONSTRAINT fk_operador_caixa_store_group
    FOREIGN KEY (id_empresa, id_grupo, id_loja) REFERENCES loja(id_empresa, id_grupo, id) ON DELETE CASCADE;
ALTER TABLE operador_caixa ADD CONSTRAINT fk_operador_caixa_employee_group
    FOREIGN KEY (id_empresa, id_grupo, id_funcionario) REFERENCES funcionario(id_empresa, id_grupo, id) ON DELETE CASCADE;
DROP INDEX IF EXISTS ux_operador_caixa_loja_codigo_normalizado;
CREATE UNIQUE INDEX ux_operador_caixa_loja_codigo_normalizado
    ON operador_caixa(id_empresa, id_grupo, id_loja, UPPER(BTRIM(codigo_local)));
CREATE INDEX idx_operador_caixa_group_employee ON operador_caixa(id_empresa, id_grupo, id_funcionario);

ALTER TABLE transferencia ALTER COLUMN id_grupo SET NOT NULL;
ALTER TABLE transferencia DROP CONSTRAINT transferencia_pkey;
ALTER TABLE transferencia ADD CONSTRAINT pk_transferencia_group PRIMARY KEY (id, id_grupo);
ALTER TABLE transferencia ADD CONSTRAINT ux_transferencia_group_id UNIQUE (id_empresa, id_grupo, id);
ALTER TABLE transferencia ADD CONSTRAINT fk_transferencia_group FOREIGN KEY (id_empresa, id_grupo)
    REFERENCES grupo_loja(id_empresa, id) ON DELETE CASCADE;
ALTER TABLE transferencia ADD CONSTRAINT fk_transferencia_origin_group
    FOREIGN KEY (id_empresa, id_grupo, id_loja_origem) REFERENCES loja(id_empresa, id_grupo, id) ON DELETE RESTRICT;
ALTER TABLE transferencia ADD CONSTRAINT fk_transferencia_destination_group
    FOREIGN KEY (id_empresa, id_grupo, id_loja_destino) REFERENCES loja(id_empresa, id_grupo, id) ON DELETE RESTRICT;

ALTER TABLE transferencia_item ALTER COLUMN id_grupo SET NOT NULL;
ALTER TABLE transferencia_item DROP CONSTRAINT transferencia_item_pkey;
ALTER TABLE transferencia_item ADD CONSTRAINT pk_transferencia_item_group PRIMARY KEY (id, id_grupo);
ALTER TABLE transferencia_item ADD CONSTRAINT fk_transfer_item_group FOREIGN KEY (id_empresa, id_grupo)
    REFERENCES grupo_loja(id_empresa, id) ON DELETE CASCADE;
ALTER TABLE transferencia_item ADD CONSTRAINT fk_transfer_item_transfer_group
    FOREIGN KEY (id_empresa, id_grupo, id_transferencia) REFERENCES transferencia(id_empresa, id_grupo, id) ON DELETE CASCADE;
ALTER TABLE transferencia_item ADD CONSTRAINT fk_transfer_item_product_group
    FOREIGN KEY (id_empresa, id_grupo, id_produto) REFERENCES produto(id_empresa, id_grupo, id) ON DELETE RESTRICT;

DROP TABLE group_employee_refs;
DROP TABLE group_customer_refs;
DROP TABLE group_product_refs;

-- Product IDs stay stable; their composite database identity now includes the
-- group. Existing queued payloads therefore remain valid without rewriting.
