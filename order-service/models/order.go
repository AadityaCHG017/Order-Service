package models

import "time"

type Order struct {
	ID         int64     `json:"id"`
	CustomerID int64     `json:"customer_id"`
	Status     string    `json:"status"`
	Amount     float64   `json:"amount"`
	CreatedAt  time.Time `json:"created_at"`
}
type CreateOrderRequest struct {
	CustomerID int64             `json:"customer_id"`
	Items      []CreateOrderItem `json:"items"`
}

type CreateOrderItem struct {
	ProductID int64 `json:"product_id"`
	Quantity  int   `json:"quantity"`
}
