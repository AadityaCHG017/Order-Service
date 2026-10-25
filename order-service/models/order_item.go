package models

import "time"

type OrderItem struct {
	ID        int64     `json:"id"`
	OrderID   int64     `json:"order_id"`
	ProductID int64     `json:"product_id"`
	Quantity  int       `json:"quantity"`
	UnitPrice float64   `json:"unit_price"`
	CreatedAt time.Time `json:"created_at"`
}
