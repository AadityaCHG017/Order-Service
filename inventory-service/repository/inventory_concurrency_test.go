package repository

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ecommerce-order-management/inventory-service/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestConcurrentReservations(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
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
		_, _ = pool.Exec(context.Background(),
			"DELETE FROM inventory WHERE product_id = $1", productID)
	}()

	inventory := &models.Inventory{
		ProductID: productID,
		Quantity:  10,
	}
	if err := repo.Create(ctx, inventory); err != nil {
		t.Fatalf("create inventory: %v", err)
	}

	const workers = 10
	var successful atomic.Int32
	var insufficient atomic.Int32
	var unexpected atomic.Int32

	start := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()
			<-start

			_, err := repo.Reserve(ctx, productID, 2)
			switch {
			case err == nil:
				successful.Add(1)
			case errors.Is(err, ErrInsufficientStock):
				insufficient.Add(1)
			default:
				unexpected.Add(1)
				t.Errorf("unexpected reservation error: %v", err)
			}
		}()
	}

	close(start)
	wg.Wait()

	got, err := repo.GetByProductID(ctx, productID)
	if err != nil {
		t.Fatalf("get final inventory: %v", err)
	}

	if successful.Load() != 5 {
		t.Errorf("successful reservations = %d; want 5", successful.Load())
	}
	if insufficient.Load() != 5 {
		t.Errorf("insufficient-stock responses = %d; want 5", insufficient.Load())
	}
	if unexpected.Load() != 0 {
		t.Errorf("unexpected errors = %d; want 0", unexpected.Load())
	}
	if got.ReservedQuantity != 10 || got.AvailableStock != 0 {
		t.Errorf("final stock: reserved=%d available=%d; want 10 and 0",
			got.ReservedQuantity, got.AvailableStock)
	}

	fmt.Printf(
		"Results: successful=%d, insufficient=%d, reserved=%d, available=%d\n",
		successful.Load(), insufficient.Load(),
		got.ReservedQuantity, got.AvailableStock,
	)
}
