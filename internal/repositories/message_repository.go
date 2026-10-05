package repositories

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/phsoftwares/storelink-backend/internal/models"
	"gorm.io/gorm/clause"
)

// A product publication request can fan out to every store in the group.
// Keeping this at the publication limit avoids turning one request into many
// small INSERT statements while remaining below PostgreSQL's parameter limit.
const messageRepositoryBatchSize = 1000

type IMessageQueueRepository interface {
	Advisory(uuid.UUID) error
	AdvisoryKeys([]string) error
	Due(uuid.UUID, time.Time, int) ([]models.Message, error)
	Expired(uuid.UUID, time.Time, int) ([]models.Message, error)
	PendingJobMessages(uuid.UUID) ([]models.Message, error)
}

type IMessageReadRepository interface {
	ListMessages(context.Context, uuid.UUID, Filter) (models.Page, error)
	ListIntegrationErrors(context.Context, uuid.UUID, Filter) (models.Page, error)
}

// AcknowledgeMessagesBulk confirms direct integration messages after the
// local worker has committed the page.  The claim token is matched per row;
// if one claim is stale the transaction is rolled back instead of silently
// acknowledging only part of the page.
func (r *TenantRepository) AcknowledgeMessagesBulk(store uuid.UUID, now time.Time, claims []models.IntegrationAckItemDTO) (int, error) {
	if len(claims) == 0 {
		return 0, nil
	}

	values := make([]string, 0, len(claims))
	args := make([]interface{}, 0, len(claims)*2+5)
	ids := make([]uuid.UUID, 0, len(claims))
	for _, claim := range claims {
		values = append(values, "(?::uuid, ?::uuid)")
		args = append(args, claim.ID, claim.ClaimToken)
		ids = append(ids, claim.ID)
	}

	args = append(args, now, r.company, store, now)
	query := `WITH claims(id, claim_token) AS (VALUES ` + strings.Join(values, ",") + `)
		UPDATE mensagem AS m
		SET status = 'ACKNOWLEDGED', acknowledged_at = ?,
		    claim_token = NULL, lease_until = NULL, next_attempt_at = NULL,
		    last_error = ''
		FROM claims AS c
		WHERE m.id_empresa = ?
		  AND m.id_loja_destino = ?
		  AND m.id = c.id
		  AND m.claim_token = c.claim_token
		  AND m.status = 'PROCESSING'
		  AND m.lease_until > ?
		  AND m.id_sync_batch IS NULL
		  AND m.id_transferencia IS NULL`
	result := r.db.Exec(query, args...)
	if result.Error != nil {
		return 0, result.Error
	}
	if int(result.RowsAffected) != len(claims) {
		return 0, models.Error(409, "Um ou mais claims de mensagens estao expirados, substituidos ou nao pertencem a loja")
	}

	// Rebuild the VALUES list because the second statement must match each
	// attempt token as well.
	values = make([]string, 0, len(claims))
	args = make([]interface{}, 0, len(claims)*2+2)
	for _, claim := range claims {
		values = append(values, "(?::uuid, ?::uuid)")
		args = append(args, claim.ID, claim.ClaimToken)
	}
	args = append(args, r.company)
	attemptQuery := `WITH claims(id, claim_token) AS (VALUES ` + strings.Join(values, ",") + `)
		UPDATE mensagem_tentativa AS t
		SET status = 'ACKNOWLEDGED', finished_at = CURRENT_TIMESTAMP
		FROM claims AS c
		WHERE t.id_empresa = ?
		  AND t.id_mensagem = c.id
		  AND t.claim_token = c.claim_token
		  AND t.status = 'PROCESSING'`
	if err := r.db.Exec(attemptQuery, args...).Error; err != nil {
		return 0, err
	}

	if err := r.db.Exec(`
		UPDATE integracao_erro
		SET status = 'RESOLVED', resolved_at = CURRENT_TIMESTAMP
		WHERE id_empresa = ?
		  AND id_mensagem = ANY(?::uuid[])
		  AND status = 'OPEN'
		  AND item_key = ''`, r.company, pq.Array(ids)).Error; err != nil {
		return 0, err
	}
	return len(claims), nil
}

