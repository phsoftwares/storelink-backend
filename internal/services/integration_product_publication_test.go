package services

import (
	"encoding/json"
	"testing"

	"github.com/phsoftwares/storelink-backend/internal/models"
)

func TestPublicationVersionIsSafeForLegacyJSONClients(t *testing.T) {
	value := publicationVersion("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff")
	const maxExactJSONInteger int64 = 9007199254740991
	if value <= 0 || value > maxExactJSONInteger {
		t.Fatalf("publication version %d is outside the exact JSON number range", value)
	}
	if value != publicationVersion("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff") {
		t.Fatal("publication version must be deterministic")
	}
}

func TestNextLocalCodeCandidateSkipsWithFixedWidth(t *testing.T) {
	if got := nextLocalCodeCandidate("005782"); got != "005783" {
		t.Fatalf("candidate after 005782 = %q", got)
	}
	if got := nextLocalCodeCandidate("999999"); got != "" {
		t.Fatalf("candidate after upper bound = %q", got)
	}
	if got := nextLocalCodeCandidate("ABC"); got != "" {
		t.Fatalf("candidate after nonnumeric code = %q", got)
	}
}

func TestProductPublicationPayloadKeepsExplicitZeroSalePrice(t *testing.T) {
	payload := productPublicationPayload{
		Event: "UPDATED",
		Product: models.ProductSnapshot{
			LocalCode:           "001522",
			Name:                "Produto com preco zero",
			PrecoVenda:          0,
			PrecoVendaInformado: true,
		},
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal publication: %v", err)
	}
	var decoded struct {
		Product map[string]json.RawMessage `json:"produto"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal publication: %v", err)
	}
	if _, exists := decoded.Product["preco_venda"]; exists {
		t.Fatal("zero price should remain omitted and use its explicit presence flag")
	}
	var informed bool
	if err := json.Unmarshal(decoded.Product["preco_venda_informado"], &informed); err != nil {
		t.Fatalf("unmarshal price presence flag: %v", err)
	}
	if !informed {
		t.Fatal("explicit zero sale price was lost from the queued product payload")
	}
}
