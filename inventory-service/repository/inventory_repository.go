package repository

import (
	"context"
	"errors"
	"fmt"

	"ecommerce-order-management/inventory-service/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInventoryNotFound     = errors.New("inventory not found")
	ErrQuantityBelowReserved = errors.New("quantity cannot be below reserved quantity")
	ErrInsufficientStock     = errors.New("insufficient available stock")
)

type InventoryRepository struct {
	DB *pgxpool.Pool
}

func NewInventoryRepository(db *pgxpool.Pool) *InventoryRepository {
	return &InventoryRepository{DB: db}
}

func (r *InventoryRepository) Create(ctx context.Context, inventory *models.Inventory) error {
	query := `
		INSERT INTO inventory (product_id, quantity, reserved_quantity)
		VALUES ($1, $2, $3)
		RETURNING id, updated_at
	`

	err := r.DB.QueryRow(
		ctx, query,
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

func (r *InventoryRepository) GetByProductID(ctx context.Context, productID int64) (*models.Inventory, error) {
	var inventory models.Inventory

	query := `
		SELECT id, product_id, quantity, reserved_quantity, updated_at
		FROM inventory
		WHERE product_id = $1
	`

	err := r.DB.QueryRow(ctx, query, productID).Scan(
		&inventory.ID,
		&inventory.ProductID,
		&inventory.Quantity,
		&inventory.ReservedQuantity,
		&inventory.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrInventoryNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get inventory: %w", err)
	}

	inventory.AvailableStock = inventory.Quantity - inventory.ReservedQuantity
	return &inventory, nil
}

func (r *InventoryRepository) UpdateQuantity(ctx context.Context, productID int64, quantity int) (*models.Inventory, error) {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin inventory update: %w", err)
	}
	defer tx.Rollback(ctx)

	var inventory models.Inventory

	err = tx.QueryRow(ctx, `
		SELECT id, product_id, quantity, reserved_quantity, updated_at
		FROM inventory
		WHERE product_id = $1
		FOR UPDATE
	`, productID).Scan(
		&inventory.ID,
		&inventory.ProductID,
		&inventory.Quantity,
		&inventory.ReservedQuantity,
		&inventory.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrInventoryNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read inventory for update: %w", err)
	}

	if quantity < inventory.ReservedQuantity {
		return nil, ErrQuantityBelowReserved
	}

	err = tx.QueryRow(ctx, `
		UPDATE inventory
		SET quantity = $1, updated_at = NOW()
		WHERE product_id = $2
		RETURNING id, product_id, quantity, reserved_quantity, updated_at
	`, quantity, productID).Scan(
		&inventory.ID,
		&inventory.ProductID,
		&inventory.Quantity,
		&inventory.ReservedQuantity,
		&inventory.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("update inventory quantity: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit inventory update: %w", err)
	}

	inventory.AvailableStock = inventory.Quantity - inventory.ReservedQuantity
	return &inventory, nil
}

func (r *InventoryRepository) Reserve(ctx context.Context, productID int64, quantity int) (*models.Inventory, error) {
	var inventory models.Inventory

	query := `
		UPDATE inventory
		SET reserved_quantity = reserved_quantity + $1,
		    updated_at = NOW()
		WHERE product_id = $2
		  AND quantity - reserved_quantity >= $1
		RETURNING id, product_id, quantity, reserved_quantity, updated_at
	`

	err := r.DB.QueryRow(ctx, query, quantity, productID).Scan(
		&inventory.ID,
		&inventory.ProductID,
		&inventory.Quantity,
		&inventory.ReservedQuantity,
		&inventory.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		// Distinguish a missing product from insufficient stock.
		err = r.DB.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM inventory WHERE product_id = $1
			)
		`, productID).Scan(new(bool))

		if err != nil {
			return nil, fmt.Errorf("check inventory existence: %w", err)
		}

		var exists bool
		err = r.DB.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM inventory WHERE product_id = $1
			)
		`, productID).Scan(&exists)
		if err != nil {
			return nil, fmt.Errorf("check inventory existence: %w", err)
		}
		if !exists {
			return nil, ErrInventoryNotFound
		}
		return nil, ErrInsufficientStock
	}

	if err != nil {
		return nil, fmt.Errorf("reserve inventory: %w", err)
	}

	inventory.AvailableStock = inventory.Quantity - inventory.ReservedQuantity
	return &inventory, nil
}

func (r *InventoryRepository) Delete(ctx context.Context, productID int64) error {
	result, err := r.DB.Exec(ctx, `
		DELETE FROM inventory
		WHERE product_id = $1
		  AND reserved_quantity = 0
	`, productID)

	if err != nil {
		return fmt.Errorf("delete inventory: %w", err)
	}

	if result.RowsAffected() == 0 {
		var exists bool
		err := r.DB.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM inventory WHERE product_id = $1
			)
		`, productID).Scan(&exists)
		if err != nil {
			return fmt.Errorf("check inventory existence: %w", err)
		}
		if !exists {
			return ErrInventoryNotFound
		}
		return ErrQuantityBelowReserved
	}

	return nil
}

func (r *InventoryRepository) Release(ctx context.Context, productID int64, quantity int) (*models.Inventory, error) {
	var inventory models.Inventory

	query := `
		UPDATE inventory
		SET reserved_quantity = reserved_quantity - $1,
		    updated_at = NOW()
		WHERE product_id = $2
		  AND reserved_quantity >= $1
		RETURNING id, product_id, quantity, reserved_quantity, updated_at
	`

	err := r.DB.QueryRow(ctx, query, quantity, productID).Scan(
		&inventory.ID,
		&inventory.ProductID,
		&inventory.Quantity,
		&inventory.ReservedQuantity,
		&inventory.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		var exists bool
		err := r.DB.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM inventory WHERE product_id = $1
			)
		`, productID).Scan(&exists)

		if err != nil {
			return nil, fmt.Errorf("check inventory existence: %w", err)
		}
		if !exists {
			return nil, ErrInventoryNotFound
		}
		return nil, errors.New("cannot release more stock than is reserved")
	}

	if err != nil {
		return nil, fmt.Errorf("release inventory: %w", err)
	}

	inventory.AvailableStock = inventory.Quantity - inventory.ReservedQuantity
	return &inventory, nil
}

func (r *InventoryRepository) Commit(ctx context.Context, productID int64, quantity int) (*models.Inventory, error) {
	var inventory models.Inventory

	query := `
		UPDATE inventory
		SET quantity = quantity - $1,
		    reserved_quantity = reserved_quantity - $1,
		    updated_at = NOW()
		WHERE product_id = $2
		  AND reserved_quantity >= $1
		RETURNING id, product_id, quantity, reserved_quantity, updated_at
	`

	err := r.DB.QueryRow(ctx, query, quantity, productID).Scan(
		&inventory.ID,
		&inventory.ProductID,
		&inventory.Quantity,
		&inventory.ReservedQuantity,
		&inventory.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		var exists bool

		err := r.DB.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM inventory WHERE product_id = $1
			)
		`, productID).Scan(&exists)

		if err != nil {
			return nil, fmt.Errorf("check inventory existence: %w", err)
		}
		if !exists {
			return nil, ErrInventoryNotFound
		}

		return nil, errors.New("cannot commit more stock than is reserved")
	}

	if err != nil {
		return nil, fmt.Errorf("commit inventory: %w", err)
	}

	inventory.AvailableStock = inventory.Quantity - inventory.ReservedQuantity
	return &inventory, nil
}
