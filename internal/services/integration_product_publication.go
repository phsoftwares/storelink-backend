package services

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/phsoftwares/storelink-backend/internal/models"
	"github.com/phsoftwares/storelink-backend/internal/repositories"
)

type IProductPublicationService interface {
	PublishProduct(context.Context, models.Identity, models.ProductPublicationDTO) (models.ProductPublicationResult, error)
	PublishProducts(context.Context, models.Identity, []models.ProductPublicationDTO) (models.ProductPublicationBatchResult, error)
	ProductCatalog(context.Context, models.Identity, int, int) (models.ProductCatalogPage, error)
}

var _ IProductPublicationService = (*Service)(nil)

type productPublicationPayload struct {
	ID                  uuid.UUID                     `json:"id"`
	Event               string                        `json:"evento"`
	Version             int64                         `json:"versao"`
	HashPayload         string                        `json:"hash_payload"`
	Product             models.ProductSnapshot        `json:"produto"`
	Prices              []models.ProductPriceSnapshot `json:"precos,omitempty"`
	PricesAuthoritative bool                          `json:"precos_autoritativos,omitempty"`
}

// publicationIdentity is the idempotency material for a publication created
// without an explicit ID. Prices are part of the product snapshot delivered
// to the stores, so changing only a price must create a new publication event
// instead of colliding with the previous event for the same product.
type publicationIdentity struct {
	Event               string                        `json:"evento"`
	Product             models.ProductSnapshot        `json:"produto"`
	Prices              []models.ProductPriceSnapshot `json:"precos,omitempty"`
	PricesAuthoritative bool                          `json:"precos_autoritativos,omitempty"`
	LocalCodeCandidate  string                        `json:"codigo_local_candidato,omitempty"`
	RequestNewLocalCode bool                          `json:"solicitar_novo_codigo_local,omitempty"`
}

func (s *Service) PublishProduct(ctx context.Context, identity models.Identity, request models.ProductPublicationDTO) (models.ProductPublicationResult, error) {
	if err := normalizeProductPublication(&request); err != nil {
		return models.ProductPublicationResult{}, err
	}
	var result models.ProductPublicationResult
	err := s.Repo.Atomic(ctx, identity.CompanyID, func(r *repositories.TenantRepository) error {
		var err error
		result, err = s.publishProductInTransaction(r, identity, request)
		return err
	})
	return result, err
}

