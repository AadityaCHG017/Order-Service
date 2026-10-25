package models

import "time"

type Order struct {
	ID          int64       `json:"id"`
	CustomerID  int64       `json:"customer_id"`
	Status      string      `json:"status"`
	TotalAmount float64     `json:"total_amount"`
	Items       []OrderItem `json:"items"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}
