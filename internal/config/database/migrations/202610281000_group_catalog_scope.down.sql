-- Rolling back this migration merges separate customer networks back into one
-- company-wide catalog. Refuse the rollback if post-migration data cannot be
-- merged without dropping a group's catalog identities.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM produto p
        WHERE NOT EXISTS (
            SELECT 1 FROM storelink_migration_group_product_map m
            WHERE m.id_empresa = p.id_empresa AND m.id_grupo = p.id_grupo AND m.id_produto = p.id
        )
    ) OR EXISTS (
        SELECT 1 FROM cliente c
        WHERE NOT EXISTS (
            SELECT 1 FROM storelink_migration_group_customer_map m
            WHERE m.id_empresa = c.id_empresa AND m.id_grupo = c.id_grupo AND m.id_cliente = c.id
        )
    ) OR EXISTS (
        SELECT 1 FROM funcionario e
        WHERE NOT EXISTS (
            SELECT 1 FROM storelink_migration_group_employee_map m
            WHERE m.id_empresa = e.id_empresa AND m.id_grupo = e.id_grupo AND m.id_funcionario = e.id
        )
    ) THEN
        RAISE EXCEPTION 'Group catalog rollback stopped: new group-scoped records exist; export and merge them manually before rollback';
    END IF;

    IF EXISTS (
        SELECT 1 FROM produto p
        JOIN storelink_migration_group_product_map m ON m.id_empresa = p.id_empresa AND m.id_grupo = p.id_grupo AND m.id_produto = p.id
        JOIN produto original ON original.id_empresa = m.id_empresa AND original.id = m.id_original
          AND original.id_grupo = (
              SELECT (array_agg(pm.id_grupo ORDER BY pm.id_grupo))[1]
              FROM storelink_migration_group_product_map pm
              WHERE pm.id_empresa = m.id_empresa AND pm.id_original = m.id_original
          )
        WHERE m.id_grupo <> original.id_grupo
          AND (to_jsonb(p) - 'id' - 'id_grupo') <> (to_jsonb(original) - 'id' - 'id_grupo')
    ) OR EXISTS (
        SELECT 1 FROM cliente c
        JOIN storelink_migration_group_customer_map m ON m.id_empresa = c.id_empresa AND m.id_grupo = c.id_grupo AND m.id_cliente = c.id
        JOIN cliente original ON original.id_empresa = m.id_empresa AND original.id = m.id_original
          AND original.id_grupo = (
              SELECT (array_agg(cm.id_grupo ORDER BY cm.id_grupo))[1]
              FROM storelink_migration_group_customer_map cm
              WHERE cm.id_empresa = m.id_empresa AND cm.id_original = m.id_original
          )
        WHERE m.id_grupo <> original.id_grupo
          AND (to_jsonb(c) - 'id' - 'id_grupo') <> (to_jsonb(original) - 'id' - 'id_grupo')
    ) OR EXISTS (
        SELECT 1 FROM funcionario e
        JOIN storelink_migration_group_employee_map m ON m.id_empresa = e.id_empresa AND m.id_grupo = e.id_grupo AND m.id_funcionario = e.id
        JOIN funcionario original ON original.id_empresa = m.id_empresa AND original.id = m.id_original
          AND original.id_grupo = (
              SELECT (array_agg(em.id_grupo ORDER BY em.id_grupo))[1]
              FROM storelink_migration_group_employee_map em
              WHERE em.id_empresa = m.id_empresa AND em.id_original = m.id_original
          )
        WHERE m.id_grupo <> original.id_grupo
          AND (to_jsonb(e) - 'id' - 'id_grupo') <> (to_jsonb(original) - 'id' - 'id_grupo')
    ) THEN
        RAISE EXCEPTION 'Group catalog rollback stopped: a copied product, customer or employee changed after migration; merge those records manually';
    END IF;

    IF EXISTS (
        SELECT 1 FROM produto p
        JOIN storelink_migration_group_product_map m ON m.id_empresa = p.id_empresa AND m.id_grupo = p.id_grupo AND m.id_produto = p.id
        WHERE p.sku IS NOT NULL AND BTRIM(p.sku) <> ''
        GROUP BY p.id_empresa, UPPER(p.sku)
        HAVING COUNT(DISTINCT m.id_original) > 1
    ) OR EXISTS (
        SELECT 1 FROM produto_codigo_barras b
        JOIN storelink_migration_group_product_map m ON m.id_empresa = b.id_empresa AND m.id_grupo = b.id_grupo AND m.id_produto = b.id_produto
        WHERE b.ativo = TRUE
        GROUP BY b.id_empresa, b.codigo_barras_normalizado
        HAVING COUNT(DISTINCT m.id_original) > 1
    ) OR EXISTS (
        SELECT 1 FROM cliente c
        JOIN storelink_migration_group_customer_map m ON m.id_empresa = c.id_empresa AND m.id_grupo = c.id_grupo AND m.id_cliente = c.id
        WHERE c.documento <> ''
        GROUP BY c.id_empresa, regexp_replace(c.documento, '[^0-9]', '', 'g')
        HAVING COUNT(DISTINCT m.id_original) > 1
    ) OR EXISTS (
        SELECT 1 FROM funcionario e
        JOIN storelink_migration_group_employee_map m ON m.id_empresa = e.id_empresa AND m.id_grupo = e.id_grupo AND m.id_funcionario = e.id
        WHERE e.documento <> ''
        GROUP BY e.id_empresa, regexp_replace(e.documento, '[^0-9]', '', 'g')
        HAVING COUNT(DISTINCT m.id_original) > 1
    ) OR EXISTS (
        SELECT 1
        FROM storelink_migration_group_product_map pm
        JOIN produto_codigo_barras copy ON copy.id_empresa = pm.id_empresa AND copy.id_grupo = pm.id_grupo AND copy.id_produto = pm.id_produto
        WHERE pm.id_grupo <> (
            SELECT (array_agg(pm2.id_grupo ORDER BY pm2.id_grupo))[1]
            FROM storelink_migration_group_product_map pm2
            WHERE pm2.id_empresa = pm.id_empresa AND pm2.id_original = pm.id_original
        )
        AND NOT EXISTS (
            SELECT 1 FROM produto_codigo_barras primary_barcode
            WHERE primary_barcode.id_empresa = pm.id_empresa
              AND primary_barcode.id_grupo = (
                  SELECT (array_agg(pm2.id_grupo ORDER BY pm2.id_grupo))[1]
                  FROM storelink_migration_group_product_map pm2
                  WHERE pm2.id_empresa = pm.id_empresa AND pm2.id_original = pm.id_original
              )
              AND primary_barcode.id_produto = pm.id_original
              AND primary_barcode.codigo_barras = copy.codigo_barras
              AND primary_barcode.codigo_barras_normalizado = copy.codigo_barras_normalizado
              AND primary_barcode.principal = copy.principal
              AND primary_barcode.ativo = copy.ativo
              AND primary_barcode.fraction IS NOT DISTINCT FROM copy.fraction
              AND primary_barcode.c00055 = copy.c00055
        )
    ) OR EXISTS (
        SELECT 1
        FROM storelink_migration_group_product_map pm
        JOIN produto_codigo_barras primary_barcode
          ON primary_barcode.id_empresa = pm.id_empresa
         AND primary_barcode.id_grupo = (
             SELECT (array_agg(pm2.id_grupo ORDER BY pm2.id_grupo))[1]
             FROM storelink_migration_group_product_map pm2
             WHERE pm2.id_empresa = pm.id_empresa AND pm2.id_original = pm.id_original
         )
         AND primary_barcode.id_produto = pm.id_original
        WHERE pm.id_grupo <> primary_barcode.id_grupo
        AND NOT EXISTS (
            SELECT 1 FROM produto_codigo_barras copy
            WHERE copy.id_empresa = pm.id_empresa AND copy.id_grupo = pm.id_grupo
              AND copy.id_produto = pm.id_produto
              AND copy.codigo_barras = primary_barcode.codigo_barras
              AND copy.codigo_barras_normalizado = primary_barcode.codigo_barras_normalizado
              AND copy.principal = primary_barcode.principal
              AND copy.ativo = primary_barcode.ativo
              AND copy.fraction IS NOT DISTINCT FROM primary_barcode.fraction
              AND copy.c00055 = primary_barcode.c00055
        )
    ) THEN
        RAISE EXCEPTION 'Group catalog rollback stopped: group catalogs now contain conflicting SKU, barcode or document identities';
    END IF;
