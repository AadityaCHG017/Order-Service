package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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
	getErr         error
	updateErr      error
	updatedItems   []models.OrderItem
	completed      bool
	restored       bool
	acquireLockErr error
	releaseLockErr error
	lockAcquired   bool
	lockReleased   bool
}

func (f *fakeOrderRepository) CreateWithItems(context.Context, *models.Order) error {
	return nil
}

func (f *fakeOrderRepository) GetAll(context.Context) ([]models.Order, error) {
	return nil, nil
}

func (f *fakeOrderRepository) GetByID(context.Context, int64) (*models.Order, error) {
	return f.order, f.getErr
}
func (f *fakeOrderRepository) UpdateItems(ctx context.Context, orderID int64, lockID string, items []models.OrderItem) (*models.Order, error) {
	if f.updateErr != nil {
		return nil, f.updateErr
	}

	f.updatedItems = items
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
type inventoryCall struct {
	productID int64
	quantity  int32
}

type fakeInventoryClient struct {
	reserveCalls int
	releaseCalls int
	reserveErrs  []error
	releaseErrs  []error
	reserved     []inventoryCall
	released     []inventoryCall
}

func (f *fakeInventoryClient) ReserveStock(_ context.Context, productID int64, quantity int32) (*inventoryv1.ReserveStockResponse, error) {
	index := f.reserveCalls
	f.reserveCalls++

	f.reserved = append(f.reserved, inventoryCall{
		productID: productID,
		quantity:  quantity,
	})

	if index < len(f.reserveErrs) && f.reserveErrs[index] != nil {
		return nil, f.reserveErrs[index]
	}

	return &inventoryv1.ReserveStockResponse{}, nil
}

func (f *fakeInventoryClient) ReleaseStock(_ context.Context, productID int64, quantity int32) (*inventoryv1.ReleaseStockResponse, error) {
	index := f.releaseCalls
	f.releaseCalls++

	f.released = append(f.released, inventoryCall{
		productID: productID,
		quantity:  quantity,
	})

	if index < len(f.releaseErrs) && f.releaseErrs[index] != nil {
		return nil, f.releaseErrs[index]
	}

	return &inventoryv1.ReleaseStockResponse{}, nil
}

func newCancellationTestHandler(repo *fakeOrderRepository, inventory *fakeInventoryClient) *OrderHandler {
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
func (f *fakeOrderRepository) GetItemsByOrderID(context.Context, int64) ([]models.OrderItem, error) {
	return nil, nil
}

func newUpdateRequest(body string) *http.Request {
	req := httptest.NewRequest(
		http.MethodPut,
		"/orders/1",
		strings.NewReader(body),
	)
	req.SetPathValue("id", "1")
	req.Header.Set("Content-Type", "application/json")
	return req
}

func TestUpdateIncreaseQuantity(t *testing.T) {
	repo := &fakeOrderRepository{
		order: &models.Order{
			ID:     1,
			Status: "pending",
			Items: []models.OrderItem{
				{ProductID: 1, Quantity: 2},
			},
		},
	}

	inventory := &fakeInventoryClient{}
	handler := NewOrderHandler(repo, inventory)

	recorder := httptest.NewRecorder()

	handler.Update(
		recorder,
		newUpdateRequest(`{"items":[{"product_id":1,"quantity":5}]}`),
	)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	if inventory.reserveCalls != 1 {
		t.Fatalf("expected 1 reservation, got %d", inventory.reserveCalls)
	}

	call := inventory.reserved[0]

	if call.productID != 1 || call.quantity != 3 {
		t.Fatalf("expected reserve product 1 quantity 3, got %+v", call)
	}

	if inventory.releaseCalls != 0 {
		t.Fatal("unexpected inventory release")
	}
}

func TestUpdateDecreaseQuantity(t *testing.T) {
	repo := &fakeOrderRepository{
		order: &models.Order{
			ID:     1,
			Status: "pending",
			Items: []models.OrderItem{
				{ProductID: 1, Quantity: 5},
			},
		},
	}

	inventory := &fakeInventoryClient{}
	handler := NewOrderHandler(repo, inventory)

	recorder := httptest.NewRecorder()

	handler.Update(
		recorder,
		newUpdateRequest(`{"items":[{"product_id":1,"quantity":2}]}`),
	)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	if inventory.releaseCalls != 1 {
		t.Fatalf("expected 1 release, got %d", inventory.releaseCalls)
	}

	call := inventory.released[0]

	if call.productID != 1 || call.quantity != 3 {
		t.Fatalf("expected release product 1 quantity 3, got %+v", call)
	}

	if inventory.reserveCalls != 0 {
		t.Fatal("unexpected inventory reservation")
	}
}

func TestUpdateRemoveProduct(t *testing.T) {
	repo := &fakeOrderRepository{
		order: &models.Order{
			ID:     1,
			Status: "pending",
			Items: []models.OrderItem{
				{ProductID: 1, Quantity: 2},
				{ProductID: 2, Quantity: 3},
			},
		},
	}

	inventory := &fakeInventoryClient{}
	handler := NewOrderHandler(repo, inventory)

	recorder := httptest.NewRecorder()

	handler.Update(
		recorder,
		newUpdateRequest(`{"items":[{"product_id":1,"quantity":2}]}`),
	)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	if inventory.releaseCalls != 1 {
		t.Fatalf("expected 1 release, got %d", inventory.releaseCalls)
	}

	call := inventory.released[0]

	if call.productID != 2 || call.quantity != 3 {
		t.Fatalf("expected release product 2 quantity 3, got %+v", call)
	}

	if inventory.reserveCalls != 0 {
		t.Fatal("unexpected inventory reservation")
	}
}

func TestUpdateReservationFailure(t *testing.T) {
	order := &models.Order{
		ID:     1,
		Status: "pending",
		Items: []models.OrderItem{
			{ProductID: 1, Quantity: 2},
		},
	}

	repo := &fakeOrderRepository{order: order}

	inventory := &fakeInventoryClient{
		reserveErrs: []error{errors.New("insufficient stock")},
	}

	handler := &OrderHandler{
		Repo:            repo,
		InventoryClient: inventory,
	}

	req := newUpdateRequest(`{"items":[{"product_id":1,"quantity":5}]}`)
	recorder := httptest.NewRecorder()

	handler.Update(recorder, req)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("expected status 502, got %d", recorder.Code)
	}

	if inventory.reserveCalls != 1 {
		t.Fatalf("expected 1 reservation attempt, got %d", inventory.reserveCalls)
	}

	if len(repo.updatedItems) != 0 {
		t.Fatal("order should not be updated when reservation fails")
	}
}
func TestUpdateDatabaseFailure(t *testing.T) {
	order := &models.Order{
		ID:     1,
		Status: "pending",
		Items: []models.OrderItem{
			{ProductID: 1, Quantity: 2},
		},
	}

	repo := &fakeOrderRepository{
		order:     order,
		updateErr: errors.New("database update failed"),
	}

	inventory := &fakeInventoryClient{}

	handler := &OrderHandler{
		Repo:            repo,
		InventoryClient: inventory,
	}

	req := newUpdateRequest(`{"items":[{"product_id":1,"quantity":5}]}`)
	recorder := httptest.NewRecorder()

	handler.Update(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", recorder.Code)
	}

	if inventory.reserveCalls != 1 {
		t.Fatalf("expected 1 reservation call, got %d", inventory.reserveCalls)
	}

	if inventory.releaseCalls != 1 {
		t.Fatalf("expected 1 rollback release call, got %d", inventory.releaseCalls)
	}

	if inventory.released[0].productID != 1 || inventory.released[0].quantity != 3 {
		t.Fatalf("expected rollback of 3 units of product 1, got %+v", inventory.released[0])
	}
}
func TestUpdateRollbackFailure(t *testing.T) {
	order := &models.Order{
		ID:     1,
		Status: "pending",
		Items: []models.OrderItem{
			{ProductID: 1, Quantity: 2},
		},
	}

	repo := &fakeOrderRepository{
		order:     order,
		updateErr: errors.New("database update failed"),
	}

	inventory := &fakeInventoryClient{
		releaseErrs: []error{
			errors.New("inventory release failed"),
		},
	}

	handler := &OrderHandler{
		Repo:            repo,
		InventoryClient: inventory,
	}

	req := newUpdateRequest(`{"items":[{"product_id":1,"quantity":5}]}`)
	recorder := httptest.NewRecorder()

	handler.Update(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", recorder.Code)
	}

	if inventory.reserveCalls != 1 {
		t.Fatalf("expected 1 reservation call, got %d", inventory.reserveCalls)
	}

	if inventory.releaseCalls != 1 {
		t.Fatalf("expected 1 rollback attempt, got %d", inventory.releaseCalls)
	}

	if len(repo.updatedItems) != 0 {
		t.Fatal("order should not be updated")
	}
}
func (f *fakeOrderRepository) AcquireUpdateLock(ctx context.Context, orderID int64) (string, error) {
	if f.acquireLockErr != nil {
		return "", f.acquireLockErr
	}

	f.lockAcquired = true
	return "test-lock-id", nil
}

func (f *fakeOrderRepository) ReleaseUpdateLock(ctx context.Context, orderID int64, lockID string) error {
	if f.releaseLockErr != nil {
		return f.releaseLockErr
	}

	f.lockReleased = true
	return nil
}
func TestUpdateLockAcquiredAndReleased(t *testing.T) {
	order := &models.Order{
		ID:     1,
		Status: "pending",
		Items: []models.OrderItem{
			{ProductID: 1, Quantity: 2},
		},
	}

	repo := &fakeOrderRepository{order: order}
	inventory := &fakeInventoryClient{}

	handler := &OrderHandler{
		Repo:            repo,
		InventoryClient: inventory,
	}

	req := newUpdateRequest(`{"items":[{"product_id":1,"quantity":2}]}`)
	recorder := httptest.NewRecorder()

	handler.Update(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	if !repo.lockAcquired {
		t.Fatal("expected update lock to be acquired")
	}

	if !repo.lockReleased {
		t.Fatal("expected update lock to be released")
	}
}

func TestUpdateLockAcquisitionFailure(t *testing.T) {
	repo := &fakeOrderRepository{
		acquireLockErr: errors.New("order already being updated"),
	}

	handler := &OrderHandler{
		Repo:            repo,
		InventoryClient: &fakeInventoryClient{},
	}

	req := newUpdateRequest(`{"items":[{"product_id":1,"quantity":2}]}`)
	recorder := httptest.NewRecorder()

	handler.Update(recorder, req)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d", recorder.Code)
	}

	if repo.lockReleased {
		t.Fatal("lock should not be released when acquisition failed")
	}
}