var _ IMessageQueueRepository = (*TenantRepository)(nil)
var _ IMessageReadRepository = (*Repository)(nil)

// MessagesByIDs is used by bulk publishers to validate idempotent retries in
// one query instead of probing the queue once per product.
func (r *TenantRepository) MessagesByIDs(ids []uuid.UUID) ([]models.Message, error) {
	if len(ids) == 0 {
		return []models.Message{}, nil
	}
	var values []models.Message
	err := r.base(&models.Message{}).Where("id IN ?", ids).Find(&values).Error
	return values, err
}

// InsertMessagesUnique inserts a publication batch atomically and ignores
// already-existing deterministic message IDs. It is intentionally scoped to
// the tenant and uses the same unique-conflict behavior as InsertUnique.
func (r *TenantRepository) InsertMessagesUnique(messages []models.Message) (int, error) {
	if len(messages) == 0 {
		return 0, nil
	}
	for index := range messages {
		messages[index].SetCompany(r.company)
	}
	result := r.db.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(&messages, messageRepositoryBatchSize)
	return int(result.RowsAffected), result.Error
}

func (r *TenantRepository) Advisory(key uuid.UUID) error {
	return r.db.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", key.String()).Error
}

func (r *TenantRepository) AdvisoryKeys(keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	keys = append([]string(nil), keys...)
	sort.Strings(keys)
	unique := keys[:0]
	for _, key := range keys {
		if key != "" && (len(unique) == 0 || unique[len(unique)-1] != key) {
			unique = append(unique, key)
		}
	}
	if len(unique) == 0 {
		return nil
	}
	return r.db.Exec("SELECT pg_advisory_xact_lock(hashtextextended(lock_key, 0)) FROM unnest(?::text[]) AS lock_keys(lock_key) ORDER BY lock_key", pq.Array(unique)).Error
}
func (r *TenantRepository) Due(store uuid.UUID, now time.Time, limit int) ([]models.Message, error) {
	return r.due(store, now, limit, "", false)
}

func (r *TenantRepository) DueOperation(store uuid.UUID, now time.Time, limit int, operation string) ([]models.Message, error) {
	return r.due(store, now, limit, operation, false)
}

func (r *TenantRepository) DueOperationCoalesced(store uuid.UUID, now time.Time, limit int, operation string) ([]models.Message, error) {
	return r.due(store, now, limit, operation, true)
}

