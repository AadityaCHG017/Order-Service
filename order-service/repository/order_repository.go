package repository

import (
	"context"
	"ecommerce-order-management/order-service/models"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrOrderNotFound          = errors.New("order not found")
	ErrOrderCannotBeCancelled = errors.New("order cannot be cancelled")
)

type OrderRepository struct {
	DB *pgxpool.Pool
}

func NewOrderRepository(db *pgxpool.Pool) *OrderRepository {
	return &OrderRepository{
		DB: db,
	}
}

func (r *OrderRepository) CreateWithItems(ctx context.Context, order *models.Order) error {
	if len(order.Items) == 0 {
		return fmt.Errorf("order must contain at least one item")
	}
	var total float64

	for _, item := range order.Items {
		if item.ProductID <= 0 || item.Quantity <= 0 || item.UnitPrice < 0 {
			return fmt.Errorf("invalid order item")
		}

		total += float64(item.Quantity) * item.UnitPrice
	}

	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `
        INSERT INTO orders (customer_id, status, total_amount)
        VALUES ($1, $2, $3)
        RETURNING id, created_at, updated_at
    `

	err = tx.QueryRow(
		ctx,
		query,
		order.CustomerID,
		order.Status,
		total,
	).Scan(
		&order.ID,
		&order.CreatedAt,
		&order.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert order: %w", err)
	}

	order.TotalAmount = total

	// Insert each order item.
	itemQuery := `
        INSERT INTO order_items (order_id, product_id, quantity, unit_price)
        VALUES ($1, $2, $3, $4)
        RETURNING id, created_at
    `

	for i := range order.Items {
		item := &order.Items[i]

		err = tx.QueryRow(
			ctx,
			itemQuery,
			order.ID,
			item.ProductID,
			item.Quantity,
			item.UnitPrice,
		).Scan(
			&item.ID,
			&item.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("insert order item: %w", err)
		}

		item.OrderID = order.ID
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit order transaction: %w", err)
	}

	return nil
}