func (s *Service) PublishProducts(ctx context.Context, identity models.Identity, requests []models.ProductPublicationDTO) (models.ProductPublicationBatchResult, error) {
	result := models.ProductPublicationBatchResult{Items: make([]models.ProductPublicationResult, 0, len(requests))}
	if len(requests) < 1 || len(requests) > 1000 {
		return result, models.Error(400, "O lote deve conter de 1 a 1000 publicacoes")
	}
	for index := range requests {
		if err := normalizeProductPublication(&requests[index]); err != nil {
			return result, err
		}
	}
	err := s.Repo.Atomic(ctx, identity.CompanyID, func(r *repositories.TenantRepository) error {
		// A local-code remap can affect an already occupied destination mapping
		// and therefore keeps the conservative per-item path. A normal initial
		// load, however, has no remap request and can reconcile all products in
		// one transaction. This removes thousands of repeated identity queries.
		for _, request := range requests {
			if request.RequestNewLocalCode {
				for _, itemRequest := range requests {
					item, err := s.publishProductInTransaction(r, identity, itemRequest)
					if err != nil {
						return err
					}
					result.Items = append(result.Items, item)
				}
				result.ProcessedCount = len(result.Items)
				return nil
			}
		}

		if err := requireStoreIntegration(r, identity, "product"); err != nil {
			return err
		}
		needsPrices := false
		for _, request := range requests {
			if len(request.Prices) > 0 || request.PricesAuthoritative {
				needsPrices = true
				break
			}
		}
		if needsPrices {
			if err := requireStoreIntegration(r, identity, "prices"); err != nil {
				return err
			}
		}
		if err := lockProductSnapshots(r, identity, productSnapshots(requests)); err != nil {
			return err
		}

		canonicalItems, err := upsertProductSnapshotsBatch(r, identity, productSnapshots(requests))
		if err != nil {
			return err
		}
		if len(canonicalItems) != len(requests) {
			return models.Error(500, "Quantidade de produtos reconciliados diferente do lote recebido")
		}
		for index := range requests {
			// The backend is authoritative for the canonical code. Propagate the
			// chosen value to the source response and to every queued payload;
			// otherwise a remap would be persisted in the catalog but the next
			// store would still receive the colliding local code.
			canonicalCode := canonicalItems[index].LocalCode
			requests[index].Product.ProductID = canonicalItems[index].ID.String()
			requests[index].Product.LocalCode = canonicalCode
			requests[index].Product.SKU = canonicalItems[index].SKU
			for priceIndex := range requests[index].Prices {
				requests[index].Prices[priceIndex].LocalCode = canonicalCode
			}
		}

		group, err := groupForIdentity(r, identity)
		if err != nil {
			return err
		}
		for _, request := range requests[1:] {
			if !sameDestinationIDs(requests[0].DestinationStoreIDs, request.DestinationStoreIDs) {
				return models.Error(400, "Todos os itens do lote devem usar os mesmos destinos")
			}
		}
		destinations, err := publicationDestinations(r, identity, requests[0].DestinationStoreIDs)
		if err != nil {
			return err
		}
		// Validate the route once per destination instead of once per product.
		for _, destination := range destinations {
			if _, err := route(r, identity.StoreID, destination.ID); err != nil {
				return err
			}
		}

		priceRows := make([]models.ProductPrice, 0)
		authoritativeProductIDs := make([]uuid.UUID, 0, len(requests))
		seenAuthoritative := make(map[uuid.UUID]struct{}, len(requests))
		for index, request := range requests {
			productID := canonicalItems[index].ID
			if request.PricesAuthoritative {
				if _, seen := seenAuthoritative[productID]; !seen {
					authoritativeProductIDs = append(authoritativeProductIDs, productID)
					seenAuthoritative[productID] = struct{}{}
				}
			}
			for _, snapshot := range request.Prices {
				priceRows = append(priceRows, models.ProductPrice{
					Tenant: models.NewTenant(r.CompanyID()), StoreID: identity.StoreID, ProductID: productID,
					Kind: snapshot.Kind, Quantity: snapshot.Quantity, Amount: *snapshot.Amount,
					Active: *snapshot.Active, ValidFrom: snapshot.ValidFrom,
				})
			}
		}
		if len(authoritativeProductIDs) > 0 {
			if err := r.ReplaceProductPricesBatch(identity.StoreID, authoritativeProductIDs, priceRows); err != nil {
				return err
			}
		} else if len(priceRows) > 0 {
			if err := r.UpsertProductPrices(priceRows); err != nil {
				return err
			}
		}

		preparedResults := make([]models.ProductPublicationResult, len(requests))
		queuedByItem := make([]int, len(requests))
		messageRows := make([]models.Message, 0, len(requests)*len(destinations))
		messageIndexes := make([]int, 0, len(requests)*len(destinations))
		messageByID := make(map[uuid.UUID]int, len(requests)*len(destinations))
		for index, request := range requests {
			productID := canonicalItems[index].ID
			request.Product.ProductID = productID.String()
			publicationID := uuid.Nil
			if request.ID != nil && *request.ID != uuid.Nil {
				publicationID = *request.ID
			} else {
				identityPayload := publicationIdentity{
					Event: request.Event, Product: request.Product,
					Prices:              request.Prices,
					PricesAuthoritative: request.PricesAuthoritative,
					LocalCodeCandidate:  request.LocalCodeCandidate,
					RequestNewLocalCode: request.RequestNewLocalCode,
				}
				publicationID = uuid.NewSHA1(uuid.Nil, []byte(identity.StoreID.String()+"|"+productID.String()+"|"+hashJSON(identityPayload)))
			}
			payload := productPublicationPayload{ID: publicationID, Event: request.Event,
				Product: request.Product, Prices: request.Prices,
				PricesAuthoritative: request.PricesAuthoritative}
			payload.HashPayload = hashJSON(payload)
			payload.Version = publicationVersion(payload.HashPayload)
			payloadJSON := models.ToJSON(payload)
			operation := map[string]string{"CREATED": "product.created", "UPDATED": "product.updated", "DISABLED": "product.disabled"}[request.Event]
			if err := requireMessageIntegration(group, operation, payloadJSON); err != nil {
				return err
			}
			for _, destination := range destinations {
				message := models.Message{Tenant: models.NewTenant(r.CompanyID()), Type: "EVENT", Operation: operation,
					OriginStoreID: identity.StoreID, DestinationStoreID: destination.ID, Payload: payloadJSON, Status: "PENDING"}
				message.ID = uuid.NewSHA1(uuid.Nil, []byte(publicationID.String()+"|"+destination.ID.String()))
				if priorIndex, exists := messageByID[message.ID]; exists {
					prior := messageRows[priorIndex]
					if prior.Operation != message.Operation || prior.OriginStoreID != message.OriginStoreID || prior.DestinationStoreID != message.DestinationStoreID || !sameJSON(prior.Payload, message.Payload) {
						return models.Error(409, "ID de publicacao ja utilizado com outro conteudo")
					}
					continue
				}
				messageByID[message.ID] = len(messageRows)
				messageRows = append(messageRows, message)
				messageIndexes = append(messageIndexes, index)
			}
			preparedResults[index] = models.ProductPublicationResult{ID: publicationID, ProductID: productID,
				LocalCode: request.Product.LocalCode, Event: request.Event, Version: payload.Version,
				HashPayload: payload.HashPayload}
		}

		// Validate existing deterministic IDs in one read. Only messages not
		// already present reach the bulk insert below; retries therefore remain
		// idempotent without 2N round trips.
		messageIDs := make([]uuid.UUID, 0, len(messageRows))
		for _, message := range messageRows {
			messageIDs = append(messageIDs, message.ID)
		}
		existingMessages, err := r.MessagesByIDs(messageIDs)
		if err != nil {
			return err
		}
		existingByID := make(map[uuid.UUID]models.Message, len(existingMessages))
		for _, existing := range existingMessages {
			existingByID[existing.ID] = existing
		}
		newMessages := make([]models.Message, 0, len(messageRows))
		for index, message := range messageRows {
			if existing, exists := existingByID[message.ID]; exists {
				if existing.Operation != message.Operation || existing.OriginStoreID != message.OriginStoreID || existing.DestinationStoreID != message.DestinationStoreID || !sameJSON(existing.Payload, message.Payload) {
					return models.Error(409, "ID de publicacao ja utilizado com outro conteudo")
				}
				continue
			}
			newMessages = append(newMessages, message)
			queuedByItem[messageIndexes[index]]++
		}
		if _, err := r.InsertMessagesUnique(newMessages); err != nil {
			return err
		}
		for index := range preparedResults {
			preparedResults[index].DestinationsQueued = queuedByItem[index]
			result.Items = append(result.Items, preparedResults[index])
		}
		result.ProcessedCount = len(result.Items)
		return nil
	})
	return result, err
}

