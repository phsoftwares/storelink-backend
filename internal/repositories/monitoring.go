package repositories

import (
	"github.com/google/uuid"
	"github.com/phsoftwares/storelink-backend/internal/models"
	"gorm.io/gorm"
	"time"
)

type IMonitoringRepository interface {
	Dashboard(Filter, time.Duration) (models.Dashboard, error)
}

var _ IMonitoringRepository = (*TenantRepository)(nil)

func (r *TenantRepository) Dashboard(f Filter, offlineAfter time.Duration) (models.Dashboard, error) {
	d := models.Dashboard{Companies: 1}
	groupQ := r.base(&models.Group{})
	if f.GroupID != uuid.Nil {
		groupQ = groupQ.Where("id = ?", f.GroupID)
	}
	if e := groupQ.Count(&d.Groups).Error; e != nil {
		return d, e
	}
	f.PageSize = 100
	f.Page = 1
	for {
		page, e := r.Stores(f, offlineAfter)
		if e != nil {
			return d, e
		}
		d.Stores = page.Total
		for _, s := range page.Items.([]models.StoreView) {
			switch s.Status {
			case "ONLINE":
				d.Online++
			case "DEGRADED":
				d.Degraded++
			default:
				d.Offline++
			}
			d.Suppliers += int64(s.Suppliers)
		}
		if int64(f.Page*100) >= page.Total {
			break
		}
		f.Page++
	}
	var err error
	if d.Products, err = r.catalogCount(&models.Product{}, "produto_loja_mapeamento", "id_produto", f); err != nil {
		return d, err
	}
	if d.Customers, err = r.catalogCount(&models.Customer{}, "cliente_loja_mapeamento", "id_cliente", f); err != nil {
		return d, err
	}
	if d.Employees, err = r.catalogCount(&models.Employee{}, "funcionario_loja_mapeamento", "id_funcionario", f); err != nil {
		return d, err
	}
	queries := []struct {
		model     interface{}
		target    *int64
		condition string
		args      []interface{}
	}{
		{&models.Message{}, &d.MessagesToday, "data_hora_criacao >= ?", []interface{}{time.Now().UTC().Truncate(24 * time.Hour)}},
		{&models.Message{}, &d.PendingMessages, "status IN ('PENDING','RETRY','PROCESSING')", nil},
		{&models.Message{}, &d.FailedMessages, "status IN ('FAILED','DEAD_LETTER')", nil},
		{&models.Transfer{}, &d.PendingTransfers, "status IN ('PENDING','SYNCED')", nil},
		{&models.Transfer{}, &d.FailedTransfers, "status = 'FAILED'", nil},
		{&models.SyncJob{}, &d.RunningJobs, "status IN ('PENDING','RUNNING')", nil},
		{&models.SyncJob{}, &d.CompletedJobs, "status = 'COMPLETED'", nil},
		{&models.SyncJob{}, &d.FailedJobs, "status IN ('FAILED','COMPLETED_WITH_ERRORS')", nil},
	}
	for _, item := range queries {
		q := r.base(item.model).Where(item.condition, item.args...)
		q = r.monitorScope(q, f)
		if e := q.Count(item.target).Error; e != nil {
			return d, e
		}
	}
	return d, nil
}

func (r *TenantRepository) catalogCount(model interface{}, mappingTable, entityColumn string, f Filter) (int64, error) {
	if f.GroupID == uuid.Nil && f.StoreID == uuid.Nil {
		var count int64
		err := r.base(model).Count(&count).Error
		return count, err
	}
	var count int64
	query := r.db.Table(mappingTable+" AS mapping").
		Select("COUNT(DISTINCT mapping."+entityColumn+")").
		Where("mapping.id_empresa = ?", r.company)
	if f.StoreID != uuid.Nil {
		query = query.Where("mapping.id_loja = ?", f.StoreID)
	}
	if f.GroupID != uuid.Nil {
		query = query.Joins("JOIN loja AS store ON store.id_empresa = mapping.id_empresa AND store.id = mapping.id_loja").
			Where("store.id_grupo = ?", f.GroupID)
	}
	err := query.Scan(&count).Error
	return count, err
}

func (r *TenantRepository) monitorScope(q *gorm.DB, f Filter) *gorm.DB {
	if f.GroupID != uuid.Nil {
		q = q.Where("id_loja_origem IN (SELECT id FROM loja WHERE id_empresa = ? AND id_grupo = ?)", r.company, f.GroupID)
	}
	if f.StoreID != uuid.Nil {
		q = q.Where("(id_loja_origem = ? OR id_loja_destino = ?)", f.StoreID, f.StoreID)
	}
	return q
}
