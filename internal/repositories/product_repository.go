package repositories

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/phsoftwares/storelink-backend/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type IProductRepository interface {
	ProductBarcode(string) (models.ProductBarcode, bool, error)
	ProductBarcodeForProduct(string, uuid.UUID) (models.ProductBarcode, bool, error)
	ProductBySKU(string) (models.Product, bool, error)
	ProductsByIDs([]uuid.UUID) ([]models.Product, error)
	ProductIDsOwnedByAnotherCompany([]uuid.UUID) ([]uuid.UUID, error)
	ProductsBySKUs([]string) ([]models.Product, error)
	NextProductCanonicalCode() (string, error)
	ProductBarcodesByNormalizedValues([]string) ([]models.ProductBarcode, error)
	ProductMapping(uuid.UUID, string) (models.ProductStoreMapping, bool, error)
	ProductMappingsByLocalCodes(uuid.UUID, []string) ([]models.ProductStoreMapping, error)
	ProductMappingsByProductIDs(uuid.UUID, []uuid.UUID) ([]models.ProductStoreMapping, error)
	ProductMappingForProduct(uuid.UUID, uuid.UUID) (models.ProductStoreMapping, bool, error)
	ProductBarcodesForProducts([]uuid.UUID) ([]models.ProductBarcode, error)
	ReassignProductBarcodes([]string, uuid.UUID) error
	RemoveProductMappings(uuid.UUID, []uuid.UUID) error
	MoveProductMapping(uuid.UUID, uuid.UUID, string) error
	UpsertProducts([]models.Product) error
	UpsertProductMappings([]models.ProductStoreMapping) error
	UpsertProductBarcodes([]models.ProductBarcode) error
	ClearPrimaryProductBarcodes([]uuid.UUID) error
	UpsertProductPrices([]models.ProductPrice) error
	ReplaceProductPrices(uuid.UUID, uuid.UUID, []models.ProductPrice) error
	ReplaceProductPricesBatch(uuid.UUID, []uuid.UUID, []models.ProductPrice) error
}

// Product rows carry many catalog attributes. PostgreSQL's extended protocol
// accepts at most 65,535 bind parameters, so 500 products keep each product
// statement safely below that limit while the API can still receive batches of
// 1,000 products. Auxiliary tables have fewer columns and can use larger
// database batches without hitting the protocol limit.
const (
	productRepositoryBatchSize    = 500
	productRepositoryAuxBatchSize = 1000
)

var _ IProductRepository = (*TenantRepository)(nil)

func (r *TenantRepository) ProductBarcode(normalized string) (models.ProductBarcode, bool, error) {
	var value models.ProductBarcode
	err := r.db.Where("id_empresa = ? AND codigo_barras_normalizado = ? AND ativo = TRUE", r.company, normalized).First(&value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return value, false, nil
	}
	return value, err == nil, err
}

func (r *TenantRepository) ProductBarcodeForProduct(normalized string, productID uuid.UUID) (models.ProductBarcode, bool, error) {
	var value models.ProductBarcode
	err := r.db.Where("id_empresa = ? AND id_produto = ? AND codigo_barras_normalizado = ?", r.company, productID, normalized).First(&value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return value, false, nil
	}
	return value, err == nil, err
}

func (r *TenantRepository) ProductBySKU(normalized string) (models.Product, bool, error) {
	var value models.Product
	err := r.db.Where("id_empresa = ? AND UPPER(BTRIM(sku)) = ?", r.company, normalized).First(&value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return value, false, nil
	}
	return value, err == nil, err
}

func (r *TenantRepository) ProductsByIDs(ids []uuid.UUID) ([]models.Product, error) {
	var values []models.Product
	if len(ids) == 0 {
		return values, nil
	}
	err := r.db.Where("id_empresa = ? AND id IN ?", r.company, ids).Find(&values).Error
	return values, err
}

