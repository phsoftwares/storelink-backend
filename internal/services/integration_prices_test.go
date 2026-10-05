package services

import (
	"context"
	"math"
	"strings"
	"testing"

	"github.com/phsoftwares/storelink-backend/internal/models"
)

func TestNormalizeAndValidateProductPrice(t *testing.T) {
	active := false
	amount := 0.0
	tests := []struct {
		name    string
		price   models.ProductPriceSnapshot
		wantErr bool
	}{
		{name: "valid zero amount and false active", price: models.ProductPriceSnapshot{LocalCode: " P-1 ", Kind: " sale ", Quantity: 1.2345, Amount: &amount, Active: &active}},
		{name: "missing local code", price: models.ProductPriceSnapshot{Kind: "SALE", Quantity: 1, Amount: &amount, Active: &active}, wantErr: true},
		{name: "local code too long", price: models.ProductPriceSnapshot{LocalCode: strings.Repeat("x", 101), Kind: "SALE", Quantity: 1, Amount: &amount, Active: &active}, wantErr: true},
		{name: "unsupported kind", price: models.ProductPriceSnapshot{LocalCode: "P-1", Kind: "PROMO", Quantity: 1, Amount: &amount, Active: &active}, wantErr: true},
		{name: "quantity must be positive", price: models.ProductPriceSnapshot{LocalCode: "P-1", Kind: "SALE", Amount: &amount, Active: &active}, wantErr: true},
		{name: "quantity precision", price: models.ProductPriceSnapshot{LocalCode: "P-1", Kind: "SALE", Quantity: 1.00001, Amount: &amount, Active: &active}, wantErr: true},
		{name: "amount is required", price: models.ProductPriceSnapshot{LocalCode: "P-1", Kind: "SALE", Quantity: 1, Active: &active}, wantErr: true},
		{name: "negative amount", price: models.ProductPriceSnapshot{LocalCode: "P-1", Kind: "SALE", Quantity: 1, Amount: floatRef(-1), Active: &active}, wantErr: true},
		{name: "amount precision", price: models.ProductPriceSnapshot{LocalCode: "P-1", Kind: "SALE", Quantity: 1, Amount: floatRef(1.00001), Active: &active}, wantErr: true},
		{name: "active is required", price: models.ProductPriceSnapshot{LocalCode: "P-1", Kind: "SALE", Quantity: 1, Amount: &amount}, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := normalizeAndValidateProductPrice(&test.price)
			if (err != nil) != test.wantErr {
				t.Fatalf("normalizeAndValidateProductPrice() error = %v, wantErr %v", err, test.wantErr)
			}
			if !test.wantErr && (test.price.LocalCode != "P-1" || test.price.Kind != "SALE" || *test.price.Amount != 0 || *test.price.Active) {
				t.Fatalf("normalização incorreta: %+v", test.price)
			}
		})
	}
}

func TestPriceNumberHelpers(t *testing.T) {
	tests := []struct {
		name             string
		value            float64
		strictlyPositive bool
		valid            bool
	}{
		{name: "zero amount", value: 0, valid: true},
		{name: "zero quantity", value: 0, strictlyPositive: true, valid: false},
		{name: "negative", value: -1, valid: false},
		{name: "nan", value: math.NaN(), valid: false},
		{name: "infinite", value: math.Inf(1), valid: false},
		{name: "above database precision", value: maxPriceValue + 100, valid: false},
		{name: "more than four decimals", value: 1.00001, valid: false},
		{name: "quantity", value: 2.5, strictlyPositive: true, valid: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := validPriceNumber(test.value, test.strictlyPositive); got != test.valid {
				t.Fatalf("validPriceNumber(%v, %t) = %t, want %t", test.value, test.strictlyPositive, got, test.valid)
			}
		})
	}
	if got := roundPriceNumber(1.23456); got != 1.2346 {
		t.Fatalf("roundPriceNumber() = %v, want 1.2346", got)
	}
	if got := normalizedPriceQuantity(2.5); got != "2.5000" {
		t.Fatalf("normalizedPriceQuantity() = %q, want 2.5000", got)
	}
	if !samePriceNumber(2.5, 2.5000) || samePriceNumber(0, 1) || samePriceNumber(1, 0) || samePriceNumber(1, 1.00001) {
		t.Fatal("samePriceNumber() did not enforce positive, four-decimal values")
	}
}

func TestProductPriceServiceRejectsInvalidRequestsBeforeDatabase(t *testing.T) {
	service := &Service{}
	identity := models.Identity{}
	price := models.ProductPriceSnapshot{LocalCode: "P-1", Kind: "SALE", Quantity: 1, Amount: floatRef(10), Active: boolRef(true)}
	if _, err := service.UpsertProductPrice(context.Background(), identity, price, "P-1", "SALE", "NaN"); err == nil {
		t.Fatal("quantity não numérica deveria ser rejeitada")
	}
	if _, err := service.UpsertProductPrice(context.Background(), identity, price, "P-2", "SALE", "1"); err == nil {
		t.Fatal("localCode diferente da rota deveria ser rejeitado")
	}
	if _, err := service.UpsertProductPrice(context.Background(), identity, price, "P-1", "WHOLESALE", "1"); err == nil {
		t.Fatal("kind diferente da rota deveria ser rejeitado")
	}
	if _, err := service.UpsertProductPrice(context.Background(), identity, price, "P-1", "SALE", "2"); err == nil {
		t.Fatal("quantity diferente da rota deveria ser rejeitado")
	}
	if _, err := service.UpsertProductPrices(context.Background(), identity, nil); err == nil {
		t.Fatal("lote vazio deveria ser rejeitado")
	}
	if _, err := service.UpsertProductPrices(context.Background(), identity, make([]models.ProductPriceSnapshot, 1001)); err == nil {
		t.Fatal("lote acima do limite deveria ser rejeitado")
	}
	invalidPrice := models.ProductPriceSnapshot{LocalCode: "P-1", Kind: "SALE", Quantity: 1, Active: boolRef(true)}
	if _, err := service.UpsertProductPrices(context.Background(), identity, []models.ProductPriceSnapshot{invalidPrice}); err == nil {
		t.Fatal("lote com preço incompleto deveria ser rejeitado antes do acesso ao banco")
	}
}

func floatRef(value float64) *float64 { return &value }
func boolRef(value bool) *bool        { return &value }
