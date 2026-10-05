package services

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/phsoftwares/storelink-backend/internal/models"
)

func TestNormalizeProductBatchSnapshotValidationAndNormalization(t *testing.T) {
	active, inactive := true, false
	primary, barcodeActive := true, true
	valid := func() models.ProductSnapshot {
		return models.ProductSnapshot{
			LocalCode: " P-1 ", SKU: " SKU-1 ", Name: " Product ", Unit: " UN ",
			Active: &active, TracksSerial: &inactive,
			Group:    &models.ProductCategoryValue{LocalCode: " G-1 ", Name: " Group "},
			Subgroup: &models.ProductCategoryValue{LocalCode: " SG-1 ", Name: " Subgroup "},
			Barcodes: []models.ProductBarcodeValue{{Value: " 12345 ", Primary: &primary, Active: &barcodeActive}},
		}
	}
	t.Run("normalizes all identifiers and optional category data", func(t *testing.T) {
		snapshot := valid()
		if err := normalizeProductBatchSnapshot(&snapshot); err != nil {
			t.Fatal(err)
		}
		if snapshot.LocalCode != "P-1" || snapshot.SKU != "SKU-1" || snapshot.Name != "Product" || snapshot.Unit != "UN" || snapshot.Group.LocalCode != "G-1" || snapshot.Group.Name != "Group" || snapshot.Subgroup.LocalCode != "SG-1" || snapshot.Subgroup.Name != "Subgroup" || snapshot.Barcodes[0].Value != "12345" {
			t.Fatalf("snapshot was not normalized: %+v", snapshot)
		}
	})
	t.Run("empty barcode collection becomes an explicit empty slice", func(t *testing.T) {
		snapshot := valid()
		snapshot.Barcodes = nil
		if err := normalizeProductBatchSnapshot(&snapshot); err != nil || snapshot.Barcodes == nil || len(snapshot.Barcodes) != 0 {
			t.Fatalf("empty barcode normalization failed: %+v, %v", snapshot.Barcodes, err)
		}
	})
	invalid := []struct {
		name   string
		change func(*models.ProductSnapshot)
	}{
		{"missing active", func(s *models.ProductSnapshot) { s.Active = nil }},
		{"missing serial boolean", func(s *models.ProductSnapshot) { s.TracksSerial = nil }},
		{"empty local code", func(s *models.ProductSnapshot) { s.LocalCode = " " }},
		{"oversized name", func(s *models.ProductSnapshot) { s.Name = strings.Repeat("x", 256) }},
		{"oversized description", func(s *models.ProductSnapshot) { s.Description = strings.Repeat("x", 1001) }},
		{"oversized unit", func(s *models.ProductSnapshot) { s.Unit = strings.Repeat("x", 21) }},
		{"oversized sku", func(s *models.ProductSnapshot) { s.SKU = strings.Repeat("x", 101) }},
		{"oversized brand", func(s *models.ProductSnapshot) { s.Brand = strings.Repeat("x", 101) }},
		{"oversized ncm", func(s *models.ProductSnapshot) { s.NCM = strings.Repeat("x", 21) }},
		{"oversized cest", func(s *models.ProductSnapshot) { s.CEST = strings.Repeat("x", 21) }},
		{"missing barcode primary flag", func(s *models.ProductSnapshot) { s.Barcodes[0].Primary = nil }},
		{"missing barcode active flag", func(s *models.ProductSnapshot) { s.Barcodes[0].Active = nil }},
		{"empty barcode", func(s *models.ProductSnapshot) { s.Barcodes[0].Value = " " }},
		{"oversized barcode", func(s *models.ProductSnapshot) { s.Barcodes[0].Value = strings.Repeat("x", 33) }},
		{"duplicate normalized barcodes", func(s *models.ProductSnapshot) {
			s.Barcodes = append(s.Barcodes, models.ProductBarcodeValue{Value: " 12345 ", Primary: &primary, Active: &barcodeActive})
		}},
		{"negative barcode fraction", func(s *models.ProductSnapshot) { s.Barcodes[0].Fraction = floatRef(-0.01) }},
		{"excessive barcode fraction", func(s *models.ProductSnapshot) { s.Barcodes[0].Fraction = floatRef(1000001) }},
		{"multiple primary barcodes", func(s *models.ProductSnapshot) {
			s.Barcodes = append(s.Barcodes, models.ProductBarcodeValue{Value: "67890", Primary: &primary, Active: &barcodeActive})
		}},
		{"invalid product id", func(s *models.ProductSnapshot) { s.ProductID = "not-a-uuid" }},
		{"zero product id", func(s *models.ProductSnapshot) { s.ProductID = uuid.Nil.String() }},
		{"oversized group code", func(s *models.ProductSnapshot) { s.Group.LocalCode = strings.Repeat("x", 101) }},
		{"oversized group name", func(s *models.ProductSnapshot) { s.Group.Name = strings.Repeat("x", 101) }},
		{"oversized subgroup code", func(s *models.ProductSnapshot) { s.Subgroup.LocalCode = strings.Repeat("x", 101) }},
		{"oversized subgroup name", func(s *models.ProductSnapshot) { s.Subgroup.Name = strings.Repeat("x", 101) }},
	}
	for _, test := range invalid {
		t.Run(test.name, func(t *testing.T) {
			snapshot := valid()
			test.change(&snapshot)
			if err := normalizeProductBatchSnapshot(&snapshot); err == nil {
				t.Fatal("expected invalid product snapshot to fail")
			}
		})
	}
}