// ProductIDsOwnedByAnotherCompany checks explicit canonical IDs without
// returning any data from the other tenant.
func (r *TenantRepository) ProductIDsOwnedByAnotherCompany(ids []uuid.UUID) ([]uuid.UUID, error) {
	var foreignIDs []uuid.UUID
	if len(ids) == 0 {
		return foreignIDs, nil
	}
	err := r.db.Model(&models.Product{}).
		Where("id IN ? AND id_empresa <> ?", ids, r.company).
		Pluck("id", &foreignIDs).Error
	return foreignIDs, err
}

func (r *TenantRepository) ProductsBySKUs(skus []string) ([]models.Product, error) {
	var values []models.Product
	if len(skus) == 0 {
		return values, nil
	}
	err := r.db.Where("id_empresa = ? AND UPPER(BTRIM(sku)) IN ?", r.company, skus).Find(&values).Error
	return values, err
}

// NextProductCanonicalCode returns the next six-digit numeric code available
// in the tenant-wide product namespace. The caller still checks the value
// against the current store mapping and the in-memory batch because a batch
// may reserve more than one code before it commits.
func (r *TenantRepository) NextProductCanonicalCode() (string, error) {
	var maximum int64
	err := r.db.Model(&models.Product{}).
		Where("id_empresa = ? AND sku ~ '^[0-9]{1,6}$'", r.company).
		Select("COALESCE(MAX(CAST(sku AS BIGINT)), 0)").
		Scan(&maximum).Error
	if err != nil {
		return "", err
	}
	if maximum >= 999999 {
		return "", nil
	}
	return fmt.Sprintf("%06d", maximum+1), nil
}

func (r *TenantRepository) ProductBarcodesByNormalizedValues(values []string) ([]models.ProductBarcode, error) {
	var rows []models.ProductBarcode
	if len(values) == 0 {
		return rows, nil
	}
	err := r.db.Where("id_empresa = ? AND codigo_barras_normalizado IN ?", r.company, values).Find(&rows).Error
	return rows, err
}

func (r *TenantRepository) ProductMapping(storeID uuid.UUID, localCode string) (models.ProductStoreMapping, bool, error) {
	var value models.ProductStoreMapping
	err := r.db.Where("id_empresa = ? AND id_loja = ? AND UPPER(BTRIM(codigo_local)) = UPPER(BTRIM(?))", r.company, storeID, localCode).First(&value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return value, false, nil
	}
	return value, err == nil, err
}

func (r *TenantRepository) ProductMappingsByLocalCodes(storeID uuid.UUID, localCodes []string) ([]models.ProductStoreMapping, error) {
	var values []models.ProductStoreMapping
	if len(localCodes) == 0 {
		return values, nil
	}
	err := r.db.Where("id_empresa = ? AND id_loja = ? AND UPPER(BTRIM(codigo_local)) IN ?", r.company, storeID, localCodes).Find(&values).Error
	return values, err
}

func (r *TenantRepository) ProductMappingsByProductIDs(storeID uuid.UUID, productIDs []uuid.UUID) ([]models.ProductStoreMapping, error) {
	var values []models.ProductStoreMapping
	if len(productIDs) == 0 {
		return values, nil
	}
	err := r.db.Where("id_empresa = ? AND id_loja = ? AND id_produto IN ?", r.company, storeID, productIDs).Find(&values).Error
	return values, err
}

func (r *TenantRepository) ProductMappingForProduct(storeID, productID uuid.UUID) (models.ProductStoreMapping, bool, error) {
	var value models.ProductStoreMapping
	err := r.db.Where("id_empresa = ? AND id_loja = ? AND id_produto = ?", r.company, storeID, productID).First(&value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return value, false, nil
	}
	return value, err == nil, err
}

