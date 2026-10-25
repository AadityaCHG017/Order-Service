package models

type UpdateOrderRequest struct {
	Items []CreateOrderItemRequest `json:"items"`
}
