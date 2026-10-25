package repository

import (
	"context"
	"ecommerce-order-management/order-service/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type OrderRepository struct {
	DB *pgxpool.Pool
}

func NewOrderRepository(db *pgxpool.Pool) *OrderRepository {
	return &OrderRepository{
		DB: db,
	}
}

func (r *OrderRepository) Create(ctx context.Context, order *models.Order) error {
	query := `
	INSERT INTO Products(customer_id, status, total_amount)
	VALUES($1,$2,$3)
	RETURNING id, created_at,updated_at
	`

	err := r.DB.QueryRow(
		ctx,
		query,
		order.CustomerID,
		order.Status,
		order.TotalAmount,
	).Scan(
		&order.ID,
		&order.CreatedAt,
	)
	if err != nil {
		return err
	}
	return nil
}