func productSnapshots(requests []models.ProductPublicationDTO) []models.ProductSnapshot {
	out := make([]models.ProductSnapshot, len(requests))
	for index := range requests {
		out[index] = requests[index].Product
	}
	return out
}

func sameDestinationIDs(a, b []uuid.UUID) bool {
	if len(a) != len(b) {
		return false
	}
	for index := range a {
		if a[index] != b[index] {
			return false
		}
	}
	return true
}

func normalizeProductPublication(request *models.ProductPublicationDTO) error {
	request.Event = strings.ToUpper(strings.TrimSpace(request.Event))
	request.LocalCodeCandidate = strings.TrimSpace(request.LocalCodeCandidate)
	if request.LocalCodeCandidate != "" && len(request.LocalCodeCandidate) > 100 {
		return models.Error(400, "localCodeCandidate excede 100 caracteres")
	}
	if request.RequestNewLocalCode && request.LocalCodeCandidate == "" {
		return models.Error(400, "localCodeCandidate e obrigatorio quando requestNewLocalCode=true")
	}
	if request.Event != "CREATED" && request.Event != "UPDATED" && request.Event != "DISABLED" {
		return models.Error(400, "event deve ser CREATED, UPDATED ou DISABLED")
	}
	if err := normalizeProductBatchSnapshot(&request.Product); err != nil {
		return err
	}
	if request.Event == "DISABLED" {
		active := false
		request.Product.Active = &active
	}
	if len(request.Prices) > 100 {
		return models.Error(400, "Uma publicacao pode conter no maximo 100 precos")
	}
	for index := range request.Prices {
		if strings.TrimSpace(request.Prices[index].LocalCode) == "" {
			request.Prices[index].LocalCode = request.Product.LocalCode
		}
		if !strings.EqualFold(strings.TrimSpace(request.Prices[index].LocalCode), request.Product.LocalCode) {
			return models.Error(400, "Todos os precos devem pertencer ao localCode do produto publicado")
		}
		if err := normalizeAndValidateProductPrice(&request.Prices[index]); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) publishProductInTransaction(r *repositories.TenantRepository, identity models.Identity, request models.ProductPublicationDTO) (models.ProductPublicationResult, error) {
	if err := requireStoreIntegration(r, identity, "product"); err != nil {
		return models.ProductPublicationResult{}, err
	}
	if len(request.Prices) > 0 || request.PricesAuthoritative {
		if err := requireStoreIntegration(r, identity, "prices"); err != nil {
			return models.ProductPublicationResult{}, err
		}
	}
	if err := lockProductSnapshots(r, identity, []models.ProductSnapshot{request.Product}); err != nil {
		return models.ProductPublicationResult{}, err
	}
	if request.RequestNewLocalCode {
		// During a remap the local code identifies the product that is being
		// moved.  The FNTS payload intentionally removes barcodes that belong
		// to the incoming canonical product, and its SKU is the old local code
		// for compatibility.  Resolving that payload by SKU first can therefore
		// select the incoming product instead of the occupied product.  EAN and
		// an explicit global id remain stronger identities; when neither exists,
		// prefer the current store mapping and do not fall through to SKU.
		productID, err := resolvePublicationProductID(r, identity, request.Product, true)
		if err != nil {
			return models.ProductPublicationResult{}, err
		}
		newCanonicalProduct := productID == uuid.Nil
		if productID == uuid.Nil {
			// A base local pode ser anterior ao vínculo global.  If the
			// occupied product has no EAN/global id and StoreLink has no
			// mapping for it yet, it is still a real local product and must
			// be published as a new canonical identity, never merged by its
			// legacy SKU (which is only the old local code in this contract).
			productID = uuid.New()
		}
		candidate := request.LocalCodeCandidate
		for attempt := 0; ; attempt++ {
			// The local code is also the legacy SKU in the FNTS contract.
			// A candidate can therefore be free in the destination mapping while
			// still belonging to another canonical product globally.  Treat both
			// namespaces as one reservation here; otherwise the subsequent batch
			// upsert sees ProductID + SKU for different products and returns the
			// misleading PRODUCT_IDENTITY_CONFLICT that used to stop the load.
			if newCanonicalProduct {
				_, occupied, lookupErr := r.ProductMapping(identity.StoreID, candidate)
				err = lookupErr
				if err == nil && occupied {
					err = models.Error(409, "PRODUCT_LOCAL_CODE_OCCUPIED: o codigo local candidato ja pertence a outro produto")
				}
			} else {
				err = r.MoveProductMapping(identity.StoreID, productID, candidate)
			}
			if err == nil {
				break
			}
			app, isAppError := err.(*models.AppError)
			if !isAppError || app.Codigo != 409 || !strings.HasPrefix(err.Error(), "PRODUCT_LOCAL_CODE_OCCUPIED:") {
				return models.ProductPublicationResult{}, err
			}
			if attempt >= 1000 {
				return models.ProductPublicationResult{}, models.Error(409, "PRODUCT_LOCAL_CODE_EXHAUSTED: nao foi possivel reservar um codigo local livre")
			}
			candidate = nextLocalCodeCandidate(candidate)
			if candidate == "" {
				return models.ProductPublicationResult{}, models.Error(409, "PRODUCT_LOCAL_CODE_EXHAUSTED: nao foi possivel reservar um codigo local livre")
			}
		}
		request.LocalCodeCandidate = candidate
		request.Product.ProductID = productID.String()
		request.Product.LocalCode = candidate
	}
	items, err := upsertProductSnapshotsBatch(r, identity, []models.ProductSnapshot{request.Product})
	if err != nil {
		return models.ProductPublicationResult{}, err
	}
	if len(items) != 1 {
		return models.ProductPublicationResult{}, models.Error(500, "Produto publicado sem identidade canonica")
	}
	productID := items[0].ID
	request.Product.ProductID = productID.String()
	request.Product.LocalCode = items[0].LocalCode
	request.Product.SKU = items[0].SKU
	for index := range request.Prices {
		request.Prices[index].LocalCode = items[0].LocalCode
	}

	if len(request.Prices) > 0 || request.PricesAuthoritative {
		prices := make([]models.ProductPrice, 0, len(request.Prices))
		for _, snapshot := range request.Prices {
			prices = append(prices, models.ProductPrice{
				Tenant: models.NewTenant(r.CompanyID()), StoreID: identity.StoreID, ProductID: productID,
				Kind: snapshot.Kind, Quantity: snapshot.Quantity, Amount: *snapshot.Amount,
				Active: *snapshot.Active, ValidFrom: snapshot.ValidFrom,
			})
		}
		var priceErr error
		if request.PricesAuthoritative {
			priceErr = r.ReplaceProductPrices(identity.StoreID, productID, prices)
		} else {
			priceErr = r.UpsertProductPrices(prices)
		}
		if priceErr != nil {
			return models.ProductPublicationResult{}, priceErr
		}
	}

	publicationID := uuid.Nil
	if request.ID != nil && *request.ID != uuid.Nil {
		publicationID = *request.ID
	} else {
		publicationID = uuid.NewSHA1(uuid.Nil, []byte(identity.StoreID.String()+"|"+productID.String()+"|"+request.Event+"|"+hashJSON(request.Product)))
	}
	payload := productPublicationPayload{ID: publicationID, Event: request.Event,
		Product: request.Product, Prices: request.Prices,
		PricesAuthoritative: request.PricesAuthoritative}
	payload.HashPayload = hashJSON(payload)
	payload.Version = publicationVersion(payload.HashPayload)
	payloadJSON := models.ToJSON(payload)
	operation := map[string]string{"CREATED": "product.created", "UPDATED": "product.updated", "DISABLED": "product.disabled"}[request.Event]
	group, err := groupForIdentity(r, identity)
	if err != nil {
		return models.ProductPublicationResult{}, err
	}
	destinations, err := publicationDestinations(r, identity, request.DestinationStoreIDs)
	if err != nil {
		return models.ProductPublicationResult{}, err
	}
	queued := 0
	for _, destination := range destinations {
		if _, err := route(r, identity.StoreID, destination.ID); err != nil {
			return models.ProductPublicationResult{}, err
		}
		if err := requireMessageIntegration(group, operation, payloadJSON); err != nil {
			return models.ProductPublicationResult{}, err
		}
		messageID := uuid.NewSHA1(uuid.Nil, []byte(publicationID.String()+"|"+destination.ID.String()))
		message := models.Message{Tenant: models.NewTenant(r.CompanyID()), Type: "EVENT", Operation: operation,
			OriginStoreID: identity.StoreID, DestinationStoreID: destination.ID, Payload: payloadJSON, Status: "PENDING"}
		message.ID = messageID
		var existing models.Message
		findErr := r.Find(&existing, messageID, false)
		if findErr == nil {
			if existing.Operation != message.Operation || existing.OriginStoreID != message.OriginStoreID || existing.DestinationStoreID != message.DestinationStoreID || !sameJSON(existing.Payload, payloadJSON) {
				return models.ProductPublicationResult{}, models.Error(409, "ID de publicacao ja utilizado com outro conteudo")
			}
			continue
		}
		if !repositories.IsNotFound(findErr) {
			return models.ProductPublicationResult{}, findErr
		}
		created, err := r.InsertUnique(&message)
		if err != nil {
			return models.ProductPublicationResult{}, err
		}
		if created {
			queued++
		}
	}
	return models.ProductPublicationResult{ID: publicationID, ProductID: productID, LocalCode: request.Product.LocalCode,
		Event: request.Event, Version: payload.Version, HashPayload: payload.HashPayload,
		DestinationsQueued: queued}, nil
}

// nextLocalCodeCandidate advances the six-digit numeric code used by the
// FNTS product tables.  A remap request may arrive with a candidate already
// occupied by another agent, so the backend must be able to choose the next
// free code without making the ERP retry a request with stale local state.
func nextLocalCodeCandidate(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	number, err := strconv.ParseInt(value, 10, 64)
	if err != nil || number >= 999999 {
		return ""
	}
	width := len(value)
	if width < 6 {
		width = 6
	}
	return fmt.Sprintf("%0*d", width, number+1)
}

// resolvePublicationProductID deliberately gives active EANs precedence over
// the local code. A stale local code is exactly the conflict that the remap
// request is meant to repair; accepting it first would move the wrong product.
func resolvePublicationProductID(r *repositories.TenantRepository, identity models.Identity, snapshot models.ProductSnapshot, preferLocalMapping bool) (uuid.UUID, error) {
	// An active EAN is the strongest identity available from FNTS. A stale
	// productId/SKU must not make a valid EAN fail the remap: this is exactly
	// the situation produced when a local occupied code is being liberated.
	var barcodeOwner uuid.UUID
	barcodeFound := false
	for _, barcode := range snapshot.Barcodes {
		if barcode.Active == nil || !*barcode.Active {
			continue
		}
		value, found, err := r.ProductBarcode(normalizeCatalogKey(barcode.Value))
		if err != nil {
			return uuid.Nil, err
		}
		if !found {
			continue
		}
		if barcodeFound && barcodeOwner != value.ProductID {
			return uuid.Nil, models.Error(409, "PRODUCT_IDENTITY_CONFLICT: codigos de barras ativos apontam para produtos diferentes")
		}
		barcodeOwner = value.ProductID
		barcodeFound = true
	}
	if barcodeFound {
		return barcodeOwner, nil
	}
	if preferLocalMapping {
		// A remap request describes the product that currently occupies the
		// local code.  In that payload the legacy SKU is deliberately the old
		// code and may identify the product that is arriving in the same load.
		// Comparing that SKU with productId would therefore manufacture a
		// false PRODUCT_IDENTITY_CONFLICT.  After EAN, the destination mapping
		// is the strongest local identity; an explicit global id is the next
		// fallback.  SKU must not participate in remap identity resolution.
		mapping, found, err := r.ProductMapping(identity.StoreID, snapshot.LocalCode)
		if err != nil {
			return uuid.Nil, err
		}
		if found {
			return mapping.ProductID, nil
		}
		if strings.TrimSpace(snapshot.ProductID) != "" {
			id, err := uuid.Parse(strings.TrimSpace(snapshot.ProductID))
			if err != nil || id == uuid.Nil {
				return uuid.Nil, models.Error(400, "productId deve ser UUID valido")
			}
			var product models.Product
			if err := r.Find(&product, id, false); err != nil {
				return uuid.Nil, err
			}
			return product.ID, nil
		}
		return uuid.Nil, nil
	}

	selected := uuid.Nil
	selectID := func(candidate uuid.UUID) error {
		if candidate == uuid.Nil {
			return nil
		}
		if selected != uuid.Nil && selected != candidate {
			return models.Error(409, "PRODUCT_IDENTITY_CONFLICT: productId, SKU e EAN apontam para produtos diferentes")
		}
		selected = candidate
		return nil
	}
	if strings.TrimSpace(snapshot.ProductID) != "" {
		id, err := uuid.Parse(strings.TrimSpace(snapshot.ProductID))
		if err != nil || id == uuid.Nil {
			return uuid.Nil, models.Error(400, "productId deve ser UUID valido")
		}
		var product models.Product
		if err := r.Find(&product, id, false); err != nil {
			return uuid.Nil, err
		}
		if err := selectID(product.ID); err != nil {
			return uuid.Nil, err
		}
	}
	if preferLocalMapping && selected == uuid.Nil {
		mapping, found, err := r.ProductMapping(identity.StoreID, snapshot.LocalCode)
		if err != nil {
			return uuid.Nil, err
		}
		if found {
			selected = mapping.ProductID
		}
	}
	if sku := normalizeCatalogKey(snapshot.SKU); sku != "" && (!preferLocalMapping || selected != uuid.Nil) {
		product, found, err := r.ProductBySKU(sku)
		if err != nil {
			return uuid.Nil, err
		}
		if found {
			if err := selectID(product.ID); err != nil {
				return uuid.Nil, err
			}
		}
	}
	if selected == uuid.Nil && !preferLocalMapping {
		mapping, found, err := r.ProductMapping(identity.StoreID, snapshot.LocalCode)
		if err != nil {
			return uuid.Nil, err
		}
		if found {
			selected = mapping.ProductID
		}
	}
	return selected, nil
}

// publicationVersion is deterministic for an identical publication and
// changes with its payload. The local FNTS receiver stores it as an integer
// version, while the backend keeps the full SHA-256 hash for idempotency.
// Keep the value below 2^53: older Delphi JSON parsers materialize numeric
// tokens through IEEE-754 doubles before converting them to Int64.
func publicationVersion(hash string) int64 {
	if len(hash) < 13 {
		return 1
	}
	var value int64
	for _, char := range hash[:13] {
		value <<= 4
		switch {
		case char >= '0' && char <= '9':
			value += int64(char - '0')
		case char >= 'a' && char <= 'f':
			value += int64(char-'a') + 10
		case char >= 'A' && char <= 'F':
			value += int64(char-'A') + 10
		}
	}
	if value <= 0 {
		return 1
	}
	return value
}

func groupForIdentity(r *repositories.TenantRepository, identity models.Identity) (models.Group, error) {
	var group models.Group
	if err := r.Find(&group, identity.GroupID, false); err != nil {
		return group, err
	}
	if !group.Ativo {
		return group, models.Error(409, "Grupo inativo")
	}
	return group, nil
}

func publicationDestinations(r *repositories.TenantRepository, identity models.Identity, requested []uuid.UUID) ([]models.Store, error) {
	if len(requested) == 0 {
		stores, err := r.ActiveStoresInGroup(identity.GroupID)
		if err != nil {
			return nil, err
		}
		out := stores[:0]
		for _, store := range stores {
			if store.ID != identity.StoreID {
				out = append(out, store)
			}
		}
		return out, nil
	}
	seen := make(map[uuid.UUID]struct{}, len(requested))
	result := make([]models.Store, 0, len(requested))
	for _, id := range requested {
		if id == uuid.Nil || id == identity.StoreID {
			return nil, models.Error(400, "Destino de publicacao invalido")
		}
		if _, exists := seen[id]; exists {
			return nil, models.Error(400, "Destino de publicacao duplicado")
		}
		seen[id] = struct{}{}
		var store models.Store
		if err := r.Find(&store, id, false); err != nil {
			return nil, err
		}
		if !store.Ativo {
			return nil, models.Error(409, fmt.Sprintf("Loja destino %s inativa", id))
		}
		result = append(result, store)
	}
	return result, nil
}

func (s *Service) ProductCatalog(ctx context.Context, identity models.Identity, page, pageSize int) (models.ProductCatalogPage, error) {
	const maxProductCatalogPageSize = 5000
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 100
	}
	if pageSize > maxProductCatalogPageSize {
		pageSize = maxProductCatalogPageSize
	}
	var result models.ProductCatalogPage
	err := s.Repo.Atomic(ctx, identity.CompanyID, func(r *repositories.TenantRepository) error {
		if err := requireStoreIntegration(r, identity, "product"); err != nil {
			return err
		}
		// A complete load must remain sourced from the group's canonical store
		// even after a previous attempt populated part of the destination. If
		// we switched to the destination as soon as it had one mapping, a
		// crash after page N would make every retry believe that the partial
		// destination was the complete catalog and permanently skip the rest.
		// The source selection is deterministic and the publication contract is
		// idempotent, so this is safe for retries and concurrent destinations.
		catalogStoreID, err := r.ProductCatalogSourceStoreID(identity.GroupID)
		if err != nil {
			return err
		}
		mappings, total, err := r.ProductMappingsForCatalog(catalogStoreID, (page-1)*pageSize, pageSize)
		if err != nil {
			return err
		}
		ids := make([]uuid.UUID, 0, len(mappings))
		for _, mapping := range mappings {
			ids = append(ids, mapping.ProductID)
		}
		products, err := r.ProductsByIDs(ids)
		if err != nil {
			return err
		}
		productByID := make(map[uuid.UUID]models.Product, len(products))
		for _, product := range products {
			productByID[product.ID] = product
		}
		barcodes, err := r.ProductBarcodesForProducts(ids)
		if err != nil {
			return err
		}
		barcodesByID := make(map[uuid.UUID][]models.ProductBarcode)
		for _, barcode := range barcodes {
			barcodesByID[barcode.ProductID] = append(barcodesByID[barcode.ProductID], barcode)
		}
		pricesByID := make(map[uuid.UUID][]models.ProductPrice)
		group, groupErr := groupForIdentity(r, identity)
		if groupErr != nil {
			return groupErr
		} else if group.IntegrarPrecos {
			prices, priceErr := r.ProductPricesForStoreProducts(catalogStoreID, ids)
			if priceErr != nil {
				return priceErr
			}
			for _, price := range prices {
				pricesByID[price.ProductID] = append(pricesByID[price.ProductID], price)
			}
		}
		result.Items = make([]models.ProductCatalogItem, 0, len(mappings))
		for _, mapping := range mappings {
			product, found := productByID[mapping.ProductID]
			if !found {
				continue
			}
			active, tracksSerial, usaGrade := product.Active, product.TracksSerial, product.UsaGrade
			snapshot := models.ProductSnapshot{
				ProductID: product.ID.String(), LocalCode: mapping.LocalCode, SKU: product.SKU,
				EstoqueMinimo: product.EstoqueMinimo, MargemMinima: product.MargemMinima,
				FlagSis: product.FlagSis,
				Name:    product.Name, Description: product.Description, Unit: product.Unit,
				Brand: product.Brand, CodigoMarca: product.CodigoMarca,
				DataCadastro: product.DataCadastro,
				Aplicacao:    product.Aplicacao, Origem: product.Origem,
				NCM: product.NCM, CEST: product.CEST,
				ClassificacaoFiscal: product.ClassificacaoFiscal, ClasseTP: product.ClasseTP,
				CST: product.CST, CodigoMS: product.CodigoMS, PrincipioAtivo: product.PrincipioAtivo,
				NomeLaboratorio: product.NomeLaboratorio, CodigoFornecedor: product.CodigoFornecedor,
				NomeFornecedor: product.NomeFornecedor, CNPJFornecedor: product.CNPJFornecedor,
				NomeFantasiaFornecedor: product.NomeFantasiaFornecedor, Tipo: product.Tipo,
				FarmaciaControlado:          product.FarmaciaControlado,
				FarmaciaApresentacao:        product.FarmaciaApresentacao,
				FarmaciaRegistroMedicamento: product.FarmaciaRegistroMedicamento,
				Apresentacao:                product.Apresentacao, CodigoOriginal: product.CodigoOriginal,
				CSOSN: product.CSOSN, UsaLote: product.UsaLote, UsaBalanca: product.UsaBalanca,
				CodigoReceita: product.CodigoReceita, CodigoBarraNovartis: product.CodigoBarraNovartis,
				SituacaoTributaria: product.SituacaoTributaria, PISCOFINS: product.PISCOFINS,
				IncidenciaPISCOFINS: product.IncidenciaPISCOFINS, IAT: product.IAT, IPPT: product.IPPT,
				IndCFOPVendaDentro:            product.IndCFOPVendaDentro,
				CodigoClassificacaoTributaria: product.CodigoClassificacaoTributaria,
				CSTIBS:                        product.CSTIBS, CSTCBS: product.CSTCBS,
				AliquotaIBS: product.AliquotaIBS, AliquotaCBS: product.AliquotaCBS,
				AliquotaEfetivaIBS:   product.AliquotaEfetivaIBS,
				AliquotaEfetivaCBS:   product.AliquotaEfetivaCBS,
				ObsReformaTributaria: product.ObsReformaTributaria,
				PrecoVenda:           product.PrecoVenda,
				PrecoVenda1:          product.PrecoVenda1, PrecoVenda2: product.PrecoVenda2,
				PrecoVenda3: product.PrecoVenda3, PrecoVenda4: product.PrecoVenda4,
				PrecoVenda5: product.PrecoVenda5, PrecoPromocao: product.PrecoPromocao,
				MargemDesconto: product.MargemDesconto, Aliquota: product.Aliquota,
				QtdeEmbalagem:      product.QtdeEmbalagem,
				DescontoPrecoVenda: product.DescontoPrecoVenda,
				FarmaciaPMC:        product.FarmaciaPMC, DataPromocao: product.DataPromocao,
				FimPromocao: product.FimPromocao, UsaTabelaPreco: product.UsaTabelaPreco,
				PMargem1: product.PMargem1,
				PMargem2: product.PMargem2, PMargem3: product.PMargem3,
				PMargem4: product.PMargem4, PMargem5: product.PMargem5,
				Active: &active, TracksSerial: &tracksSerial, UsaGrade: &usaGrade, Barcodes: []models.ProductBarcodeValue{},
			}
			if mapping.GroupName != "" || mapping.GroupLocalCode != "" {
				snapshot.Group = &models.ProductCategoryValue{LocalCode: mapping.GroupLocalCode, Name: mapping.GroupName}
			}
			if mapping.SubgroupName != "" || mapping.SubgroupLocalCode != "" {
				snapshot.Subgroup = &models.ProductCategoryValue{LocalCode: mapping.SubgroupLocalCode, Name: mapping.SubgroupName}
			}
			for _, barcode := range barcodesByID[product.ID] {
				barcodeActive, barcodePrimary := barcode.Active, barcode.Primary
				barcodeC000055 := barcode.C000055
				snapshot.Barcodes = append(snapshot.Barcodes, models.ProductBarcodeValue{Value: barcode.Value, Primary: &barcodePrimary, Active: &barcodeActive, Fraction: barcode.Fraction, C000055: &barcodeC000055})
			}
			item := models.ProductCatalogItem{Product: snapshot, Prices: []models.ProductPriceSnapshot{}, PricesAuthoritative: group.IntegrarPrecos}
			for _, price := range pricesByID[product.ID] {
				priceActive, amount := price.Active, price.Amount
				item.Prices = append(item.Prices, models.ProductPriceSnapshot{LocalCode: mapping.LocalCode, Kind: price.Kind, Quantity: price.Quantity, Amount: &amount, Active: &priceActive, ValidFrom: price.ValidFrom})
			}
			result.Items = append(result.Items, item)
		}
		result.Total, result.Page, result.PageSize = total, page, pageSize
		result.HasMore = int64(page*pageSize) < total
		return nil
	})
	return result, err
}