func (r *TenantRepository) due(store uuid.UUID, now time.Time, limit int, operation string, coalesce bool) ([]models.Message, error) {
	out := []models.Message{}
	const integrationEnabled = `NOT EXISTS (
		SELECT 1
		FROM loja AS destino
		JOIN grupo_loja AS grupo ON grupo.id = destino.id_grupo AND grupo.id_empresa = destino.id_empresa
		WHERE destino.id_empresa = m.id_empresa AND destino.id = m.id_loja_destino
		AND (
			(m.operation IN ('product.created','product.updated','product.disabled','sync.products') AND NOT grupo.integrar_produtos)
			OR (m.operation IN ('product_group.created','product_group.updated','product_group.disabled','product_subgroup.created','product_subgroup.updated','product_subgroup.disabled') AND NOT (grupo.integrar_produtos OR grupo.integrar_grupos_subgrupos))
			OR (m.operation IN ('price.updated','product.update_price') AND NOT grupo.integrar_precos)
			OR (m.operation IN ('customer.created','customer.updated','customer.disabled','sync.customers') AND NOT grupo.integrar_clientes)
			OR (m.operation IN ('employee.created','employee.updated','employee.disabled','cashier_operator.created','cashier_operator.updated','cashier_operator.disabled','sync.employees') AND NOT grupo.integrar_funcionarios)
			OR (m.operation = 'sync.batch' AND m.payload->>'entity' = 'product' AND NOT grupo.integrar_produtos)
			OR (m.operation = 'sync.batch' AND m.payload->>'entity' IN ('product_group','product_subgroup') AND NOT (grupo.integrar_produtos OR grupo.integrar_grupos_subgrupos))
			OR (m.operation = 'sync.batch' AND m.payload->>'entity' = 'customer' AND NOT grupo.integrar_clientes)
			OR (m.operation = 'sync.batch' AND m.payload->>'entity' = 'employee' AND NOT grupo.integrar_funcionarios)
		)
	)`
	query := r.db.Table("mensagem AS m").Select("m.*").Where("m.id_empresa = ? AND m.id_loja_destino = ? AND m.status IN ('PENDING','RETRY') AND (m.next_attempt_at IS NULL OR m.next_attempt_at <= ?)", r.company, store, now).Where(integrationEnabled)
	if strings.TrimSpace(operation) != "" {
		if strings.HasSuffix(operation, ".*") {
			query = query.Where("m.operation LIKE ?", strings.TrimSuffix(operation, "*")+"%")
		} else {
			query = query.Where("m.operation = ?", operation)
		}
	}
	if coalesce {
		// O payload de cadastro contem o codigo local e representa o estado
		// completo do registro. Durante uma carga inicial, somente o ultimo
		// estado pendente de cada codigo precisa ser entregue.
		const identity = `COALESCE(NULLIF(m.payload->>'codigo', ''), NULLIF(m.payload->>'id', ''), m.id::text)`
		if strings.HasSuffix(operation, ".*") {
			// A carga de cadastro sempre informa o dominio (customer.*,
			// employee.* etc.). Evite o CASE por linha: alem de mais caro,
			// ele impedia o PostgreSQL de aproveitar o indice de operacao.
			operationPrefix := strings.TrimSuffix(operation, "*")
			query = query.Where(`NOT EXISTS (
				SELECT 1
				FROM mensagem AS newer
				WHERE newer.id_empresa = m.id_empresa
				  AND newer.id_loja_origem = m.id_loja_origem
				  AND newer.id_loja_destino = m.id_loja_destino
				  AND newer.status IN ('PENDING','RETRY')
				  AND (newer.next_attempt_at IS NULL OR newer.next_attempt_at <= ?)
				  AND newer.operation LIKE ?
				  AND `+identity+` = COALESCE(NULLIF(newer.payload->>'codigo', ''), NULLIF(newer.payload->>'id', ''), newer.id::text)
				  AND (newer.data_hora_criacao, newer.id) > (m.data_hora_criacao, m.id)
			)`, now, operationPrefix+"%")
		} else {
			const domain = `CASE
				WHEN m.operation LIKE 'customer.%' THEN 'customer'
				WHEN m.operation LIKE 'employee.%' THEN 'employee'
				WHEN m.operation LIKE 'cashier_operator.%' THEN 'cashier_operator'
				WHEN m.operation LIKE 'agreement.%' THEN 'agreement'
				ELSE m.operation
			END`
			query = query.Where(`NOT EXISTS (
				SELECT 1
				FROM mensagem AS newer
				WHERE newer.id_empresa = m.id_empresa
				  AND newer.id_loja_origem = m.id_loja_origem
				  AND newer.id_loja_destino = m.id_loja_destino
				  AND newer.status IN ('PENDING','RETRY')
				  AND (newer.next_attempt_at IS NULL OR newer.next_attempt_at <= ?)
				  AND `+domain+` = CASE
					WHEN newer.operation LIKE 'customer.%' THEN 'customer'
					WHEN newer.operation LIKE 'employee.%' THEN 'employee'
					WHEN newer.operation LIKE 'cashier_operator.%' THEN 'cashier_operator'
					WHEN newer.operation LIKE 'agreement.%' THEN 'agreement'
					ELSE newer.operation
				  END
				  AND `+identity+` = COALESCE(NULLIF(newer.payload->>'codigo', ''), NULLIF(newer.payload->>'id', ''), newer.id::text)
				  AND (newer.data_hora_criacao, newer.id) > (m.data_hora_criacao, m.id)
			)`, now)
		}
	}
	e := query.Order("m.data_hora_criacao,m.id").Limit(limit).Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Find(&out).Error
	return out, e
}

