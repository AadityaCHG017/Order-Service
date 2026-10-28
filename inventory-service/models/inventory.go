package models

import "time"

type Inventory struct {
	ID               int64     `json:"id"`
	ProductID        int64     `json:"product_id"`
	Quantity         int       `json:"quantity"`
	ReservedQuantity int       `json:"reserved_quantity"`
	AvailableStock   int       `json:"available_stock"`
	UpdatedAt        time.Time `json:"updated_at"`
}