END $$;

ALTER TABLE produto_codigo_barras DROP CONSTRAINT IF EXISTS fk_produto_codigo_barras_product_group;
ALTER TABLE produto_codigo_barras DROP CONSTRAINT IF EXISTS fk_produto_codigo_barras_group;
ALTER TABLE produto_loja_mapeamento DROP CONSTRAINT IF EXISTS fk_produto_mapping_product_group;
ALTER TABLE produto_loja_mapeamento DROP CONSTRAINT IF EXISTS fk_produto_mapping_store_group;
ALTER TABLE produto_loja_mapeamento DROP CONSTRAINT IF EXISTS fk_produto_mapping_group;
ALTER TABLE produto_preco_loja DROP CONSTRAINT IF EXISTS fk_produto_price_product_group;
ALTER TABLE produto_preco_loja DROP CONSTRAINT IF EXISTS fk_produto_price_store_group;
ALTER TABLE produto_preco_loja DROP CONSTRAINT IF EXISTS fk_produto_price_group;
ALTER TABLE cliente_loja_mapeamento DROP CONSTRAINT IF EXISTS fk_cliente_mapping_customer_group;
ALTER TABLE cliente_loja_mapeamento DROP CONSTRAINT IF EXISTS fk_cliente_mapping_store_group;
ALTER TABLE cliente_loja_mapeamento DROP CONSTRAINT IF EXISTS fk_cliente_mapping_group;
ALTER TABLE funcionario_loja_mapeamento DROP CONSTRAINT IF EXISTS fk_funcionario_mapping_employee_group;
ALTER TABLE funcionario_loja_mapeamento DROP CONSTRAINT IF EXISTS fk_funcionario_mapping_store_group;
ALTER TABLE funcionario_loja_mapeamento DROP CONSTRAINT IF EXISTS fk_funcionario_mapping_group;
ALTER TABLE operador_caixa DROP CONSTRAINT IF EXISTS fk_operador_caixa_employee_group;
ALTER TABLE operador_caixa DROP CONSTRAINT IF EXISTS fk_operador_caixa_store_group;
ALTER TABLE operador_caixa DROP CONSTRAINT IF EXISTS fk_operador_caixa_group;
ALTER TABLE transferencia_item DROP CONSTRAINT IF EXISTS fk_transfer_item_product_group;
ALTER TABLE transferencia_item DROP CONSTRAINT IF EXISTS fk_transfer_item_transfer_group;
ALTER TABLE transferencia_item DROP CONSTRAINT IF EXISTS fk_transfer_item_group;
ALTER TABLE transferencia DROP CONSTRAINT IF EXISTS fk_transferencia_destination_group;
ALTER TABLE transferencia DROP CONSTRAINT IF EXISTS fk_transferencia_origin_group;
ALTER TABLE transferencia DROP CONSTRAINT IF EXISTS fk_transferencia_group;

