package services

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/phsoftwares/storelink-backend/internal/models"
)

func requireGroupIntegration(group models.Group, capability string) error {
	var enabled bool
	var label string
	switch capability {
	case "prices":
		enabled, label = group.IntegrarPrecos, "preços"
	case "products":
		enabled, label = group.IntegrarProdutos, "produtos, grupos e subgrupos"
	case "categories":
		enabled, label = group.IntegrarProdutos || group.IntegrarGruposSubgrupos, "grupos e subgrupos"
	case "customers":
		enabled, label = group.IntegrarClientes, "clientes"
	case "employees":
		enabled, label = group.IntegrarFuncionarios, "funcion\u00e1rios e operadores de caixa"
	case "users":
		enabled, label = group.IntegrarUsuarios, "usu\u00e1rios e permiss\u00f5es"
	default:
		return nil
	}
	if !enabled {
		return models.Error(409, fmt.Sprintf("Integração de %s desativada neste grupo", label))
	}
	return nil
}

func requireEntityIntegration(group models.Group, entity string) error {
	switch strings.ToLower(strings.TrimSpace(entity)) {
	case "price", "product_price", "prices":
		return requireGroupIntegration(group, "prices")
	case "product":
		return requireGroupIntegration(group, "products")
	case "product_group", "product_subgroup":
		return requireGroupIntegration(group, "categories")
	case "customer":
		return requireGroupIntegration(group, "customers")
	case "agreement":
		return requireGroupIntegration(group, "customers")
	case "employee", "cashier_operator":
		return requireGroupIntegration(group, "employees")
	case "usercontrol_usuario", "usercontrol_user", "usercontrol_direito",
		"usercontrol_direito_extra", "usercontrol_right", "usercontrol_right_extra":
		return requireGroupIntegration(group, "users")
	case "users":
		return requireGroupIntegration(group, "users")
	default:
		return nil
	}
}

func requireMessageIntegration(group models.Group, operation string, payload models.JSON) error {
	switch {
	case operation == "product.update_price", operation == "price.updated":
		return requireGroupIntegration(group, "prices")
	case operation == "sync.products":
		return requireGroupIntegration(group, "products")
	case operation == "sync.customers":
		return requireGroupIntegration(group, "customers")
	case operation == "sync.employees":
		return requireGroupIntegration(group, "employees")
	case operation == "sync.batch":
		var batch struct {
			Entity string `json:"entity"`
		}
		if err := json.Unmarshal(payload, &batch); err != nil {
			return models.Error(400, "Lote de sincronização inválido")
		}
		return requireEntityIntegration(group, batch.Entity)
	case strings.HasPrefix(operation, "product_group."), strings.HasPrefix(operation, "product_subgroup."):
		return requireGroupIntegration(group, "categories")
	case strings.HasPrefix(operation, "product."):
		return requireGroupIntegration(group, "products")
	case strings.HasPrefix(operation, "customer."):
		return requireGroupIntegration(group, "customers")
	case strings.HasPrefix(operation, "agreement."):
		return requireGroupIntegration(group, "customers")
	case strings.HasPrefix(operation, "employee."), strings.HasPrefix(operation, "cashier_operator."):
		return requireGroupIntegration(group, "employees")
	case strings.HasPrefix(operation, "usercontrol_user."),
		strings.HasPrefix(operation, "usercontrol_right."),
		strings.HasPrefix(operation, "usercontrol_right_extra."):
		return requireGroupIntegration(group, "users")
	default:
		return nil
	}
}
