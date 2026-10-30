package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	inventoryv1 "ecommerce-order-management/contracts/gen/inventory/v1"
	"ecommerce-order-management/order-service/models"
	"ecommerce-order-management/order-service/repository"
)

// Fake repository for testing without PostgreSQL.
type fakeOrderRepository struct {
	order          *models.Order
	originalStatus string
	startErr       error
	completeErr    error
	restoreErr     error
	completed      bool
	restored       bool
}

func (f *fakeOrderRepository) CreateWithItems(context.Context, *models.Order) error {
	return nil
}

func (f *fakeOrderRepository) GetAll(context.Context) ([]models.Order, error) {
	return nil, nil
}

func (f *fakeOrderRepository) GetByID(context.Context, int64) (*models.Order, error) {
	return f.order, nil
}

func (f *fakeOrderRepository) UpdateItems(context.Context, int64, []models.OrderItem) (*models.Order, error) {
	return f.order, nil
}

func (f *fakeOrderRepository) StartCancellation(context.Context, int64) (*models.Order, string, error) {
	if f.startErr != nil {
		return nil, "", f.startErr
	}
	return f.order, f.originalStatus, nil
}

func (f *fakeOrderRepository) CompleteCancellation(context.Context, int64) error {
	if f.completeErr != nil {
		return f.completeErr
	}
	f.completed = true
	return nil
}

func (f *fakeOrderRepository) RestoreCancellation(_ context.Context, _ int64, _ string) error {
	f.restored = true
	return f.restoreErr
}

// Fake inventory client for testing gRPC outcomes.
type fakeInventoryClient struct {
	releaseErrs  []error
	releaseCalls int
	reserveCalls int
}

func (f *fakeInventoryClient) ReserveStock(
	context.Context, int64, int32,
) (*inventoryv1.ReserveStockResponse, error) {
	f.reserveCalls++
	return &inventoryv1.ReserveStockResponse{}, nil
}

func (f *fakeInventoryClient) ReleaseStock(
	context.Context, int64, int32,
) (*inventoryv1.ReleaseStockResponse, error) {
	index := f.releaseCalls
	f.releaseCalls++

	if index < len(f.releaseErrs) && f.releaseErrs[index] != nil {
		return nil, f.releaseErrs[index]
	}

	return &inventoryv1.ReleaseStockResponse{}, nil
}

func newCancellationTestHandler(
	repo *fakeOrderRepository,
	inventory *fakeInventoryClient,
) *OrderHandler {
	return NewOrderHandler(repo, inventory)
}

func newCancelRequest() *http.Request {
	req := httptest.NewRequest(http.MethodPatch, "/orders/1/cancel", nil)
	req.SetPathValue("id", "1")
	return req
}

func TestCancelSuccess(t *testing.T) {
	repo := &fakeOrderRepository{
		order: &models.Order{
			ID:     1,
			Status: "cancelling",
			Items: []models.OrderItem{
				{ProductID: 1, Quantity: 2},
				{ProductID: 2, Quantity: 1},
			},
		},
		originalStatus: "pending",
	}

	inventory := &fakeInventoryClient{}
	handler := newCancellationTestHandler(repo, inventory)

	recorder := httptest.NewRecorder()
	handler.Cancel(recorder, newCancelRequest())

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	if !repo.completed {
		t.Fatal("expected order cancellation to be completed")
	}

	if inventory.releaseCalls != 2 {
		t.Fatalf("expected 2 inventory releases, got %d", inventory.releaseCalls)
	}
}

func TestCancelDuplicate(t *testing.T) {
	repo := &fakeOrderRepository{
		startErr: repository.ErrOrderCannotBeCancelled,
	}
	inventory := &fakeInventoryClient{}
	handler := newCancellationTestHandler(repo, inventory)

	recorder := httptest.NewRecorder()
	handler.Cancel(recorder, newCancelRequest())

	if recorder.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d", recorder.Code)
	}

	if inventory.releaseCalls != 0 {
		t.Fatal("inventory should not be released for duplicate cancellation")
	}
}

func TestCancelInventoryReleaseFailure(t *testing.T) {
	repo := &fakeOrderRepository{
		order: &models.Order{
			ID:     1,
			Status: "cancelling",
			Items: []models.OrderItem{
				{ProductID: 1, Quantity: 2},
				{ProductID: 2, Quantity: 1},
			},
		},
		originalStatus: "pending",
	}

	inventory := &fakeInventoryClient{
		releaseErrs: []error{
			nil,
			errors.New("inventory service unavailable"),
		},
	}

	handler := newCancellationTestHandler(repo, inventory)

	recorder := httptest.NewRecorder()
	handler.Cancel(recorder, newCancelRequest())

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("expected status 502, got %d", recorder.Code)
	}

	if !repo.restored {
		t.Fatal("expected order status to be restored")
	}

	if inventory.releaseCalls != 2 {
		t.Fatalf("expected 2 release calls, got %d", inventory.releaseCalls)
	}

	if inventory.reserveCalls != 1 {
		t.Fatalf("expected 1 stock restoration call, got %d", inventory.reserveCalls)
	}

	if repo.completed {
		t.Fatal("order cancellation should not be completed after release failure")
	}
}
