package repositories

import (
	"context"
	"errors"
	"reflect"
	"time"

	"github.com/google/uuid"
	"github.com/phsoftwares/storelink-backend/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct{ db *gorm.DB }
type TenantRepository struct {
	db      *gorm.DB
	company uuid.UUID
}

type IRepository interface {
	IAuthRepository
	ICompanyRepository
	IAgentIdentityRepository
	IAgentRepository
	IGroupRepository
	IStoreRepository
	IAuditRepository
	IMessageReadRepository
	ISyncJobRepository
	ITransferRepository
	Scoped(context.Context, uuid.UUID) *TenantRepository
	Atomic(context.Context, uuid.UUID, func(*TenantRepository) error) error
}

var _ IRepository = (*Repository)(nil)

func (r *TenantRepository) CompanyID() uuid.UUID { return r.company }

type Filter struct {
	Page, PageSize                            int
	Q, Status, Operation                      string
	GroupID, StoreID, OriginID, DestinationID uuid.UUID
	From, To                                  *time.Time
	Where                                     map[string]interface{}
}

func (f Filter) Pagination() (int, int) {
	p, n := f.Page, f.PageSize
	if p < 1 {
		p = 1
	}
	if n < 1 {
		n = 25
	}
	if n > 100 {
		n = 100
	}
	return p, n
}
func New(db *gorm.DB) *Repository { return &Repository{db: db} }
func (r *Repository) Scoped(ctx context.Context, company uuid.UUID) *TenantRepository {
	return &TenantRepository{db: r.db.WithContext(ctx), company: company}
}
func (r *Repository) Atomic(ctx context.Context, company uuid.UUID, fn func(*TenantRepository) error) error {
	if company == uuid.Nil {
		return models.Error(403, "Empresa obrigatória")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return fn(&TenantRepository{db: tx, company: company}) })
}
func (r *TenantRepository) base(out interface{}) *gorm.DB {
	q := r.db.Model(out)
	if r.company == uuid.Nil {
		return q.Where("1 = 0")
	}
	return q.Where("id_empresa = ?", r.company)
}
func (r *TenantRepository) Find(out interface{}, id uuid.UUID, lock bool) error {
	q := r.base(out).Where("id = ?", id)
	if lock {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	return q.First(out).Error
}
func (r *TenantRepository) Create(out models.ScopedEntity) error {
	if r.company == uuid.Nil {
		return models.Error(403, "Empresa obrigatória")
	}
	out.SetCompany(r.company)
	return r.db.Create(out).Error
}

func (r *TenantRepository) CreateMany(out interface{}, batchSize int) error {
	if r.company == uuid.Nil {
		return models.Error(403, "Empresa obrigat\u00f3ria")
	}
	value := reflect.ValueOf(out)
	if value.Kind() != reflect.Ptr || value.IsNil() || value.Elem().Kind() != reflect.Slice {
		return errors.New("CreateMany requires a pointer to a slice")
	}
	rows := value.Elem()
	for index := 0; index < rows.Len(); index++ {
		row := rows.Index(index)
		if row.Kind() != reflect.Ptr {
			row = row.Addr()
		}
		entity, ok := row.Interface().(models.ScopedEntity)
		if !ok {
			return errors.New("CreateMany slice values must be tenant scoped entities")
		}
		entity.SetCompany(r.company)
	}
	if batchSize < 1 {
		batchSize = 500
	}
	return r.db.CreateInBatches(out, batchSize).Error
}
func (r *TenantRepository) InsertUnique(out models.ScopedEntity) (bool, error) {
	if r.company == uuid.Nil {
		return false, models.Error(403, "Empresa obrigatória")
	}
	out.SetCompany(r.company)
	q := r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(out)
	return q.RowsAffected > 0, q.Error
}
func (r *TenantRepository) Save(out models.ScopedEntity, id uuid.UUID) error {
	out.SetCompany(r.company)
	q := r.base(out).Where("id = ?", id).Select("*").Updates(out)
	if q.Error != nil {
		return q.Error
	}
	if q.RowsAffected == 0 {
		// PostgreSQL triggers and dialects configured to report changed rows can
		// return zero for an idempotent update. Distinguish that from a missing
		// row before exposing a false 404 to callers.
		var count int64
		if err := r.base(out).Where("id = ?", id).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return gorm.ErrRecordNotFound
		}
	}
	return nil
}
func (r *TenantRepository) Update(out interface{}, where map[string]interface{}, values map[string]interface{}) error {
	return r.base(out).Where(where).Updates(values).Error
}
func (r *TenantRepository) Count(out interface{}, where map[string]interface{}) (int64, error) {
	var n int64
	err := r.base(out).Where(where).Count(&n).Error
	return n, err
}
func (r *TenantRepository) List(out interface{}, f Filter) (models.Page, error) {
	p, n := f.Pagination()
	q := r.base(out)
	for k, v := range f.Where {
		q = q.Where(map[string]interface{}{k: v})
	}
	q = applyFilter(q, out, f, r.company)
	var count int64
	if e := q.Count(&count).Error; e != nil {
		return models.Page{}, e
	}
	e := q.Order("data_hora_criacao DESC, id DESC").Limit(n).Offset((p - 1) * n).Find(out).Error
	return models.Page{Items: out, Total: count, Page: p, PageSize: n}, e
}
func (r *TenantRepository) All(out interface{}, where map[string]interface{}) error {
	return r.base(out).Where(where).Order("data_hora_criacao ASC, id ASC").Find(out).Error
}