// MoveProductMapping changes only the local code of one canonical product in
// one store. It is intentionally performed in the caller's transaction: the
// product identity, occupied-code check and mapping update therefore commit
// together with the publication that caused the remap.
func (r *TenantRepository) MoveProductMapping(storeID, productID uuid.UUID, localCode string) error {
	localCode = strings.TrimSpace(localCode)
	if localCode == "" {
		return models.Error(400, "localCodeCandidate e obrigatorio")
	}
	var occupied models.ProductStoreMapping
	err := r.db.Where("id_empresa = ? AND id_loja = ? AND UPPER(BTRIM(codigo_local)) = UPPER(BTRIM(?))", r.company, storeID, localCode).First(&occupied).Error
	if err == nil && occupied.ProductID != productID {
		return models.Error(409, "PRODUCT_LOCAL_CODE_OCCUPIED: o codigo local candidato ja pertence a outro produto")
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	var mapping models.ProductStoreMapping
	err = r.db.Where("id_empresa = ? AND id_loja = ? AND id_produto = ?", r.company, storeID, productID).First(&mapping).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		mapping = models.ProductStoreMapping{Tenant: models.NewTenant(r.company), StoreID: storeID, ProductID: productID, LocalCode: localCode}
		return r.db.Create(&mapping).Error
	}
	if err != nil {
		return err
	}
	mapping.LocalCode = localCode
	return r.db.Save(&mapping).Error
}

// ProductMappingsForCatalog returns the local view of the canonical catalog
// for one store. Pagination is applied before loading the related snapshots so
// a full load cannot materialize the whole tenant in memory.
func (r *TenantRepository) ProductMappingsForCatalog(storeID uuid.UUID, offset, limit int) ([]models.ProductStoreMapping, int64, error) {
	var total int64
	countQuery := r.db.Model(&models.ProductStoreMapping{}).
		Where("id_empresa = ? AND id_loja = ?", r.company, storeID)
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var values []models.ProductStoreMapping
	query := r.db.Where("id_empresa = ? AND id_loja = ?", r.company, storeID).
		Order("codigo_local, id_produto").Offset(offset).Limit(limit).Find(&values)
	return values, total, query.Error
}

// ProductCatalogSourceStoreID returns the matrix as the canonical catalog
// source whenever the group has one. A filial can publish products that do not
// exist in the matrix yet, but it must not become the source for the next
// complete load merely because it temporarily has more mappings. Falling back
// to mapping coverage is useful for legacy groups without an explicit matrix;
// the remaining ordering keeps retries deterministic.
func (r *TenantRepository) ProductCatalogSourceStoreID(groupID uuid.UUID) (uuid.UUID, error) {
	var store models.Store
	err := r.db.Where("id_empresa = ? AND id_grupo = ? AND ativo = TRUE", r.company, groupID).
		Order("CASE WHEN UPPER(tipo) IN ('MATRIZ', 'MATRIX') THEN 0 ELSE 1 END").
		Order("(SELECT COUNT(*) FROM produto_loja_mapeamento plm WHERE plm.id_empresa = loja.id_empresa AND plm.id_loja = loja.id) DESC").
		Order("data_hora_criacao ASC").
		Order("codigo, id").
		First(&store).Error
	return store.ID, err
}

func (r *TenantRepository) ProductBarcodesForProducts(productIDs []uuid.UUID) ([]models.ProductBarcode, error) {
	var values []models.ProductBarcode
	if len(productIDs) == 0 {
		return values, nil
	}
	err := r.db.Where("id_empresa = ? AND id_produto IN ?", r.company, productIDs).
		Order("id_produto, principal DESC, codigo_barras_normalizado").Find(&values).Error
	return values, err
}

