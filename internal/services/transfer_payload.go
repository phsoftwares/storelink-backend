package services

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/phsoftwares/storelink-backend/internal/models"
)

type agentTransferPayload struct {
	ID              uuid.UUID           `json:"id"`
	Number          string              `json:"number"`
	OriginCNPJ      string              `json:"originCnpj,omitempty"`
	DestinationCNPJ string              `json:"destinationCnpj,omitempty"`
	SenderUsername  string              `json:"senderUsername,omitempty"`
	SenderNote      string              `json:"senderNote,omitempty"`
	SentAt          *time.Time          `json:"sentAt,omitempty"`
	Items           []agentTransferLine `json:"items"`
}

type agentTransferLine struct {
	ID                     uuid.UUID  `json:"id"`
	ProductID              *uuid.UUID `json:"productId,omitempty"`
	LocalProductCode       string     `json:"localProductCode,omitempty"`
	SKU                    string     `json:"sku,omitempty"`
	Barcode                string     `json:"barcode"`
	Name                   string     `json:"name"`
	Unit                   string     `json:"unit,omitempty"`
	Quantity               float64    `json:"quantity"`
	SalePrice              float64    `json:"salePrice"`
	CostPrice              float64    `json:"costPrice"`
	BatchCode              string     `json:"batchCode,omitempty"`
	LotSerial              string     `json:"lotSerial,omitempty"`
	Lot                    string     `json:"lot,omitempty"`
	Serial                 string     `json:"serial,omitempty"`
	ExpiresAt              *time.Time `json:"expiresAt,omitempty"`
	ManufacturedAt         *time.Time `json:"manufacturedAt,omitempty"`
	Controlled             *bool      `json:"controlled,omitempty"`
	TracksBatch            *bool      `json:"tracksBatch,omitempty"`
	TracksSerial           *bool      `json:"tracksSerial,omitempty"`
	RegistrationMS         string     `json:"registrationMS,omitempty"`
	TherapeuticClass       string     `json:"therapeuticClass,omitempty"`
	SNGPCType              string     `json:"sngpcType,omitempty"`
	SNGPCUnit              string     `json:"sngpcUnit,omitempty"`
	PricesInformed         *bool      `json:"pricesInformed,omitempty"`
	PriceTablePayload      string     `json:"priceTablePayload,omitempty"`
	UsesPriceTableInformed *bool      `json:"usesPriceTableInformed,omitempty"`
	UsesPriceTable         string     `json:"usesPriceTable,omitempty"`
}

func decodeTransferPayload(raw models.JSON) (models.TransferPayload, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		if err == nil {
			err = json.Unmarshal(raw, &fields)
		}
		return models.TransferPayload{}, err
	}
	if _, legacy := fields["number"]; !legacy {
		var payload models.TransferPayload
		if err := json.Unmarshal(raw, &payload); err != nil {
			return models.TransferPayload{}, err
		}
		return payload, nil
	}

	var value agentTransferPayload
	if err := json.Unmarshal(raw, &value); err != nil {
		return models.TransferPayload{}, err
	}
	payload := models.TransferPayload{
		ID: value.ID, Number: value.Number, OriginCNPJ: value.OriginCNPJ,
		DestinationCNPJ: value.DestinationCNPJ, SenderUsername: value.SenderUsername,
		SenderNote: value.SenderNote, SentAt: value.SentAt,
		Items: make([]models.TransferLine, 0, len(value.Items)),
	}
	for _, line := range value.Items {
		payload.Items = append(payload.Items, models.TransferLine{
			ID: line.ID, ProductID: line.ProductID, LocalProductCode: line.LocalProductCode,
			SKU: line.SKU, Barcode: line.Barcode, Name: line.Name, Unit: line.Unit,
			Quantity: line.Quantity, SalePrice: line.SalePrice, CostPrice: line.CostPrice,
			BatchCode: line.BatchCode, LotSerial: line.LotSerial, Lot: line.Lot,
			Serial: line.Serial, ExpiresAt: line.ExpiresAt, ManufacturedAt: line.ManufacturedAt,
			Controlled: line.Controlled, TracksBatch: line.TracksBatch, TracksSerial: line.TracksSerial,
			RegistrationMS: line.RegistrationMS, TherapeuticClass: line.TherapeuticClass,
			SNGPCType: line.SNGPCType, SNGPCUnit: line.SNGPCUnit,
			PricesInformed: line.PricesInformed, PriceTablePayload: line.PriceTablePayload,
			UsesPriceTableInformed: line.UsesPriceTableInformed, UsesPriceTable: line.UsesPriceTable,
		})
	}
	return payload, nil
}