func applyFilter(q *gorm.DB, out interface{}, f Filter, company uuid.UUID) *gorm.DB {
	if f.From != nil {
		q = q.Where("data_hora_criacao >= ?", f.From)
	}
	if f.To != nil {
		q = q.Where("data_hora_criacao < ?", f.To)
	}
	switch out.(type) {
	case *[]models.Group:
		if f.Q != "" {
			q = q.Where("nome ILIKE ?", "%"+f.Q+"%")
		}
	case *[]models.Agent:
		if f.StoreID != uuid.Nil {
			q = q.Where("id_loja = ?", f.StoreID)
		}
	case *[]models.Message, *[]models.Transfer, *[]models.SyncJob:
		if f.Status != "" {
			q = q.Where("status = ?", f.Status)
		}
		if f.OriginID != uuid.Nil {
			q = q.Where("id_loja_origem = ?", f.OriginID)
		}
		if f.DestinationID != uuid.Nil {
			q = q.Where("id_loja_destino = ?", f.DestinationID)
		}
		if f.StoreID != uuid.Nil {
			q = q.Where("(id_loja_origem = ? OR id_loja_destino = ?)", f.StoreID, f.StoreID)
		}
		if f.GroupID != uuid.Nil {
			q = q.Where("id_loja_origem IN (SELECT id FROM loja WHERE id_empresa = ? AND id_grupo = ?)", company, f.GroupID)
		}
	case *[]models.IntegrationError:
		if f.Status != "" {
			q = q.Where("status = ?", f.Status)
		}
		if f.StoreID != uuid.Nil {
			q = q.Where("id_loja = ?", f.StoreID)
		}
	case *[]models.Audit:
		if f.StoreID != uuid.Nil {
			q = q.Where("id_loja = ?", f.StoreID)
		}
	}
	switch out.(type) {
	case *[]models.Message, *[]models.IntegrationError:
		if f.Operation != "" {
			q = q.Where("operation = ?", f.Operation)
		}
		if f.Q != "" {
			q = q.Where("operation ILIKE ?", "%"+f.Q+"%")
		}
	case *[]models.Transfer:
		if f.Q != "" {
			q = q.Where("numero ILIKE ?", "%"+f.Q+"%")
		}
	}
	return q
}

func IsNotFound(err error) bool { return errors.Is(err, gorm.ErrRecordNotFound) }
