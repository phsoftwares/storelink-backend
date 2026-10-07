package services

import "testing"
import "github.com/google/uuid"
import "github.com/phsoftwares/storelink-backend/internal/models"

func TestUserControlIdentityKeyIsStableAndRequired(t *testing.T) {
	key, err := userControlIdentityKey("USERCONTROL_USUARIO", map[string]interface{}{
		"chave_identidade": "  uc|user|login|ana  ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if key != "UC|USER|LOGIN|ANA" {
		t.Fatalf("identidade UserControl nao foi normalizada: %q", key)
	}

	if _, err := userControlIdentityKey("USERCONTROL_USUARIO", map[string]interface{}{}); err == nil {
		t.Fatal("UserControl sem chave estavel deveria ser rejeitado")
	}
	if key, err := userControlIdentityKey("CUSTOMER", map[string]interface{}{}); err != nil || key != "" {
		t.Fatalf("entidade comum nao deveria exigir chave UserControl: %q, %v", key, err)
	}
}

func TestUserControlPublicationIDIgnoresLocalUserID(t *testing.T) {
	companyID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	first := userControlPublicationID(companyID, "USERCONTROL_USUARIO", "UC|USER|LOGIN|ANA")
	second := userControlPublicationID(companyID, "USERCONTROL_USUARIO", "UC|USER|LOGIN|ANA")
	if first != second {
		t.Fatalf("identidade global UserControl mudou para o mesmo usuario")
	}
	other := userControlPublicationID(companyID, "USERCONTROL_USUARIO", "UC|USER|LOGIN|BRUNO")
	if first == other {
		t.Fatalf("usuarios UserControl distintos receberam o mesmo ID global")
	}
}

func TestUserControlUsesDedicatedGroupIntegrationFlag(t *testing.T) {
	group := models.Group{IntegrarFuncionarios: true, IntegrarUsuarios: false}
	for _, entity := range []string{"USERCONTROL_USUARIO", "USERCONTROL_DIREITO", "USERCONTROL_DIREITO_EXTRA"} {
		if err := requireEntityIntegration(group, entity); err == nil {
			t.Fatalf("entidade %s deveria respeitar integrarUsuarios desligado", entity)
		}
	}
	if err := requireEntityIntegration(group, "users"); err == nil {
		t.Fatal("o alias da publicacao direta deveria respeitar integrarUsuarios desligado")
	}
	for _, operation := range []string{"usercontrol_user.updated", "usercontrol_right.updated", "usercontrol_right_extra.updated"} {
		if err := requireMessageIntegration(group, operation, models.JSON(`{"id":"uc-1"}`)); err == nil {
			t.Fatalf("operacao %s deveria respeitar integrarUsuarios desligado", operation)
		}
	}

	group.IntegrarUsuarios = true
	for _, entity := range []string{"USERCONTROL_USUARIO", "USERCONTROL_DIREITO", "USERCONTROL_DIREITO_EXTRA"} {
		if err := requireEntityIntegration(group, entity); err != nil {
			t.Fatalf("entidade %s deveria ser liberada: %v", entity, err)
		}
	}
	if err := requireEntityIntegration(group, "users"); err != nil {
		t.Fatalf("o alias da publicacao direta deveria ser liberado: %v", err)
	}
	for _, operation := range []string{"usercontrol_user.updated", "usercontrol_right.updated", "usercontrol_right_extra.updated"} {
		if err := requireMessageIntegration(group, operation, models.JSON(`{"id":"uc-1"}`)); err != nil {
			t.Fatalf("operacao %s deveria ser liberada: %v", operation, err)
		}
	}

	if err := requireMessageIntegration(group, "employee.updated", models.JSON(`{"id":"employee-1"}`)); err != nil {
		t.Fatalf("a flag de funcionarios nao deveria depender de integrarUsuarios: %v", err)
	}
}
