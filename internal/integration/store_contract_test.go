package integration

import (
	"testing"

	"github.com/google/uuid"
	"github.com/phsoftwares/storelink-backend/internal/models"
)

// TestStoreCNPJIsMandatoryAndUnique protects the identity used by a shared
// FNTS_Server credential. A store without CNPJ must never become addressable
// by an agent, and punctuation differences must not create a second store.
func TestStoreCNPJIsMandatoryAndUnique(t *testing.T) {
	h := newHarness(t)
	var user models.User
	if err := h.db.First(&user, "id = ?", h.user).Error; err != nil {
		t.Fatal(err)
	}
	_, login := h.request("POST", "/api/auth/login", models.LoginDTO{
		Email: user.Email, Senha: "password-de-teste-segura",
	}, "", "")
	admin, ok := login["token"].(string)
	if !ok || admin == "" {
		t.Fatalf("login de teste invalido: %#v", login)
	}
	company := bodyID(h, "POST", "/api/companies", models.CompanyDTO{
		Nome: "Empresa CNPJ obrigatorio", CNPJ: "88888888000188",
	}, admin, "")
	group := bodyID(h, "POST", "/api/groups", models.GroupDTO{
		Nome: "Grupo CNPJ obrigatorio",
	}, admin, company)

	h.statusRequest("POST", "/api/stores", map[string]interface{}{
		"groupId": group,
		"codigo":  "001",
		"nome":    "Loja sem CNPJ",
		"tipo":    "MATRIZ",
	}, admin, company, 400)

	store := bodyID(h, "POST", "/api/stores", models.StoreDTO{
		GroupID: mustUUID(t, group), Codigo: "001", Nome: "Loja com CNPJ",
		CNPJ: "11.111.111/0001-11", Tipo: "MATRIZ",
	}, admin, company)
	if store == "" {
		t.Fatal("loja criada sem id")
	}
	h.statusRequest("POST", "/api/stores", models.StoreDTO{
		GroupID: mustUUID(t, group), Codigo: "002", Nome: "Duplicata CNPJ",
		CNPJ: "11111111000111", Tipo: "FILIAL",
	}, admin, company, 409)

	var saved models.Store
	if err := h.db.First(&saved, "id = ?", store).Error; err != nil {
		t.Fatal(err)
	}
	if saved.CNPJ != "11111111000111" || saved.ID != mustUUID(t, store) {
		t.Fatalf("CNPJ nao foi normalizado no banco: %+v", saved)
	}
	if saved.GroupID == uuid.Nil {
		t.Fatal("loja criada sem grupo")
	}
}
