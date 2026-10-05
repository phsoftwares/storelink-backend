package services

import (
	"testing"

	"github.com/phsoftwares/storelink-backend/internal/models"
)

func TestEffectiveProductBarcodePrimaryKeepsCanonicalEAN(t *testing.T) {
	tests := []struct {
		name            string
		current         string
		incoming        string
		incomingPrimary string
		active          bool
		markedPrimary   bool
		matrix          bool
		want            bool
	}{
		{name: "first primary", incoming: "EAN-A", incomingPrimary: "EAN-A", active: true, markedPrimary: true, want: true},
		{name: "filial cannot replace canonical", current: "EAN-A", incoming: "EAN-B", incomingPrimary: "EAN-B", active: true, markedPrimary: true, want: false},
		{name: "matrix can correct canonical", current: "EAN-A", incoming: "EAN-B", incomingPrimary: "EAN-B", active: true, markedPrimary: true, matrix: true, want: true},
		{name: "filial keeps canonical when it sends it as alternate", current: "EAN-A", incoming: "EAN-A", incomingPrimary: "EAN-B", active: true, markedPrimary: false, want: true},
		{name: "matrix declared replacement wins over old alternate", current: "EAN-A", incoming: "EAN-A", incomingPrimary: "EAN-B", active: true, markedPrimary: false, matrix: true, want: false},
		{name: "snapshot without primary preserves canonical", current: "EAN-A", incoming: "EAN-A", active: true, markedPrimary: false, want: true},
		{name: "inactive barcode is never primary", current: "EAN-A", incoming: "EAN-B", incomingPrimary: "EAN-B", markedPrimary: true, matrix: true, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := effectiveProductBarcodePrimary(test.current, test.incoming, test.incomingPrimary, test.active, test.markedPrimary, test.matrix)
			if got != test.want {
				t.Fatalf("effective primary = %v, want %v", got, test.want)
			}
		})
	}
}

func TestActiveEANIsAProductIdentity(t *testing.T) {
	active := true
	inactive := false
	withoutEAN := models.ProductSnapshot{Barcodes: []models.ProductBarcodeValue{{Value: "123", Active: &inactive}}}
	withEAN := models.ProductSnapshot{Barcodes: []models.ProductBarcodeValue{{Value: "123", Active: &active}}}
	if hasActiveProductBarcode(withoutEAN) {
		t.Fatal("inativo nao pode ser identidade EAN")
	}
	if !hasActiveProductBarcode(withEAN) {
		t.Fatal("EAN ativo deve ser identidade forte")
	}
}

func TestCanonicalCodeCollisionUsesSequenceAndGeneratedFallback(t *testing.T) {
	if got := nextCanonicalProductCode("000100", "000001"); got != "000101" {
		t.Fatalf("proximo codigo numerico = %q", got)
	}
	if got := nextCanonicalProductCode("Y", "000001"); got != "000001" {
		t.Fatalf("fallback para codigo textual = %q", got)
	}
	if got := nextCanonicalProductCode("999999", ""); got != "" {
		t.Fatalf("sequencia esgotada = %q", got)
	}
}