func TestCatalogSnapshotValidators(t *testing.T) {
	active, inactive := true, false
	t.Run("product path", func(t *testing.T) {
		valid := models.ProductSnapshot{LocalCode: "P-1"}
		if err := validateProductPath(valid, " P-1 "); err != nil {
			t.Fatal(err)
		}
		for _, path := range []string{"", strings.Repeat("x", 101), "P-2"} {
			if err := validateProductPath(valid, path); err == nil {
				t.Errorf("path %q should fail", path)
			}
		}
	})
	t.Run("customer payload", func(t *testing.T) {
		valid := models.CustomerSnapshot{LocalCode: "C-1", Name: "Customer", TaxIdentifier: "529.982.247-25", Email: "customer@example.test", CreditStatus: "standard"}
		if err := validateCustomerSnapshot(valid); err != nil {
			t.Fatal(err)
		}
		invalid := []models.CustomerSnapshot{
			{LocalCode: " ", Name: "Customer", CreditStatus: "STANDARD"},
			{LocalCode: "C-1", Name: " ", CreditStatus: "STANDARD"},
			{LocalCode: "C-1", Name: strings.Repeat("x", 256), CreditStatus: "STANDARD"},
			{LocalCode: "C-1", Name: "Customer", TaxIdentifier: strings.Repeat("x", 21), CreditStatus: "STANDARD"},
			{LocalCode: "C-1", Name: "Customer", Phone: strings.Repeat("x", 51), CreditStatus: "STANDARD"},
			{LocalCode: "C-1", Name: "Customer", Mobile: strings.Repeat("x", 51), CreditStatus: "STANDARD"},
			{LocalCode: "C-1", Name: "Customer", Email: strings.Repeat("x", 255), CreditStatus: "STANDARD"},
			{LocalCode: "C-1", Name: "Customer", CreditStatus: "INVALID"},
			{LocalCode: "C-1", Name: "Customer", Email: "bad address", CreditStatus: "STANDARD"},
			{LocalCode: "C-1", Name: "Customer", TaxIdentifier: "1234", CreditStatus: "STANDARD"},
			{LocalCode: "C-1", Name: "Customer", CreditStatus: "STANDARD", Address: models.Address{Street: strings.Repeat("x", 256)}},
		}
		for index, snapshot := range invalid {
			if err := validateCustomerSnapshot(snapshot); err == nil {
				t.Errorf("invalid customer case %d should fail", index)
			}
		}
	})
	t.Run("employee payload", func(t *testing.T) {
		valid := models.EmployeeSnapshot{LocalCode: "E-1", Name: "Employee", Active: &inactive}
		if err := validateEmployeeSnapshot(valid); err != nil {
			t.Fatal(err)
		}
		operator := models.CashierOperatorValue{LocalCode: "O-1", EmployeeLocalCode: "E-2", CanOpenGeneralCash: &active, CanViewAllReports: &active, BlindClosing: &inactive}
		invalid := []models.EmployeeSnapshot{
			{LocalCode: " ", Name: "Employee", Active: &active},
			{LocalCode: "E-1", Name: " ", Active: &active},
			{LocalCode: "E-1", Name: strings.Repeat("x", 256), Active: &active},
			{LocalCode: "E-1", Name: "Employee", Active: nil},
			{LocalCode: "E-1", Name: "Employee", Email: "bad address", Active: &active},
			{LocalCode: "E-1", Name: "Employee", Document: "1234", Active: &active},
			{LocalCode: "E-1", Name: "Employee", Active: &active, CashierOperators: []models.CashierOperatorValue{operator}},
			{LocalCode: "E-1", Name: "Employee", Active: &active, CashierOperators: []models.CashierOperatorValue{{LocalCode: "O-1", CanOpenGeneralCash: nil, CanViewAllReports: &active, BlindClosing: &active}}},
			{LocalCode: "E-1", Name: "Employee", Active: &active, CashierOperators: []models.CashierOperatorValue{{LocalCode: "O-1", CanOpenGeneralCash: &active, CanViewAllReports: &active, BlindClosing: &active}, {LocalCode: "o-1", CanOpenGeneralCash: &active, CanViewAllReports: &active, BlindClosing: &active}}},
		}
		for index, snapshot := range invalid {
			if err := validateEmployeeSnapshot(snapshot); err == nil {
				t.Errorf("invalid employee case %d should fail", index)
			}
		}
	})
	t.Run("cashier operator flags and address limits", func(t *testing.T) {
		if err := validateCashierOperator("O-1", &active, &inactive, &active); err != nil {
			t.Fatal(err)
		}
		if err := validateCashierOperator(" ", &active, &inactive, &active); err == nil {
			t.Fatal("blank cashier code should fail")
		}
		if err := validateCashierOperator("O-1", nil, &inactive, &active); err == nil {
			t.Fatal("missing cashier boolean should fail")
		}
		if err := validateAddress(models.Address{State: "SP", PostalCode: "01001000"}); err != nil {
			t.Fatal(err)
		}
		if err := validateAddress(models.Address{Street: strings.Repeat("x", 256)}); err == nil {
			t.Fatal("oversized address should fail")
		}
	})
}

func TestSharedCatalogInputHelpers(t *testing.T) {
	if err := validateBatchSize(1); err != nil {
		t.Fatal(err)
	}
	for _, size := range []int{0, 1001} {
		if err := validateBatchSize(size); err == nil {
			t.Errorf("batch size %d should fail", size)
		}
	}
	seen := make(map[string]struct{})
	if err := uniqueLocalCode(seen, " C-1 "); err != nil {
		t.Fatal(err)
	}
	for _, code := range []string{"c-1", " "} {
		if err := uniqueLocalCode(seen, code); err == nil {
			t.Errorf("local code %q should fail", code)
		}
	}
	if _, err := parseCanonicalID(uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{"bad-id", uuid.Nil.String()} {
		if _, err := parseCanonicalID(raw); err == nil {
			t.Errorf("canonical ID %q should fail", raw)
		}
	}
	if got := onlyDigits("529.982.247-25"); got != "52998224725" {
		t.Fatalf("onlyDigits() = %q", got)
	}
}
