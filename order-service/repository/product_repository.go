package repository

import (
	"context"
	"ecommerce-order-management/order-service/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ProductRepository struct {
	DB *pgxpool.Pool
}

func NewProductRepository(db *pgxpool.Pool) *ProductRepository {
	return &ProductRepository{
		DB: db,
	}
}

func (r *ProductRepository) Create(
	ctx context.Context,
	product *models.Product,
) error {
	query := `
	INSET INTO Products(name, description, price, stock)
	VALUES($1,$2,$3,$4)
	RETURNING id, created_at
	`

	err := r.DB.QueryRow(
		ctx,
		query,
		product.Name,
		product.Description,
		product.Price,
		product.Stock,
	).Scan(
		&product.ID,
		&product.CreatedAt,
	)
	if err != nil {
		return err
	}
	return nil
}