UPDATE transferencia_item i
SET id_produto = pm.id_original
FROM storelink_migration_group_product_map pm
WHERE pm.id_empresa = i.id_empresa AND pm.id_grupo = i.id_grupo AND pm.id_produto = i.id_produto;
UPDATE produto_preco_loja p
SET id_produto = pm.id_original
FROM storelink_migration_group_product_map pm
WHERE pm.id_empresa = p.id_empresa AND pm.id_grupo = p.id_grupo AND pm.id_produto = p.id_produto;
UPDATE produto_loja_mapeamento p
SET id_produto = pm.id_original
FROM storelink_migration_group_product_map pm
WHERE pm.id_empresa = p.id_empresa AND pm.id_grupo = p.id_grupo AND pm.id_produto = p.id_produto;

UPDATE cliente_loja_mapeamento c
SET id_cliente = cm.id_original
FROM storelink_migration_group_customer_map cm
WHERE cm.id_empresa = c.id_empresa AND cm.id_grupo = c.id_grupo AND cm.id_cliente = c.id_cliente;
UPDATE funcionario_loja_mapeamento e
SET id_funcionario = em.id_original
FROM storelink_migration_group_employee_map em
WHERE em.id_empresa = e.id_empresa AND em.id_grupo = e.id_grupo AND em.id_funcionario = e.id_funcionario;
UPDATE operador_caixa o
SET id_funcionario = em.id_original
FROM storelink_migration_group_employee_map em
WHERE em.id_empresa = o.id_empresa AND em.id_grupo = o.id_grupo AND em.id_funcionario = o.id_funcionario;