// ReassignProductBarcodes is used only while reconciling an authoritative
// matrix snapshot. An EAN may have been attached to two stale global products
// by older filial loads; the matrix primary EAN chooses the survivor and the
// other EAN rows are moved to it in the same transaction. Primary flags are
// cleared before changing the product id so the partial unique index cannot
// reject the merge.
func (r *TenantRepository) ReassignProductBarcodes(values []string, winner uuid.UUID) error {
	if len(values) == 0 || winner == uuid.Nil {
		return nil
	}
	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.ToUpper(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	if len(normalized) == 0 {
		return nil
	}
	where := r.db.Model(&models.ProductBarcode{}).
		Where("id_empresa = ? AND ativo = TRUE AND codigo_barras_normalizado IN ? AND id_produto <> ?", r.company, normalized, winner)
	if err := where.Update("principal", false).Error; err != nil {
		return err
	}
	return where.Update("id_produto", winner).Error
}

// RemoveProductMappings removes only the stale mappings of products that were
// merged by the current authoritative snapshot. The product rows themselves
// are deliberately retained for audit/history; no stock or movement data is
// deleted by this reconciliation.
func (r *TenantRepository) RemoveProductMappings(storeID uuid.UUID, productIDs []uuid.UUID) error {
	if storeID == uuid.Nil || len(productIDs) == 0 {
		return nil
	}
	return r.db.Where("id_empresa = ? AND id_loja = ? AND id_produto IN ?", r.company, storeID, productIDs).
		Delete(&models.ProductStoreMapping{}).Error
}

func (r *TenantRepository) ProductPricesForStoreProducts(storeID uuid.UUID, productIDs []uuid.UUID) ([]models.ProductPrice, error) {
	var values []models.ProductPrice
	if len(productIDs) == 0 {
		return values, nil
	}
	err := r.db.Where("id_empresa = ? AND id_loja = ? AND id_produto IN ?", r.company, storeID, productIDs).
		Order("id_produto, tipo, quantity").Find(&values).Error
	return values, err
}

func (r *TenantRepository) UpsertProductPrices(prices []models.ProductPrice) error {
	if len(prices) == 0 {
		return nil
	}
	conflict := clause.OnConflict{
		Columns:   []clause.Column{{Name: "id_empresa"}, {Name: "id_loja"}, {Name: "id_produto"}, {Name: "tipo"}, {Name: "quantity"}},
		DoUpdates: clause.AssignmentColumns([]string{"amount", "ativo", "valid_from", "data_hora_atualizacao"}),
	}
	return r.db.Clauses(conflict).CreateInBatches(&prices, productRepositoryAuxBatchSize).Error
}

// ReplaceProductPrices applies the authoritative price snapshot atomically
// for one store/product. It is deliberately scoped by tenant, store and
// product so a malformed integration request can never clear another store's
// prices.
func (r *TenantRepository) ReplaceProductPrices(storeID, productID uuid.UUID, prices []models.ProductPrice) error {
	if err := r.db.Where("id_empresa = ? AND id_loja = ? AND id_produto = ?", r.company, storeID, productID).
		Delete(&models.ProductPrice{}).Error; err != nil {
		return err
	}
	return r.UpsertProductPrices(prices)
}

// ReplaceProductPricesBatch applies authoritative price snapshots for many
// products in one statement. The caller owns the surrounding transaction, so
// an interrupted initial load cannot leave half of one batch with stale tiers.
// The product list is explicit and tenant-scoped to prevent a malformed
// request from clearing prices from another store or company.
func (r *TenantRepository) ReplaceProductPricesBatch(storeID uuid.UUID, productIDs []uuid.UUID, prices []models.ProductPrice) error {
	if len(productIDs) == 0 {
		return nil
	}
	if err := r.db.Where("id_empresa = ? AND id_loja = ? AND id_produto IN ?", r.company, storeID, productIDs).
		Delete(&models.ProductPrice{}).Error; err != nil {
		return err
	}
	return r.UpsertProductPrices(prices)
}

func (r *TenantRepository) UpsertProducts(products []models.Product) error {
	if len(products) == 0 {
		return nil
	}
	conflict := clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"sku", "estoque_minimo", "margem_minima", "flag_sis", "nome", "descricao", "unidade", "marca", "codigo_marca", "ncm", "cest",
			"data_cadastro", "aplicacao", "origem",
			"classificacao_fiscal", "classe_tp", "cst", "codigo_ms",
			"principio_ativo", "nome_laboratorio", "codigo_fornecedor",
			"nome_fornecedor", "cnpj_fornecedor", "nome_fantasia_fornecedor",
			"tipo", "farmacia_controlado", "farmacia_apresentacao", "farmacia_registro_medicamento",
			"apresentacao", "codigo_original", "csosn", "usa_lote", "usa_balanca",
			"codigo_receita", "codigo_barra_novartis",
			"situacao_tributaria", "piscofins", "incidencia_piscofins", "iat", "ippt",
			"ind_cfop_venda_dentro", "codigo_classificacao_tributaria", "cst_ibs", "cst_cbs",
			"aliquota_ibs", "aliquota_cbs", "aliquota_efetiva_ibs", "aliquota_efetiva_cbs",
			"obs_reforma_tributaria", "grupo_nome", "subgrupo_nome", "ativo", "tracks_serial",
			"preco_venda", "preco_venda1", "preco_venda2", "preco_venda3", "preco_venda4", "preco_venda5",
			"preco_promocao", "margem_desconto", "aliquota", "qtde_embalagem",
			"desconto_preco_venda", "farmacia_pmc", "data_promocao", "fim_promocao",
			"usa_tabela_preco", "preco_custo_anterior", "preco_venda_anterior",
			"preco_farma_pop", "aliquota_pis", "aliquota_cofins", "pfcp", "pfcpst",
			"pmargem1", "pmargem2", "pmargem3", "pmargem4", "pmargem5",
			"usa_grade",
			"data_hora_atualizacao",
		}),
		Where: clause.Where{Exprs: []clause.Expression{
			clause.Eq{Column: clause.Column{Table: "produto", Name: "id_empresa"}, Value: clause.Column{Table: "excluded", Name: "id_empresa"}},
		}},
	}
	result := r.db.Clauses(conflict).CreateInBatches(&products, productRepositoryBatchSize)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != int64(len(products)) {
		return models.Error(409, "PRODUCT_TENANT_CONFLICT: productId pertence a outra empresa")
	}
	return nil
}

