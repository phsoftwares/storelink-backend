package repositories

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/phsoftwares/storelink-backend/internal/models"
	"gorm.io/gorm"
)

type IStoreRepository interface {
	ListStores(context.Context, uuid.UUID, Filter, time.Duration) (models.Page, error)
}

var _ IStoreRepository = (*Repository)(nil)

func (r *Repository) ListStores(ctx context.Context, companyID uuid.UUID, filter Filter, offlineAfter time.Duration) (models.Page, error) {
	return r.Scoped(ctx, companyID).Stores(filter, offlineAfter)
}

func (r *TenantRepository) Stores(f Filter, offlineAfter time.Duration) (models.Page, error) {
	p, n := f.Pagination()
	cutoff := time.Now().UTC().Add(-offlineAfter)
	q := r.db.Table("loja l").
		Joins("JOIN grupo_loja g ON g.id = l.id_grupo AND g.id_empresa = l.id_empresa").
		Joins(`LEFT JOIN LATERAL (
			SELECT a.*
			FROM agente a
			WHERE a.id_empresa = l.id_empresa
			  AND a.ativo = true
			  AND (a.id_loja = l.id OR a.id_loja IS NULL)
			ORDER BY CASE WHEN a.id_loja = l.id THEN 0 ELSE 1 END, a.id
			LIMIT 1
		) a ON true`).
		Joins("LEFT JOIN LATERAL (SELECT h.* FROM heartbeat h WHERE h.id_empresa = l.id_empresa AND h.id_loja = l.id AND h.id_agente = a.id ORDER BY h.data_hora_criacao DESC, h.id DESC LIMIT 1) h ON true").
		Where("l.id_empresa = ?", r.company)
	if f.GroupID != uuid.Nil {
		q = q.Where("l.id_grupo = ?", f.GroupID)
	}
	if f.StoreID != uuid.Nil {
		q = q.Where("l.id = ?", f.StoreID)
	}
	if f.Q != "" {
		q = q.Where("(l.nome ILIKE ? OR l.codigo ILIKE ?)", "%"+f.Q+"%", "%"+f.Q+"%")
	}
	state := `CASE WHEN NOT l.ativo OR NOT g.ativo OR a.id IS NULL OR h.data_hora_criacao IS NULL OR h.data_hora_criacao < ? OR h.status = 'OFFLINE' THEN 'OFFLINE' WHEN NOT h.database_ok OR h.error_count > 0 OR h.status = 'DEGRADED' THEN 'DEGRADED' ELSE 'ONLINE' END`
	if f.Status != "" {
		q = q.Where("("+state+") = ?", cutoff, f.Status)
	}
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return models.Page{}, err
	}
	out := []models.StoreView{}
	selectSQL := `l.*, ` + state + ` AS status, h.data_hora_criacao AS last_seen, COALESCE(h.version,'') AS agent_version, COALESCE(h.products,0) AS products, COALESCE(h.customers,0) AS customers, COALESCE(h.employees,0) AS employees, COALESCE(h.suppliers,0) AS suppliers,
 (SELECT COUNT(*) FROM mensagem m WHERE m.id_empresa = l.id_empresa AND m.id_loja_destino = l.id AND m.status IN ('PENDING','RETRY','PROCESSING')) AS pending,
 (SELECT COUNT(*) FROM integracao_erro e WHERE e.id_empresa = l.id_empresa AND e.id_loja = l.id AND e.status = 'OPEN') AS errors`
	err := q.Select(selectSQL, cutoff).Order("l.nome, l.id").Limit(n).Offset((p - 1) * n).Scan(&out).Error
	return models.Page{Items: out, Total: count, Page: p, PageSize: n}, err
}

func (r *TenantRepository) ActiveStoresInGroup(groupID uuid.UUID) ([]models.Store, error) {
	var stores []models.Store
	err := r.db.Where("id_empresa = ? AND id_grupo = ? AND ativo = TRUE", r.company, groupID).
		Order("codigo, id").Find(&stores).Error
	return stores, err
}

func (r *TenantRepository) StoreByCNPJ(cnpj string) (models.Store, bool, error) {
	var store models.Store
	err := r.db.Where("id_empresa = ? AND cnpj = ?", r.company, cnpj).First(&store).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return store, false, nil
	}
	return store, err == nil, err
}
