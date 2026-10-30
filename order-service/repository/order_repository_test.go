package repository

import (
	"context"
	"ecommerce-order-management/order-service/models"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func setupTestRepository(t *testing.T) *OrderRepository {
	t.Helper()

	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}

	if err := db.Ping(ctx); err != nil {
		db.Close()
		t.Fatalf("ping test database: %v", err)
	}

	t.Cleanup(db.Close)

	return NewOrderRepository(db)
}

func createTestOrder(t *testing.T, repo *OrderRepository) int64 {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var orderID int64

	err := repo.DB.QueryRow(ctx, `
		INSERT INTO orders (customer_id, status, total_amount)
		VALUES (999999, 'pending', 1000)
		RETURNING id
	`).Scan(&orderID)

	if err != nil {
		t.Fatalf("create test order: %v", err)
	}

	t.Cleanup(func() {
		_, err := repo.DB.Exec(
			context.Background(),
			`DELETE FROM orders WHERE id = $1`,
			orderID,
		)
		if err != nil {
			t.Errorf("clean up test order: %v", err)
		}
	})

	return orderID
}

func TestAcquireUpdateLock(t *testing.T) {
	repo := setupTestRepository(t)

	orderID := createTestOrder(t, repo)
	ctx := context.Background()

	// First request should acquire the lock.
	firstLock, err := repo.AcquireUpdateLock(ctx, orderID)
	if err != nil {
		t.Fatalf("first lock should succeed: %v", err)
	}

	if firstLock == "" {
		t.Fatal("expected a lock ID")
	}

	// Second request should not acquire an active lock.
	_, err = repo.AcquireUpdateLock(ctx, orderID)
	if err == nil {
		t.Fatal("second lock should fail while first lock is active")
	}

	// Original owner should be able to release the lock.
	err = repo.ReleaseUpdateLock(ctx, orderID, firstLock)
	if err != nil {
		t.Fatalf("release lock: %v", err)
	}
}
func TestAcquireExpiredUpdateLock(t *testing.T) {
	repo := setupTestRepository(t)

	orderID := createTestOrder(t, repo)
	ctx := context.Background()

	// First request acquires a lock.
	oldLock, err := repo.AcquireUpdateLock(ctx, orderID)
	if err != nil {
		t.Fatalf("acquire first lock: %v", err)
	}

	// Simulate expiration directly in PostgreSQL.
	_, err = repo.DB.Exec(ctx, `
		UPDATE orders
		SET update_lock_expires_at = NOW() - INTERVAL '1 second'
		WHERE id = $1
	`, orderID)

	if err != nil {
		t.Fatalf("expire lock: %v", err)
	}

	// A new request should acquire the expired lock.
	newLock, err := repo.AcquireUpdateLock(ctx, orderID)
	if err != nil {
		t.Fatalf("new request should acquire expired lock: %v", err)
	}

	if oldLock == newLock {
		t.Fatal("new request should receive a different lock ID")
	}

	// Old owner must not release the new owner's lock.
	err = repo.ReleaseUpdateLock(ctx, orderID, oldLock)
	if err != nil {
		t.Fatalf("old owner release attempt: %v", err)
	}

	// New owner must still be able to release it.
	err = repo.ReleaseUpdateLock(ctx, orderID, newLock)
	if err != nil {
		t.Fatalf("new owner release: %v", err)
	}
}
func TestUpdateItemsRejectsExpiredLockOwner(t *testing.T) {
	repo := setupTestRepository(t)

	orderID := createTestOrder(t, repo)
	ctx := context.Background()

	// Request A acquires the first lock.
	oldLock, err := repo.AcquireUpdateLock(ctx, orderID)
	if err != nil {
		t.Fatalf("acquire first lock: %v", err)
	}

	// Simulate expiration of Request A's lock.
	_, err = repo.DB.Exec(ctx, `
		UPDATE orders
		SET update_lock_expires_at = NOW() - INTERVAL '1 second'
		WHERE id = $1
	`, orderID)
	if err != nil {
		t.Fatalf("expire first lock: %v", err)
	}

	// Request B acquires a new lock.
	newLock, err := repo.AcquireUpdateLock(ctx, orderID)
	if err != nil {
		t.Fatalf("acquire replacement lock: %v", err)
	}

	// Request A tries to update using its expired lock.
	items := []models.OrderItem{
		{
			ProductID: 1,
			Quantity:  2,
			UnitPrice: 100,
		},
	}

	_, err = repo.UpdateItems(ctx, orderID, oldLock, items)
	if err == nil {
		t.Fatal("expected expired lock owner to be rejected")
	}

	// Request B should still be able to update.
	_, err = repo.UpdateItems(ctx, orderID, newLock, items)
	if err != nil {
		t.Fatalf("new lock owner should be allowed to update: %v", err)
	}

	// Release Request B's lock.
	if err := repo.ReleaseUpdateLock(ctx, orderID, newLock); err != nil {
		t.Fatalf("release new lock: %v", err)
	}
}
