package services

import (
	"strings"

	"github.com/google/uuid"
	"github.com/phsoftwares/storelink-backend/internal/models"
	"github.com/phsoftwares/storelink-backend/internal/repositories"
)

func normalizeProductBatchSnapshot(snapshot *models.ProductSnapshot) error {
	if snapshot.Active == nil || snapshot.TracksSerial == nil {
		return models.Error(400, "Produto requer active e tracksSerial como valores booleanos")
	}
	snapshot.LocalCode = strings.TrimSpace(snapshot.LocalCode)
	snapshot.Name = strings.TrimSpace(snapshot.Name)
	snapshot.SKU = strings.TrimSpace(snapshot.SKU)
	snapshot.Unit = strings.TrimSpace(snapshot.Unit)
	if snapshot.LocalCode == "" || len(snapshot.LocalCode) > 100 || snapshot.Name == "" || len(snapshot.Name) > 255 || len(snapshot.Description) > 1000 || len(snapshot.Unit) > 20 || len(snapshot.SKU) > 100 || len(snapshot.Brand) > 100 || len(snapshot.NCM) > 20 || len(snapshot.CEST) > 20 {
		return models.Error(400, "Produto contem campos obrigatorios vazios ou fora do limite")
	}
	if len(snapshot.Barcodes) == 0 {
		snapshot.Barcodes = []models.ProductBarcodeValue{}
	}
	seenBarcodes := make(map[string]struct{}, len(snapshot.Barcodes))
	primaryCount := 0
	for index := range snapshot.Barcodes {
		barcode := &snapshot.Barcodes[index]
		if barcode.Primary == nil || barcode.Active == nil {
			return models.Error(400, "Cada codigo de barras requer isPrimary e active como valores booleanos")
		}
		barcode.Value = strings.TrimSpace(barcode.Value)
		if barcode.Value == "" || len(barcode.Value) > 32 {
			return models.Error(400, "Codigo de barras deve conter de 1 a 32 caracteres")
		}
		key := normalizeCatalogKey(barcode.Value)
		if _, exists := seenBarcodes[key]; exists {
			return models.Error(400, "Produto contem codigos de barras duplicados")
		}
		seenBarcodes[key] = struct{}{}
		if *barcode.Primary {
			primaryCount++
		}
		if barcode.Fraction != nil && (*barcode.Fraction < 0 || *barcode.Fraction > 1000000) {
			return models.Error(400, "Fracao de codigo de barras fora do intervalo permitido")
		}
	}
	if primaryCount > 1 {
		return models.Error(400, "Produto pode ter no maximo um codigo de barras principal")
	}
	if snapshot.ProductID != "" {
		id, err := uuid.Parse(snapshot.ProductID)
		if err != nil || id == uuid.Nil {
			return models.Error(400, "productId deve ser UUID valido")
		}
		snapshot.ProductID = id.String()
	}
	if snapshot.Group != nil {
		snapshot.Group.LocalCode = strings.TrimSpace(snapshot.Group.LocalCode)
		snapshot.Group.Name = strings.TrimSpace(snapshot.Group.Name)
		if len(snapshot.Group.LocalCode) > 100 || len(snapshot.Group.Name) > 100 {
			return models.Error(400, "Grupo de produto excede os limites suportados")
		}
	}
	if snapshot.Subgroup != nil {
		snapshot.Subgroup.LocalCode = strings.TrimSpace(snapshot.Subgroup.LocalCode)
		snapshot.Subgroup.Name = strings.TrimSpace(snapshot.Subgroup.Name)
		if len(snapshot.Subgroup.LocalCode) > 100 || len(snapshot.Subgroup.Name) > 100 {
			return models.Error(400, "Subgrupo de produto excede os limites suportados")
		}
	}
	return nil
}

func upsertProductSnapshotsBatch(r *repositories.TenantRepository, identity models.Identity, snapshots []models.ProductSnapshot) ([]models.CatalogWriteResult, error) {
	return upsertProductSnapshotsBatchWithOptions(r, identity, snapshots, true)
}