func (r *TenantRepository) UpsertProductMappings(mappings []models.ProductStoreMapping) error {
	if len(mappings) == 0 {
		return nil
	}
	conflict := clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"codigo_local", "codigo_grupo_local", "grupo_nome", "codigo_subgrupo_local", "subgrupo_nome", "data_hora_atualizacao",
		}),
	}
	return r.db.Clauses(conflict).CreateInBatches(&mappings, productRepositoryAuxBatchSize).Error
}

func (r *TenantRepository) UpsertProductBarcodes(barcodes []models.ProductBarcode) error {
	if len(barcodes) == 0 {
		return nil
	}
	conflict := clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"codigo_barras", "codigo_barras_normalizado", "principal", "ativo", "fraction", "c00055", "data_hora_atualizacao",
		}),
	}
	return r.db.Clauses(conflict).CreateInBatches(&barcodes, productRepositoryAuxBatchSize).Error
}

// DeleteProductC00055Except replaces the authoritative C000055 membership
// without deleting the product barcode itself. The latter remains available
// for EAN identity and can still be the primary C000025 barcode.
func (r *TenantRepository) DeleteProductC00055Except(productID uuid.UUID, keep []string) error {
	query := r.db.Model(&models.ProductBarcode{}).
		Where("id_empresa = ? AND id_produto = ? AND c00055 = TRUE", r.company, productID)
	if len(keep) == 0 {
		return query.Delete(&models.ProductBarcode{}).Error
	}
	return query.Where("codigo_barras_normalizado NOT IN ?", keep).
		Delete(&models.ProductBarcode{}).Error
}

func (r *TenantRepository) ClearPrimaryProductBarcodes(productIDs []uuid.UUID) error {
	if len(productIDs) == 0 {
		return nil
	}
	return r.db.Model(&models.ProductBarcode{}).
		Where("id_empresa = ? AND id_produto IN ? AND principal = TRUE AND ativo = TRUE", r.company, productIDs).
		Update("principal", false).Error
}