DROP INDEX IF EXISTS ux_produto_codigo_barras_group_primary;
DROP INDEX IF EXISTS ux_produto_codigo_barras_group_value;
DELETE FROM produto_codigo_barras b
USING storelink_migration_group_product_map pm
WHERE pm.id_empresa = b.id_empresa AND pm.id_produto = b.id_produto
  AND pm.id_grupo <> (SELECT (array_agg(pm2.id_grupo ORDER BY pm2.id_grupo))[1]
                      FROM storelink_migration_group_product_map pm2
                      WHERE pm2.id_empresa = pm.id_empresa AND pm2.id_original = pm.id_original)
  AND b.id_grupo = pm.id_grupo;

DELETE FROM produto p
USING storelink_migration_group_product_map pm
WHERE pm.id_empresa = p.id_empresa AND pm.id_produto = p.id AND p.id_grupo = pm.id_grupo
  AND pm.id_grupo <> (SELECT (array_agg(pm2.id_grupo ORDER BY pm2.id_grupo))[1]
                      FROM storelink_migration_group_product_map pm2
                      WHERE pm2.id_empresa = pm.id_empresa AND pm2.id_original = pm.id_original);

DELETE FROM cliente c
USING storelink_migration_group_customer_map cm
WHERE cm.id_empresa = c.id_empresa AND cm.id_cliente = c.id AND c.id_grupo = cm.id_grupo
  AND cm.id_grupo <> (SELECT (array_agg(cm2.id_grupo ORDER BY cm2.id_grupo))[1]
                      FROM storelink_migration_group_customer_map cm2
                      WHERE cm2.id_empresa = cm.id_empresa AND cm2.id_original = cm.id_original);

DELETE FROM funcionario e
USING storelink_migration_group_employee_map em
WHERE em.id_empresa = e.id_empresa AND em.id_funcionario = e.id AND e.id_grupo = em.id_grupo
  AND em.id_grupo <> (SELECT (array_agg(em2.id_grupo ORDER BY em2.id_grupo))[1]
                      FROM storelink_migration_group_employee_map em2
                      WHERE em2.id_empresa = em.id_empresa AND em2.id_original = em.id_original);

