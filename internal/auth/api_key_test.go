package auth

import "testing"

func TestGeneratedAPIKeyIsFixedHighEntropySecretAndCompares(t *testing.T) {
	key, hash, err := GenerateAPIKey()
	if err != nil {
		t.Fatal(err)
	}
	if len(key) != 64 {
		t.Fatalf("generated key length = %d, want 64 hex characters", len(key))
	}
	if hash == key || len(hash) != 64 {
		t.Fatalf("database hash should be a separate SHA-256 value, got %q", hash)
	}
	if !CompareAPIKey(hash, key) {
		t.Fatal("generated key did not match its stored hash")
	}
	if CompareAPIKey(hash, key+"wrong") {
		t.Fatal("different key matched")
	}
	otherKey, _, err := GenerateAPIKey()
	if err != nil {
		t.Fatal(err)
	}
	if key == otherKey {
		t.Fatal("two generated keys should be independent")
	}
}