// AcknowledgeSuperseded marca como obsoletos os estados anteriores aos itens
// escolhidos para uma carga inicial. Faz isso depois do SELECT com lock dentro
// da mesma transacao do claim; assim uma queda antes do retorno desfaz a
// consolidacao, e uma nova alteracao criada depois do claim permanece pendente.
func (r *TenantRepository) AcknowledgeSuperseded(store uuid.UUID, now time.Time, messages []models.Message) error {
	const identity = `COALESCE(NULLIF(payload->>'codigo', ''), NULLIF(payload->>'id', ''), id::text)`
	for _, message := range messages {
		parts := strings.SplitN(message.Operation, ".", 2)
		if len(parts) != 2 {
			continue
		}
		domain := parts[0]
		if message.Operation == "cashier_operator.updated" || strings.HasPrefix(message.Operation, "cashier_operator.") {
			domain = "cashier_operator"
		}
		if message.Operation == "agreement.updated" || strings.HasPrefix(message.Operation, "agreement.") {
			domain = "agreement"
		}
		key := ""
		var payload map[string]interface{}
		if json.Unmarshal(message.Payload, &payload) == nil && payload != nil {
			if value, ok := payload["codigo"].(string); ok {
				key = strings.TrimSpace(value)
			}
			if key == "" {
				if value, ok := payload["id"].(string); ok {
					key = strings.TrimSpace(value)
				}
			}
		}
		if key == "" {
			key = message.ID.String()
		}
		if err := r.db.Exec(`
			UPDATE mensagem
			SET status = 'ACKNOWLEDGED', acknowledged_at = ?,
			    claim_token = NULL, lease_until = NULL, next_attempt_at = NULL,
			    last_error = ''
			WHERE id_empresa = ?
			  AND id_loja_origem = ?
			  AND id_loja_destino = ?
			  AND status IN ('PENDING','RETRY')
			  AND operation LIKE ?
			  AND `+identity+` = ?
			  AND (data_hora_criacao, id) < (?, ?)`,
			now, r.company, message.OriginStoreID, store, domain+".%", key,
			message.CreatedAt, message.ID).Error; err != nil {
			return err
		}
	}
	return nil
}