ALTER TABLE transferencia_item DROP CONSTRAINT IF EXISTS fk_transferencia_item_produto_group;
ALTER TABLE transferencia_item DROP CONSTRAINT IF EXISTS fk_transferencia_item_product_group;
ALTER TABLE transferencia_item DROP CONSTRAINT IF EXISTS pk_transferencia_item_group;
ALTER TABLE transferencia_item ADD CONSTRAINT transferencia_item_pkey PRIMARY KEY (id);
ALTER TABLE transferencia_item DROP COLUMN id_grupo;
ALTER TABLE transferencia DROP CONSTRAINT IF EXISTS ux_transferencia_group_id;
ALTER TABLE transferencia DROP CONSTRAINT IF EXISTS pk_transferencia_group;
ALTER TABLE transferencia ADD CONSTRAINT transferencia_pkey PRIMARY KEY (id);
ALTER TABLE transferencia DROP COLUMN id_grupo;
ALTER TABLE operador_caixa DROP CONSTRAINT IF EXISTS pk_operador_caixa_group;
ALTER TABLE operador_caixa ADD CONSTRAINT operador_caixa_pkey PRIMARY KEY (id);
ALTER TABLE operador_caixa DROP COLUMN id_grupo;
ALTER TABLE funcionario_loja_mapeamento DROP CONSTRAINT IF EXISTS pk_funcionario_loja_mapeamento_group;
ALTER TABLE funcionario_loja_mapeamento ADD CONSTRAINT funcionario_loja_mapeamento_pkey PRIMARY KEY (id);
ALTER TABLE funcionario_loja_mapeamento DROP COLUMN id_grupo;
ALTER TABLE funcionario DROP CONSTRAINT IF EXISTS fk_funcionario_group;
ALTER TABLE funcionario DROP CONSTRAINT IF EXISTS ux_funcionario_group_id;
ALTER TABLE funcionario DROP CONSTRAINT IF EXISTS pk_funcionario_group;
ALTER TABLE funcionario ADD CONSTRAINT funcionario_pkey PRIMARY KEY (id);
ALTER TABLE funcionario DROP COLUMN id_grupo;
ALTER TABLE cliente_loja_mapeamento DROP CONSTRAINT IF EXISTS pk_cliente_loja_mapeamento_group;
ALTER TABLE cliente_loja_mapeamento ADD CONSTRAINT cliente_loja_mapeamento_pkey PRIMARY KEY (id);
ALTER TABLE cliente_loja_mapeamento DROP COLUMN id_grupo;
ALTER TABLE cliente DROP CONSTRAINT IF EXISTS fk_cliente_group;
ALTER TABLE cliente DROP CONSTRAINT IF EXISTS ux_cliente_group_id;
ALTER TABLE cliente DROP CONSTRAINT IF EXISTS pk_cliente_group;
ALTER TABLE cliente ADD CONSTRAINT cliente_pkey PRIMARY KEY (id);
ALTER TABLE cliente DROP COLUMN id_grupo;
ALTER TABLE produto_preco_loja DROP CONSTRAINT IF EXISTS unq_produto_preco_loja_tier;
ALTER TABLE produto_preco_loja DROP CONSTRAINT IF EXISTS pk_produto_preco_loja_group;
ALTER TABLE produto_preco_loja ADD CONSTRAINT produto_preco_loja_pkey PRIMARY KEY (id);
ALTER TABLE produto_preco_loja DROP COLUMN id_grupo;
ALTER TABLE produto_loja_mapeamento DROP CONSTRAINT IF EXISTS pk_produto_loja_mapeamento_group;
ALTER TABLE produto_loja_mapeamento ADD CONSTRAINT produto_loja_mapeamento_pkey PRIMARY KEY (id);
ALTER TABLE produto_loja_mapeamento DROP COLUMN id_grupo;
ALTER TABLE produto_codigo_barras DROP CONSTRAINT IF EXISTS pk_produto_codigo_barras_group;
ALTER TABLE produto_codigo_barras ADD CONSTRAINT produto_codigo_barras_pkey PRIMARY KEY (id);
ALTER TABLE produto_codigo_barras DROP COLUMN id_grupo;
ALTER TABLE produto DROP CONSTRAINT IF EXISTS fk_produto_group;
ALTER TABLE produto DROP CONSTRAINT IF EXISTS ux_produto_group_id;
ALTER TABLE produto DROP CONSTRAINT IF EXISTS pk_produto_group;
ALTER TABLE produto ADD CONSTRAINT produto_pkey PRIMARY KEY (id);
ALTER TABLE produto DROP COLUMN id_grupo;
ALTER TABLE loja DROP CONSTRAINT IF EXISTS ux_loja_group_scope;

ALTER TABLE produto ADD CONSTRAINT ux_produto_id_tenant UNIQUE (id_empresa, id);
ALTER TABLE cliente ADD CONSTRAINT ux_cliente_id_tenant UNIQUE (id_empresa, id);
ALTER TABLE funcionario ADD CONSTRAINT ux_funcionario_id_tenant UNIQUE (id_empresa, id);

