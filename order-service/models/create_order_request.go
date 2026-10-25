package models

type CreateOrderRequest struct {
	CustomerID int64                    `json:"customer_id"`
	Items      []CreateOrderItemRequest `json:"items"`
}

type CreateOrderItemRequest struct {
	ProductID int64 `json:"product_id"`
	Quantity  int   `json:"quantity"`
}