// AcknowledgeSupersededBulk is the batched equivalent of the historical
// per-message implementation above. It is used by the initial catalog claim
// so a page does not execute one JSON/index scan per message.
func (r *TenantRepository) AcknowledgeSupersededBulk(store uuid.UUID, now time.Time, messages []models.Message) error {
	const identity = `COALESCE(NULLIF(m.payload->>'codigo', ''), NULLIF(m.payload->>'id', ''), m.id::text)`
	placeholders := make([]string, 0, len(messages))
	args := make([]interface{}, 0, len(messages)*5+3)
	for _, message := range messages {
		parts := strings.SplitN(message.Operation, ".", 2)
		if len(parts) != 2 {
			continue
		}
		domain := parts[0]
		if strings.HasPrefix(message.Operation, "cashier_operator.") {
			domain = "cashier_operator"
		}
		if strings.HasPrefix(message.Operation, "agreement.") {
			domain = "agreement"
		}
		key := ""
		var payload map[string]interface{}
		if json.Unmarshal(message.Payload, &payload) == nil && payload != nil {
			if value, ok := payload["codigo"].(string); ok {
				key = strings.TrimSpace(value)
			}
			if key == "" {
				if value, ok := payload["id"].(string); ok {
					key = strings.TrimSpace(value)
				}
			}
		}
		if key == "" {
			key = message.ID.String()
		}
		placeholders = append(placeholders,
			"(?::uuid, ?::text, ?::text, ?::timestamptz, ?::uuid)")
		args = append(args, message.OriginStoreID, domain, key,
			message.CreatedAt, message.ID)
	}
	if len(placeholders) == 0 {
		return nil
	}
	args = append(args, now, r.company, store)
	query := `WITH escolhidos(id_loja_origem, dominio, chave, criado_em, id) AS (VALUES ` +
		strings.Join(placeholders, ",") + `)
		UPDATE mensagem AS m
		SET status = 'ACKNOWLEDGED', acknowledged_at = ?,
		    claim_token = NULL, lease_until = NULL, next_attempt_at = NULL,
		    last_error = ''
		FROM escolhidos AS e
		WHERE m.id_empresa = ?
		  AND m.id_loja_origem = e.id_loja_origem
		  AND m.id_loja_destino = ?
		  AND m.status IN ('PENDING','RETRY')
		  AND m.operation LIKE e.dominio || '.%'
		  AND ` + identity + ` = e.chave
		  AND (m.data_hora_criacao, m.id) < (e.criado_em, e.id)`
	return r.db.Exec(query, args...).Error
}

func (r *TenantRepository) Expired(store uuid.UUID, now time.Time, limit int) ([]models.Message, error) {
	out := []models.Message{}
	if limit < 1 {
		limit = 1000
	}
	e := r.base(&models.Message{}).Where("id_loja_destino = ? AND status = 'PROCESSING' AND lease_until <= ?", store, now).Order("id").Limit(limit).Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Find(&out).Error
	return out, e
}

// RenewClaims extends only claims that still belong to this destination and
// token. An expired claim is deliberately not revived: a process that stopped
// must go through the normal reclaim path instead of racing a new worker.
func (r *TenantRepository) RenewClaims(store uuid.UUID, now time.Time, lease time.Duration, items []models.IntegrationRenewItem) (int, error) {
	if len(items) == 0 {
		return 0, nil
	}
	values := make([]string, 0, len(items))
	args := make([]interface{}, 0, len(items)*2+4)
	for _, item := range items {
		values = append(values, "(?::uuid, ?::uuid)")
		args = append(args, item.ID, item.ClaimToken)
	}
	leaseUntil := now.Add(lease)
	args = append(args, leaseUntil, r.company, store, now)
	query := `WITH claims(id, token) AS (VALUES ` + strings.Join(values, ",") + `)
		UPDATE mensagem AS m
		SET lease_until = ?
		FROM claims AS c
		WHERE m.id = c.id
		  AND m.claim_token = c.token
		  AND m.id_empresa = ?
		  AND m.id_loja_destino = ?
		  AND m.status = 'PROCESSING'
		  AND m.lease_until > ?`
	result := r.db.Exec(query, args...)
	return int(result.RowsAffected), result.Error
}

func (r *TenantRepository) PendingJobMessages(job uuid.UUID) ([]models.Message, error) {
	out := []models.Message{}
	e := r.base(&models.Message{}).Where("id_sync_job = ? AND status IN ('PENDING','RETRY','PROCESSING','DEAD_LETTER')", job).Order("id").Clauses(clause.Locking{Strength: "UPDATE"}).Find(&out).Error
	return out, e
}

func (r *Repository) ListMessages(ctx context.Context, companyID uuid.UUID, filter Filter) (models.Page, error) {
	items := []models.Message{}
	return r.Scoped(ctx, companyID).List(&items, filter)
}

func (r *Repository) ListIntegrationErrors(ctx context.Context, companyID uuid.UUID, filter Filter) (models.Page, error) {
	items := []models.IntegrationError{}
	return r.Scoped(ctx, companyID).List(&items, filter)
}