DROP INDEX IF EXISTS ux_produto_group_sku;
DROP INDEX IF EXISTS idx_produto_group_nome;
DROP INDEX IF EXISTS idx_produto_codigo_barras_group_product;
DROP INDEX IF EXISTS idx_produto_loja_mapeamento_group_product;
DROP INDEX IF EXISTS idx_produto_preco_loja_group_product;
DROP INDEX IF EXISTS ux_cliente_group_documento;
DROP INDEX IF EXISTS idx_cliente_group_nome;
DROP INDEX IF EXISTS idx_cliente_loja_group_cliente;
DROP INDEX IF EXISTS ux_funcionario_group_documento;
DROP INDEX IF EXISTS idx_funcionario_group_nome;
DROP INDEX IF EXISTS idx_funcionario_loja_group_funcionario;
DROP INDEX IF EXISTS ux_operador_caixa_loja_codigo_normalizado;
DROP INDEX IF EXISTS idx_operador_caixa_group_employee;

ALTER TABLE produto_preco_loja ADD CONSTRAINT unq_produto_preco_loja_tier
    UNIQUE (id_empresa, id_loja, id_produto, tipo, quantity);
CREATE UNIQUE INDEX ux_produto_empresa_sku ON produto(id_empresa, UPPER(sku))
    WHERE sku IS NOT NULL AND BTRIM(sku) <> '';
CREATE UNIQUE INDEX ux_produto_codigo_barras_tenant_value
    ON produto_codigo_barras(id_empresa, codigo_barras_normalizado) WHERE ativo = TRUE;
CREATE UNIQUE INDEX ux_produto_codigo_barras_primary
    ON produto_codigo_barras(id_empresa, id_produto) WHERE principal = TRUE AND ativo = TRUE;
CREATE UNIQUE INDEX ux_cliente_empresa_documento ON cliente(id_empresa, regexp_replace(documento, '[^0-9]', '', 'g'))
    WHERE documento <> '';
CREATE UNIQUE INDEX ux_funcionario_empresa_documento ON funcionario(id_empresa, regexp_replace(documento, '[^0-9]', '', 'g'))
    WHERE documento <> '';
CREATE UNIQUE INDEX ux_operador_caixa_loja_codigo_normalizado
    ON operador_caixa(id_empresa, id_loja, UPPER(BTRIM(codigo_local)));

ALTER TABLE produto_codigo_barras ADD CONSTRAINT fk_produto_codigo_barras_produto
    FOREIGN KEY (id_empresa, id_produto) REFERENCES produto(id_empresa, id) ON DELETE CASCADE;
ALTER TABLE produto_loja_mapeamento ADD CONSTRAINT fk_produto_loja_mapeamento_produto
    FOREIGN KEY (id_empresa, id_produto) REFERENCES produto(id_empresa, id) ON DELETE CASCADE;
ALTER TABLE produto_preco_loja ADD CONSTRAINT fk_produto_preco_loja_produto
    FOREIGN KEY (id_empresa, id_produto) REFERENCES produto(id_empresa, id) ON DELETE CASCADE;
ALTER TABLE cliente_loja_mapeamento ADD CONSTRAINT fk_cliente_mapeamento_cliente
    FOREIGN KEY (id_empresa, id_cliente) REFERENCES cliente(id_empresa, id) ON DELETE CASCADE;
ALTER TABLE funcionario_loja_mapeamento ADD CONSTRAINT fk_funcionario_mapeamento_funcionario
    FOREIGN KEY (id_empresa, id_funcionario) REFERENCES funcionario(id_empresa, id) ON DELETE CASCADE;
ALTER TABLE operador_caixa ADD CONSTRAINT fk_operador_caixa_funcionario
    FOREIGN KEY (id_empresa, id_funcionario) REFERENCES funcionario(id_empresa, id) ON DELETE CASCADE;
ALTER TABLE transferencia_item ADD CONSTRAINT fk_transferencia_item_produto
    FOREIGN KEY (id_empresa, id_produto) REFERENCES produto(id_empresa, id) ON DELETE RESTRICT;

DROP TABLE storelink_migration_group_employee_map;
DROP TABLE storelink_migration_group_customer_map;
DROP TABLE storelink_migration_group_product_map;
