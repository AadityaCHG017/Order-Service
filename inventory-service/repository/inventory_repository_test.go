package repository

import (
	"context"
	"ecommerce-order-management/inventory-service/models"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestInventoryStockLifecycle(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set; skipping PostgreSQL integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create database pool: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("connect to database: %v", err)
	}

	repo := NewInventoryRepository(pool)

	productID := time.Now().UnixNano()

	defer func() {
		_, _ = pool.Exec(ctx,
			"DELETE FROM inventory WHERE product_id = $1",
			productID,
		)
	}()

	inventory := &models.Inventory{
		ProductID: productID,
		Quantity:  10,
	}

	if err := repo.Create(ctx, inventory); err != nil {
		t.Fatalf("create inventory: %v", err)
	}
	got, err := repo.Reserve(ctx, productID, 5)
	if err != nil {
		t.Fatalf("reserve 5 units: %v", err)
	}
	if got.ReservedQuantity != 5 || got.AvailableStock != 5 {
		t.Fatalf("after reserve: got reserved=%d available=%d; want 5 and 5",
			got.ReservedQuantity, got.AvailableStock)
	}

	_, err = repo.Reserve(ctx, productID, 6)
	if !errors.Is(err, ErrInsufficientStock) {
		t.Fatalf("reserve excess stock: got error %v; want ErrInsufficientStock", err)
	}

	if _, err := repo.Release(ctx, productID, 6); err == nil {
		t.Fatal("expected releasing more than reserved to fail")
	}

	if _, err := repo.Commit(ctx, productID, 6); err == nil {
		t.Fatal("expected committing more than reserved to fail")
	}
	got, err = repo.GetByProductID(ctx, productID)
	if err != nil {
		t.Fatalf("get inventory after rejected operations: %v", err)
	}
	if got.Quantity != 10 || got.ReservedQuantity != 5 || got.AvailableStock != 5 {
		t.Fatalf("inventory changed unexpectedly: %+v", got)
	}

	got, err = repo.Release(ctx, productID, 2)
	if err != nil {
		t.Fatalf("release 2 units: %v", err)
	}
	if got.ReservedQuantity != 3 || got.AvailableStock != 7 {
		t.Fatalf("after release: got reserved=%d available=%d; want 3 and 7",
			got.ReservedQuantity, got.AvailableStock)
	}

	got, err = repo.Commit(ctx, productID, 3)
	if err != nil {
		t.Fatalf("commit 3 units: %v", err)
	}
	if got.Quantity != 7 || got.ReservedQuantity != 0 || got.AvailableStock != 7 {
		t.Fatalf("after commit: got quantity=%d reserved=%d available=%d; want 7, 0, 7",
			got.Quantity, got.ReservedQuantity, got.AvailableStock)
	}
}