func upsertProductSnapshotsBatchDirect(r *repositories.TenantRepository, identity models.Identity, snapshots []models.ProductSnapshot) ([]models.CatalogWriteResult, error) {
	return upsertProductSnapshotsBatchWithOptions(r, identity, snapshots, false)
}

func upsertProductSnapshotsBatchWithOptions(r *repositories.TenantRepository, identity models.Identity, snapshots []models.ProductSnapshot, reconcileCanonicalCodes bool) ([]models.CatalogWriteResult, error) {
	localCodes := make([]string, 0, len(snapshots))
	skus := make([]string, 0, len(snapshots))
	barcodeValues := make([]string, 0)
	productIDs := make([]uuid.UUID, 0, len(snapshots))
	seenSKUs := make(map[string]struct{}, len(snapshots))
	seenBarcodes := make(map[string]struct{}, len(snapshots))
	seenIDs := make(map[uuid.UUID]struct{}, len(snapshots))
	for _, snapshot := range snapshots {
		localCode := normalizeCatalogKey(snapshot.LocalCode)
		localCodes = append(localCodes, localCode)
		// SKU is a tenant-wide identity and must never be synthesized from the
		// local C000025.CODIGO. The local code is resolved through the store
		// mapping below.
		for _, candidate := range []string{normalizeCatalogKey(snapshot.SKU)} {
			if candidate == "" {
				continue
			}
			if _, exists := seenSKUs[candidate]; !exists {
				skus = append(skus, candidate)
				seenSKUs[candidate] = struct{}{}
			}
		}
		if snapshot.ProductID != "" {
			id, _ := uuid.Parse(snapshot.ProductID)
			if _, exists := seenIDs[id]; !exists {
				productIDs = append(productIDs, id)
				seenIDs[id] = struct{}{}
			}
		}
		for _, barcode := range snapshot.Barcodes {
			value := normalizeCatalogKey(barcode.Value)
			if _, exists := seenBarcodes[value]; !exists {
				barcodeValues = append(barcodeValues, value)
				seenBarcodes[value] = struct{}{}
			}
		}
	}

	storeMappings, err := r.ProductMappingsByLocalCodes(identity.StoreID, localCodes)
	if err != nil {
		return nil, err
	}
	mappingByLocalCode := make(map[string]models.ProductStoreMapping, len(storeMappings))
	for _, mapping := range storeMappings {
		mappingByLocalCode[normalizeCatalogKey(mapping.LocalCode)] = mapping
		if _, exists := seenIDs[mapping.ProductID]; !exists {
			productIDs = append(productIDs, mapping.ProductID)
			seenIDs[mapping.ProductID] = struct{}{}
		}
	}

	productsBySKU, err := r.ProductsBySKUs(skus)
	if err != nil {
		return nil, err
	}
	productBySKU := make(map[string]uuid.UUID, len(productsBySKU))
	for _, product := range productsBySKU {
		productBySKU[normalizeCatalogKey(product.SKU)] = product.ID
		if _, exists := seenIDs[product.ID]; !exists {
			productIDs = append(productIDs, product.ID)
			seenIDs[product.ID] = struct{}{}
		}
	}

	barcodes, err := r.ProductBarcodesByNormalizedValues(barcodeValues)
	if err != nil {
		return nil, err
	}
	barcodesByValue := make(map[string][]models.ProductBarcode, len(barcodes))
	for _, barcode := range barcodes {
		key := normalizeCatalogKey(barcode.NormalizedValue)
		barcodesByValue[key] = append(barcodesByValue[key], barcode)
		if _, exists := seenIDs[barcode.ProductID]; !exists {
			productIDs = append(productIDs, barcode.ProductID)
			seenIDs[barcode.ProductID] = struct{}{}
		}
	}
	sourceIsMatrix, err := storeIsMatrix(r, identity.StoreID)
	if err != nil {
		return nil, err
	}

	selectCandidate := func(selected *uuid.UUID, candidate uuid.UUID) error {
		if *selected != uuid.Nil && *selected != candidate {
			return models.Error(409, "PRODUCT_IDENTITY_CONFLICT: SKU e codigos de barras apontam para produtos diferentes")
		}
		*selected = candidate
		return nil
	}
	resolvedIDs := make([]uuid.UUID, len(snapshots))
	batchSKU := make(map[string]uuid.UUID, len(skus))
	batchBarcode := make(map[string]uuid.UUID, len(barcodeValues))
	matrixReassignments := make(map[uuid.UUID][]string)
	matrixReassignmentSeen := make(map[uuid.UUID]map[string]struct{})
	matrixLosers := make(map[uuid.UUID]struct{})
	addMatrixReassignment := func(winner uuid.UUID, value string) {
		value = normalizeCatalogKey(value)
		if winner == uuid.Nil || value == "" {
			return
		}
		seen := matrixReassignmentSeen[winner]
		if seen == nil {
			seen = make(map[string]struct{})
			matrixReassignmentSeen[winner] = seen
		}
		if _, exists := seen[value]; exists {
			return
		}
		seen[value] = struct{}{}
		matrixReassignments[winner] = append(matrixReassignments[winner], value)
	}
	for index, snapshot := range snapshots {
		localCode := normalizeCatalogKey(snapshot.LocalCode)
		incomingPrimaryValue := ""
		for _, barcode := range snapshot.Barcodes {
			if barcode.Active != nil && *barcode.Active && barcode.Primary != nil && *barcode.Primary {
				incomingPrimaryValue = normalizeCatalogKey(barcode.Value)
				break
			}
		}
		owners := make(map[uuid.UUID]struct{})
		primaryOwner := uuid.Nil
		for _, barcode := range snapshot.Barcodes {
			if barcode.Active == nil || !*barcode.Active {
				continue
			}
			key := normalizeCatalogKey(barcode.Value)
			for _, existing := range barcodesByValue[key] {
				if !existing.Active {
					continue
				}
				owners[existing.ProductID] = struct{}{}
				if key == incomingPrimaryValue {
					primaryOwner = existing.ProductID
				}
			}
			if id, found := batchBarcode[key]; found {
				owners[id] = struct{}{}
				if key == incomingPrimaryValue {
					primaryOwner = id
				}
			}
		}
		if len(owners) > 1 && (!sourceIsMatrix || primaryOwner == uuid.Nil) {
			return nil, models.Error(409, "PRODUCT_IDENTITY_CONFLICT: codigos de barras ativos apontam para produtos diferentes")
		}
		barcodeOwner := primaryOwner
		if barcodeOwner == uuid.Nil && len(owners) == 1 {
			for owner := range owners {
				barcodeOwner = owner
			}
		}
		if barcodeOwner != uuid.Nil {
			if sourceIsMatrix {
				for owner := range owners {
					if owner != barcodeOwner {
						matrixLosers[owner] = struct{}{}
					}
				}
				for _, barcode := range snapshot.Barcodes {
					if barcode.Active == nil || !*barcode.Active {
						continue
					}
					key := normalizeCatalogKey(barcode.Value)
					for _, existing := range barcodesByValue[key] {
						if existing.Active && existing.ProductID != barcodeOwner {
							addMatrixReassignment(barcodeOwner, key)
						}
					}
				}
			}
			// Do not combine a stale local code/SKU/ProductID with the EAN
			// owner. The destination reconciliation will choose a free local
			// code when its current code is occupied.
			selected := barcodeOwner
			resolvedIDs[index] = selected
			if sku := normalizeCatalogKey(snapshot.SKU); sku != "" {
				batchSKU[sku] = selected
			}
			for _, barcode := range snapshot.Barcodes {
				if barcode.Active != nil && *barcode.Active {
					batchBarcode[normalizeCatalogKey(barcode.Value)] = selected
				}
			}
			continue
		}
		if hasActiveProductBarcode(snapshot) {
			// The EAN is present but is not known in this tenant yet. It must
			// not fall through to a reused local code or SKU: that is exactly
			// how two different products from two stores used to be merged.
			selected := uuid.New()
			if snapshot.ProductID != "" {
				selectedID, parseErr := uuid.Parse(snapshot.ProductID)
				if parseErr != nil || selectedID == uuid.Nil {
					return nil, models.Error(400, "productId deve ser UUID valido")
				}
				selected = selectedID
			}
			resolvedIDs[index] = selected
			if sku := normalizeCatalogKey(snapshot.SKU); sku != "" {
				batchSKU[sku] = selected
			}
			for _, barcode := range snapshot.Barcodes {
				if barcode.Active != nil && *barcode.Active {
					batchBarcode[normalizeCatalogKey(barcode.Value)] = selected
				}
			}
			continue
		}
		var selected uuid.UUID
		if mapping, found := mappingByLocalCode[localCode]; found {
			if err := selectCandidate(&selected, mapping.ProductID); err != nil {
				return nil, err
			}
		}
		if snapshot.ProductID != "" {
			id, _ := uuid.Parse(snapshot.ProductID)
			if err := selectCandidate(&selected, id); err != nil {
				return nil, err
			}
		}
		if sku := normalizeCatalogKey(snapshot.SKU); sku != "" {
			if id, found := productBySKU[sku]; found {
				if err := selectCandidate(&selected, id); err != nil {
					return nil, err
				}
			}
			if id, found := batchSKU[sku]; found {
				if err := selectCandidate(&selected, id); err != nil {
					return nil, err
				}
			}
		}
		for _, barcode := range snapshot.Barcodes {
			if !*barcode.Active {
				continue
			}
			key := normalizeCatalogKey(barcode.Value)
			for _, existing := range barcodesByValue[key] {
				if existing.Active {
					if err := selectCandidate(&selected, existing.ProductID); err != nil {
						return nil, err
					}
				}
			}
			if id, found := batchBarcode[key]; found {
				if err := selectCandidate(&selected, id); err != nil {
					return nil, err
				}
			}
		}
		if selected == uuid.Nil {
			selected = uuid.New()
		}
		resolvedIDs[index] = selected
		if sku := normalizeCatalogKey(snapshot.SKU); sku != "" {
			batchSKU[sku] = selected
		}
		for _, barcode := range snapshot.Barcodes {
			if *barcode.Active {
				batchBarcode[normalizeCatalogKey(barcode.Value)] = selected
			}
		}
	}
	// A complete load from the matrix is authoritative for product identity.
	// Repair the stale split-EAN rows before building mappings/barcodes below;
	// this keeps the merge atomic and makes a retry idempotent.
	for winner, values := range matrixReassignments {
		if err := r.ReassignProductBarcodes(values, winner); err != nil {
			return nil, err
		}
	}
	if len(matrixLosers) > 0 {
		losers := make([]uuid.UUID, 0, len(matrixLosers))
		for loser := range matrixLosers {
			losers = append(losers, loser)
		}
		if err := r.RemoveProductMappings(identity.StoreID, losers); err != nil {
			return nil, err
		}
		storeMappings, err = r.ProductMappingsByLocalCodes(identity.StoreID, localCodes)
		if err != nil {
			return nil, err
		}
		mappingByLocalCode = make(map[string]models.ProductStoreMapping, len(storeMappings))
		for _, mapping := range storeMappings {
			mappingByLocalCode[normalizeCatalogKey(mapping.LocalCode)] = mapping
		}
		barcodes, err = r.ProductBarcodesByNormalizedValues(barcodeValues)
		if err != nil {
			return nil, err
		}
		barcodesByValue = make(map[string][]models.ProductBarcode, len(barcodes))
		for _, barcode := range barcodes {
			key := normalizeCatalogKey(barcode.NormalizedValue)
			barcodesByValue[key] = append(barcodesByValue[key], barcode)
		}
	}
	foreignIDs, err := r.ProductIDsOwnedByAnotherCompany(resolvedIDs)
	if err != nil {
		return nil, err
	}
	if len(foreignIDs) > 0 {
		return nil, models.Error(409, "PRODUCT_TENANT_CONFLICT: productId pertence a outra empresa")
	}

	storedProducts, err := r.ProductsByIDs(resolvedIDs)
	if err != nil {
		return nil, err
	}
	existingProductByID := make(map[uuid.UUID]models.Product, len(storedProducts))
	for _, product := range storedProducts {
		existingProductByID[product.ID] = product
	}
	existingMappings, err := r.ProductMappingsByProductIDs(identity.StoreID, resolvedIDs)
	if err != nil {
		return nil, err
	}
	existingBarcodes, err := r.ProductBarcodesForProducts(resolvedIDs)
	if err != nil {
		return nil, err
	}
	currentPrimaryByProduct := make(map[uuid.UUID]string, len(resolvedIDs))
	for _, barcode := range existingBarcodes {
		if barcode.Active && barcode.Primary {
			if _, exists := currentPrimaryByProduct[barcode.ProductID]; !exists {
				currentPrimaryByProduct[barcode.ProductID] = normalizeCatalogKey(barcode.NormalizedValue)
			}
		}
	}
	mappingByProductID := make(map[uuid.UUID]models.ProductStoreMapping, len(existingMappings))
	for _, mapping := range existingMappings {
		mappingByProductID[mapping.ProductID] = mapping
	}
	if reconcileCanonicalCodes {
		canonicalCodeByProduct := make(map[uuid.UUID]string, len(resolvedIDs))
		canonicalOwnerByCode := make(map[string]uuid.UUID, len(mappingByLocalCode)+len(mappingByProductID))
		for code, mapping := range mappingByLocalCode {
			canonicalOwnerByCode[code] = mapping.ProductID
		}
		for _, mapping := range mappingByProductID {
			if code := normalizeCatalogKey(mapping.LocalCode); code != "" {
				canonicalOwnerByCode[code] = mapping.ProductID
			}
		}
		nextGeneratedCode := ""
		generatedCodeLoaded := false
		codeIsOccupied := func(candidate string, productID uuid.UUID) (bool, error) {
			key := normalizeCatalogKey(candidate)
			if owner, exists := canonicalOwnerByCode[key]; exists && owner != productID {
				return true, nil
			}
			if mapping, exists := mappingByLocalCode[key]; exists && mapping.ProductID != productID {
				return true, nil
			}
			if _, loaded := canonicalOwnerByCode[key]; !loaded {
				mapping, found, err := r.ProductMapping(identity.StoreID, candidate)
				if err != nil {
					return false, err
				}
				if found {
					canonicalOwnerByCode[key] = mapping.ProductID
					if mapping.ProductID != productID {
						return true, nil
					}
				}
			}
			return false, nil
		}
		reserveCanonicalCode := func(candidate string, productID uuid.UUID) (string, error) {
			candidate = strings.TrimSpace(candidate)
			for attempt := 0; attempt < 1000000; attempt++ {
				if candidate == "" {
					return "", models.Error(409, "PRODUCT_LOCAL_CODE_EXHAUSTED: nao foi possivel reservar um codigo canonico livre")
				}
				occupied, err := codeIsOccupied(candidate, productID)
				if err != nil {
					return "", err
				}
				if !occupied {
					return candidate, nil
				}
				next := nextCanonicalProductCode(candidate, "")
				if next == "" {
					if !generatedCodeLoaded {
						nextGeneratedCode, err = r.NextProductCanonicalCode()
						if err != nil {
							return "", err
						}
						generatedCodeLoaded = true
					}
					next = nextCanonicalProductCode(candidate, nextGeneratedCode)
					if next != "" {
						nextGeneratedCode = nextCanonicalProductCode(next, "")
					}
				}
				candidate = next
			}
			return "", models.Error(409, "PRODUCT_LOCAL_CODE_EXHAUSTED: nao foi possivel reservar um codigo canonico livre")
		}
		// The first transaction that acquires the tenant advisory lock wins a
		// contested code. Every later product is moved to the next free canonical
		// code, and that chosen value is sent back to the source and to all stores.
		for index, productID := range resolvedIDs {
			if prior, exists := canonicalCodeByProduct[productID]; exists {
				snapshots[index].LocalCode = prior
				continue
			}
			candidate := strings.TrimSpace(snapshots[index].LocalCode)
			reserved, err := reserveCanonicalCode(candidate, productID)
			if err != nil {
				return nil, err
			}
			canonicalCodeByProduct[productID] = reserved
			canonicalOwnerByCode[normalizeCatalogKey(reserved)] = productID
			snapshots[index].LocalCode = reserved
		}
	}
	incomingLocalCodeByProduct := make(map[uuid.UUID]string, len(resolvedIDs))
	incomingProductByLocalCode := make(map[string]uuid.UUID, len(resolvedIDs))
	for index, productID := range resolvedIDs {
		localCode := normalizeCatalogKey(snapshots[index].LocalCode)
		if prior, exists := incomingLocalCodeByProduct[productID]; exists && prior != localCode {
			return nil, models.Error(409, "PRODUCT_LOCAL_CODE_CONFLICT: um produto nao pode usar dois codigos locais na mesma loja")
		}
		incomingLocalCodeByProduct[productID] = localCode
		if prior, exists := incomingProductByLocalCode[localCode]; exists && prior != productID {
			return nil, models.Error(409, "PRODUCT_LOCAL_CODE_CONFLICT: um codigo local nao pode apontar para dois produtos")
		}
		incomingProductByLocalCode[localCode] = productID
	}
	for localCode, mapping := range mappingByLocalCode {
		if productID, incoming := incomingProductByLocalCode[localCode]; incoming && mapping.ProductID != productID {
			return nil, models.Error(409, "PRODUCT_LOCAL_CODE_OCCUPIED: o codigo local ja pertence a outro produto")
		}
	}

	products := make([]models.Product, 0, len(snapshots))
	mappings := make([]models.ProductStoreMapping, 0, len(snapshots))
	productIDsForPrimaryReset := make([]uuid.UUID, 0)
	primaryResetSeen := make(map[uuid.UUID]struct{}, len(snapshots))
	barcodeWrites := make([]models.ProductBarcode, 0)
	authoritativeC000055 := make(map[uuid.UUID]bool, len(snapshots))
	keepC000055 := make(map[uuid.UUID]map[string]struct{}, len(snapshots))
	incomingBarcodeOwner := make(map[string]uuid.UUID, len(barcodeValues))
	for index, snapshot := range snapshots {
		productID := resolvedIDs[index]
		// The incoming snapshot is authoritative for the global SKU. An empty
		// SKU remains empty; never replace it with a store-local code or retain
		// a stale value from a different store.
		canonicalSKU := snapshot.SKU
		product := models.Product{
			Tenant: models.NewTenant(r.CompanyID()), SKU: canonicalSKU,
			EstoqueMinimo: snapshot.EstoqueMinimo, MargemMinima: snapshot.MargemMinima,
			FlagSis: snapshot.FlagSis, Name: snapshot.Name, Description: snapshot.Description,
			Unit: snapshot.Unit, Brand: snapshot.Brand, CodigoMarca: snapshot.CodigoMarca,
			DataCadastro: snapshot.DataCadastro,
			Aplicacao:    snapshot.Aplicacao, Origem: snapshot.Origem,
			NCM: snapshot.NCM, CEST: snapshot.CEST,
			ClassificacaoFiscal: snapshot.ClassificacaoFiscal, ClasseTP: snapshot.ClasseTP,
			CST: snapshot.CST, CodigoMS: snapshot.CodigoMS, PrincipioAtivo: snapshot.PrincipioAtivo,
			NomeLaboratorio: snapshot.NomeLaboratorio, CodigoFornecedor: snapshot.CodigoFornecedor,
			NomeFornecedor: snapshot.NomeFornecedor, CNPJFornecedor: snapshot.CNPJFornecedor,
			NomeFantasiaFornecedor: snapshot.NomeFantasiaFornecedor, Tipo: snapshot.Tipo,
			FarmaciaControlado:          snapshot.FarmaciaControlado,
			FarmaciaApresentacao:        snapshot.FarmaciaApresentacao,
			FarmaciaRegistroMedicamento: snapshot.FarmaciaRegistroMedicamento,
			Apresentacao:                snapshot.Apresentacao, CodigoOriginal: snapshot.CodigoOriginal,
			CSOSN: snapshot.CSOSN, UsaLote: snapshot.UsaLote, UsaBalanca: snapshot.UsaBalanca,
			CodigoReceita: snapshot.CodigoReceita, CodigoBarraNovartis: snapshot.CodigoBarraNovartis,
			SituacaoTributaria: snapshot.SituacaoTributaria, PISCOFINS: snapshot.PISCOFINS,
			IncidenciaPISCOFINS: snapshot.IncidenciaPISCOFINS, IAT: snapshot.IAT, IPPT: snapshot.IPPT,
			IndCFOPVendaDentro:            snapshot.IndCFOPVendaDentro,
			CodigoClassificacaoTributaria: snapshot.CodigoClassificacaoTributaria,
			CSTIBS:                        snapshot.CSTIBS, CSTCBS: snapshot.CSTCBS,
			AliquotaIBS: snapshot.AliquotaIBS, AliquotaCBS: snapshot.AliquotaCBS,
			AliquotaEfetivaIBS:   snapshot.AliquotaEfetivaIBS,
			AliquotaEfetivaCBS:   snapshot.AliquotaEfetivaCBS,
			ObsReformaTributaria: snapshot.ObsReformaTributaria,
			PrecoVenda:           snapshot.PrecoVenda,
			PrecoVenda1:          snapshot.PrecoVenda1, PrecoVenda2: snapshot.PrecoVenda2,
			PrecoVenda3: snapshot.PrecoVenda3, PrecoVenda4: snapshot.PrecoVenda4,
			PrecoVenda5: snapshot.PrecoVenda5, PrecoPromocao: snapshot.PrecoPromocao,
			MargemDesconto: snapshot.MargemDesconto, Aliquota: snapshot.Aliquota,
			QtdeEmbalagem:      snapshot.QtdeEmbalagem,
			DescontoPrecoVenda: snapshot.DescontoPrecoVenda,
			FarmaciaPMC:        snapshot.FarmaciaPMC, DataPromocao: snapshot.DataPromocao,
			FimPromocao: snapshot.FimPromocao, UsaTabelaPreco: snapshot.UsaTabelaPreco,
			PMargem1: snapshot.PMargem1,
			PMargem2: snapshot.PMargem2, PMargem3: snapshot.PMargem3,
			PMargem4: snapshot.PMargem4, PMargem5: snapshot.PMargem5,
			PrecoCustoAnterior: snapshot.PrecoCustoAnterior, PrecoVendaAnterior: snapshot.PrecoVendaAnterior,
			PrecoFarmaPop: snapshot.PrecoFarmaPop, AliquotaPIS: snapshot.AliquotaPIS,
			AliquotaCOFINS: snapshot.AliquotaCOFINS, PFCP: snapshot.PFCP, PFCPST: snapshot.PFCPST,
			Active: *snapshot.Active, TracksSerial: *snapshot.TracksSerial,
			UsaGrade: snapshot.UsaGrade != nil && *snapshot.UsaGrade,
		}
		if snapshot.Group != nil {
			product.GroupName = snapshot.Group.Name
		}
		if snapshot.Subgroup != nil {
			product.SubgroupName = snapshot.Subgroup.Name
		}
		product.ID = productID
		if existing, found := existingProductByID[productID]; found {
			product.CreatedAt = existing.CreatedAt
		}
		products = append(products, product)

		groupLocalCode, groupName := "", ""
		if snapshot.Group != nil {
			groupLocalCode, groupName = snapshot.Group.LocalCode, snapshot.Group.Name
		}
		subgroupLocalCode, subgroupName := "", ""
		if snapshot.Subgroup != nil {
			subgroupLocalCode, subgroupName = snapshot.Subgroup.LocalCode, snapshot.Subgroup.Name
		}
		localCode := normalizeCatalogKey(snapshot.LocalCode)
		mapping, found := mappingByLocalCode[localCode]
		if found && mapping.ProductID != productID {
			return nil, models.Error(409, "PRODUCT_LOCAL_CODE_OCCUPIED: o codigo local ja pertence a outro produto")
		}
		if !found {
			// Reuse a canonical product mapping that is currently under an
			// older local code. This is a code move, not a second mapping.
			mapping, found = mappingByProductID[productID]
		}
		if !found {
			mapping = models.ProductStoreMapping{Tenant: models.NewTenant(r.CompanyID()), StoreID: identity.StoreID}
		}
		mapping.CompanyID = r.CompanyID()
		mapping.StoreID = identity.StoreID
		mapping.ProductID = productID
		mapping.LocalCode = snapshot.LocalCode
		mapping.GroupLocalCode = groupLocalCode
		mapping.GroupName = groupName
		mapping.SubgroupLocalCode = subgroupLocalCode
		mapping.SubgroupName = subgroupName
		mappings = append(mappings, mapping)
		incomingPrimaryValue := ""
		for _, incoming := range snapshot.Barcodes {
			if incoming.Active != nil && *incoming.Active && incoming.Primary != nil && *incoming.Primary {
				incomingPrimaryValue = normalizeCatalogKey(incoming.Value)
				break
			}
		}

		for _, incoming := range snapshot.Barcodes {
			value := strings.TrimSpace(incoming.Value)
			normalized := normalizeCatalogKey(value)
			if incoming.C000055 != nil {
				authoritativeC000055[productID] = true
			}
			for _, existing := range barcodesByValue[normalized] {
				if existing.Active && existing.ProductID != productID {
					return nil, models.Error(409, "BARCODE_ALREADY_ASSIGNED: codigo de barras ativo pertence a outro produto")
				}
			}
			if *incoming.Active {
				if owner, exists := incomingBarcodeOwner[normalized]; exists && owner != productID {
					return nil, models.Error(409, "BARCODE_ALREADY_ASSIGNED: codigo de barras ativo pertence a outro produto")
				}
				incomingBarcodeOwner[normalized] = productID
			}
			barcode := models.ProductBarcode{}
			foundBarcode := false
			for _, existing := range barcodesByValue[normalized] {
				if existing.ProductID == productID {
					barcode = existing
					foundBarcode = true
					break
				}
			}
			if !foundBarcode {
				barcode.Tenant = models.NewTenant(r.CompanyID())
			}
			barcode.CompanyID = r.CompanyID()
			barcode.ProductID = productID
			barcode.Value = value
			barcode.NormalizedValue = normalized
			barcode.Primary = effectiveProductBarcodePrimary(
				currentPrimaryByProduct[productID], normalized, incomingPrimaryValue,
				*incoming.Active, *incoming.Primary, sourceIsMatrix,
			)
			barcode.Active = *incoming.Active
			barcode.Fraction = incoming.Fraction
			barcode.C000055 = incoming.C000055 != nil && *incoming.C000055
			if incoming.C000055 == nil {
				// Legacy payloads represented every non-primary barcode as C000055.
				barcode.C000055 = incoming.Primary == nil || !*incoming.Primary
			}
			barcode.C000055 = barcode.C000055 && barcode.Active
			if barcode.C000055 {
				if keepC000055[productID] == nil {
					keepC000055[productID] = make(map[string]struct{})
				}
				keepC000055[productID][normalized] = struct{}{}
			}
			barcodeWrites = append(barcodeWrites, barcode)
			if barcode.Primary && barcode.Active {
				currentPrimaryByProduct[productID] = normalized
				if _, exists := primaryResetSeen[productID]; !exists {
					productIDsForPrimaryReset = append(productIDsForPrimaryReset, productID)
					primaryResetSeen[productID] = struct{}{}
				}
			}
		}
	}

	if err := r.UpsertProducts(products); err != nil {
		return nil, err
	}
	if err := r.UpsertProductMappings(mappings); err != nil {
		return nil, err
	}
	if err := r.ClearPrimaryProductBarcodes(productIDsForPrimaryReset); err != nil {
		return nil, err
	}
	if err := r.UpsertProductBarcodes(barcodeWrites); err != nil {
		return nil, err
	}
	for productID := range authoritativeC000055 {
		keep := make([]string, 0, len(keepC000055[productID]))
		for normalized := range keepC000055[productID] {
			keep = append(keep, normalized)
		}
		if err := r.DeleteProductC00055Except(productID, keep); err != nil {
			return nil, err
		}
	}
	result := make([]models.CatalogWriteResult, 0, len(snapshots))
	for index, snapshot := range snapshots {
		result = append(result, models.CatalogWriteResult{
			ID: resolvedIDs[index], LocalCode: snapshot.LocalCode, SKU: products[index].SKU,
		})
	}
	return result, nil
}