func (r *OrderRepository) GetAll(ctx context.Context) ([]models.Order, error) {
	query := `SELECT id,customer_id,status,total_amount,created_at,updated_at FROM orders ORDER BY id`

	rows, err := r.DB.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []models.Order
	for rows.Next() {
		var order models.Order
		order.Items = []models.OrderItem{}
		if err := rows.Scan(
			&order.ID,
			&order.CustomerID,
			&order.Status,
			&order.TotalAmount,
			&order.CreatedAt,
			&order.UpdatedAt,
		); err != nil {
			return nil, err
		}

		itemQuery := `SELECT id, order_id, product_id, quantity, unit_price, created_at FROM order_items WHERE order_id = $1 ORDER BY id `

		itemRows, err := r.DB.Query(ctx, itemQuery, order.ID)
		if err != nil {
			return nil, err
		}

		for itemRows.Next() {
			var item models.OrderItem

			if err := itemRows.Scan(
				&item.ID,
				&item.OrderID,
				&item.ProductID,
				&item.Quantity,
				&item.UnitPrice,
				&item.CreatedAt,
			); err != nil {
				itemRows.Close()
				return nil, err
			}

			order.Items = append(order.Items, item)
		}

		if err := itemRows.Err(); err != nil {
			itemRows.Close()
			return nil, itemRows.Err()
		}

		itemRows.Close()
		orders = append(orders, order)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return orders, nil
}
func (r *OrderRepository) GetByID(ctx context.Context, id int64) (*models.Order, error) {
	query := `SELECT id,customer_id,status,total_amount,created_at,updated_at FROM orders WHERE id=$1`

	var order models.Order

	err := r.DB.QueryRow(ctx, query, id).Scan(
		&order.ID,
		&order.CustomerID,
		&order.Status,
		&order.TotalAmount,
		&order.CreatedAt,
		&order.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	itemQuery := `SELECT id, order_id, product_id, quantity, unit_price, created_at FROM order_items WHERE order_id = $1 ORDER BY id`

	itemRows, err := r.DB.Query(ctx, itemQuery, order.ID)
	if err != nil {
		return nil, err
	}
	defer itemRows.Close()

	for itemRows.Next() {
		var item models.OrderItem

		if err := itemRows.Scan(
			&item.ID,
			&item.OrderID,
			&item.ProductID,
			&item.Quantity,
			&item.UnitPrice,
			&item.CreatedAt,
		); err != nil {
			return nil, err
		}

		order.Items = append(order.Items, item)
	}

	if err := itemRows.Err(); err != nil {
		return nil, err
	}

	return &order, nil
}
func (r *OrderRepository) UpdateItems(ctx context.Context, orderID int64, items []models.OrderItem) (*models.Order, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("order must contain at least one item")
	}

	var total float64
	for _, item := range items {
		if item.ProductID <= 0 || item.Quantity <= 0 || item.UnitPrice < 0 {
			return nil, fmt.Errorf("invalid order item")
		}
		total += float64(item.Quantity) * item.UnitPrice
	}

	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var order models.Order

	err = tx.QueryRow(ctx, `
        UPDATE orders
        SET total_amount = $1, updated_at = NOW()
        WHERE id = $2 AND status = 'pending'
        RETURNING id, customer_id, status, total_amount, created_at, updated_at
    `, total, orderID).Scan(
		&order.ID,
		&order.CustomerID,
		&order.Status,
		&order.TotalAmount,
		&order.CreatedAt,
		&order.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(ctx, `DELETE FROM order_items WHERE order_id = $1`, orderID)
	if err != nil {
		return nil, fmt.Errorf("delete old order items: %w", err)
	}

	order.Items = make([]models.OrderItem, 0, len(items))

	for _, item := range items {
		var savedItem models.OrderItem

		err = tx.QueryRow(ctx, `
            INSERT INTO order_items (order_id, product_id, quantity, unit_price)
            VALUES ($1, $2, $3, $4)
            RETURNING id, order_id, product_id, quantity, unit_price, created_at
        `, orderID, item.ProductID, item.Quantity, item.UnitPrice).Scan(
			&savedItem.ID,
			&savedItem.OrderID,
			&savedItem.ProductID,
			&savedItem.Quantity,
			&savedItem.UnitPrice,
			&savedItem.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("insert updated order item: %w", err)
		}

		order.Items = append(order.Items, savedItem)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit order update: %w", err)
	}

	return &order, nil
}
func (r *OrderRepository) Cancel(ctx context.Context, orderID int64) (*models.Order, error) {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin cancellation transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var order models.Order

	err = tx.QueryRow(ctx, `
        SELECT id, customer_id, status, total_amount, created_at, updated_at
        FROM orders
        WHERE id = $1
        FOR UPDATE
    `, orderID).Scan(
		&order.ID,
		&order.CustomerID,
		&order.Status,
		&order.TotalAmount,
		&order.CreatedAt,
		&order.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrOrderNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find order: %w", err)
	}

	if order.Status != "pending" && order.Status != "confirmed" {
		return nil, ErrOrderCannotBeCancelled
	}

	err = tx.QueryRow(ctx, `
        UPDATE orders
        SET status = 'cancelled', updated_at = NOW()
        WHERE id = $1
        RETURNING id, customer_id, status, total_amount, created_at, updated_at
    `, orderID).Scan(
		&order.ID,
		&order.CustomerID,
		&order.Status,
		&order.TotalAmount,
		&order.CreatedAt,
		&order.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("cancel order: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit cancellation: %w", err)
	}

	return &order, nil
}

func (r *OrderRepository) StartCancellation(ctx context.Context, orderID int64) (*models.Order, string, error) {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("begin cancellation: %w", err)
	}
	defer tx.Rollback(ctx)

	var order models.Order
	var originalStatus string

	err = tx.QueryRow(ctx, `
        SELECT id, customer_id, status, total_amount,
               created_at, updated_at
        FROM orders
        WHERE id = $1
        FOR UPDATE
    `, orderID).Scan(
		&order.ID,
		&order.CustomerID,
		&originalStatus,
		&order.TotalAmount,
		&order.CreatedAt,
		&order.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", ErrOrderNotFound
	}
	if err != nil {
		return nil, "", fmt.Errorf("find order: %w", err)
	}

	if originalStatus != "pending" && originalStatus != "confirmed" {
		return nil, "", ErrOrderCannotBeCancelled
	}

	_, err = tx.Exec(ctx, `
        UPDATE orders
        SET status = 'cancelling', updated_at = NOW()
        WHERE id = $1
    `, orderID)
	if err != nil {
		return nil, "", fmt.Errorf("start cancellation: %w", err)
	}

	itemRows, err := tx.Query(ctx, `
        SELECT id, order_id, product_id, quantity,
               unit_price, created_at
        FROM order_items
        WHERE order_id = $1
        ORDER BY id
    `, orderID)
	if err != nil {
		return nil, "", fmt.Errorf("load order items: %w", err)
	}

	for itemRows.Next() {
		var item models.OrderItem
		if err := itemRows.Scan(
			&item.ID,
			&item.OrderID,
			&item.ProductID,
			&item.Quantity,
			&item.UnitPrice,
			&item.CreatedAt,
		); err != nil {
			itemRows.Close()
			return nil, "", fmt.Errorf("scan order item: %w", err)
		}
		order.Items = append(order.Items, item)
	}

	rowsErr := itemRows.Err()
	itemRows.Close()
	if rowsErr != nil {
		return nil, "", fmt.Errorf("read order items: %w", rowsErr)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, "", fmt.Errorf("commit cancellation start: %w", err)
	}

	order.Status = "cancelling"
	return &order, originalStatus, nil
}

func (r *OrderRepository) CompleteCancellation(ctx context.Context, orderID int64) error {
	result, err := r.DB.Exec(ctx, `
        UPDATE orders
        SET status = 'cancelled', updated_at = NOW()
        WHERE id = $1 AND status = 'cancelling'
    `, orderID)
	if err != nil {
		return fmt.Errorf("complete cancellation: %w", err)
	}
	if result.RowsAffected() != 1 {
		return fmt.Errorf("order is not in cancelling status")
	}
	return nil
}

func (r *OrderRepository) RestoreCancellation(ctx context.Context, orderID int64, originalStatus string) error {
	if originalStatus != "pending" && originalStatus != "confirmed" {
		return fmt.Errorf("invalid original order status")
	}

	result, err := r.DB.Exec(ctx, `
        UPDATE orders
        SET status = $1, updated_at = NOW()
        WHERE id = $2 AND status = 'cancelling'
    `, originalStatus, orderID)
	if err != nil {
		return fmt.Errorf("restore order status: %w", err)
	}
	if result.RowsAffected() != 1 {
		return fmt.Errorf("order is not in cancelling status")
	}
	return nil
}
