package repository

import (
	"context"
	"fmt"

	"ecommerce-order-management/inventory-service/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type InventoryRepository struct {
	DB *pgxpool.Pool
}

func NewInventoryRepository(db *pgxpool.Pool) *InventoryRepository {
	return &InventoryRepository{
		DB: db,
	}
}

func (r *InventoryRepository) Create(ctx context.Context, inventory *models.Inventory) error {
	query := `
		INSERT INTO inventory (product_id, quantity, reserved_quantity)
		VALUES ($1, $2, $3)
		RETURNING id, updated_at
	`

	err := r.DB.QueryRow(
		ctx,
		query,
		inventory.ProductID,
		inventory.Quantity,
		inventory.ReservedQuantity,
	).Scan(&inventory.ID, &inventory.UpdatedAt)

	if err != nil {
		return fmt.Errorf("create inventory: %w", err)
	}

	inventory.AvailableStock = inventory.Quantity - inventory.ReservedQuantity

	return nil
}
