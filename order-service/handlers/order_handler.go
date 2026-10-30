package handlers

import (
	"context"
	inventoryv1 "ecommerce-order-management/contracts/gen/inventory/v1"
	"ecommerce-order-management/order-service/models"
	"ecommerce-order-management/order-service/repository"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type OrderRepository interface {
	CreateWithItems(ctx context.Context, order *models.Order) error
	GetAll(ctx context.Context) ([]models.Order, error)
	GetByID(ctx context.Context, id int64) (*models.Order, error)
	UpdateItems(ctx context.Context, orderID int64, items []models.OrderItem) (*models.Order, error)

	StartCancellation(ctx context.Context, orderID int64) (*models.Order, string, error)
	CompleteCancellation(ctx context.Context, orderID int64) error
	RestoreCancellation(ctx context.Context, orderID int64, originalStatus string) error
}

type InventoryClient interface {
	ReserveStock(ctx context.Context, productID int64, quantity int32) (*inventoryv1.ReserveStockResponse, error)
	ReleaseStock(ctx context.Context, productID int64, quantity int32) (*inventoryv1.ReleaseStockResponse, error)
}

type OrderHandler struct {
	Repo            OrderRepository
	InventoryClient InventoryClient
}

func NewOrderHandler(
	repo OrderRepository,
	inventoryClient InventoryClient,
) *OrderHandler {
	return &OrderHandler{
		Repo:            repo,
		InventoryClient: inventoryClient,
	}
}

// Releases stock reserved before an order failed.
func (h *OrderHandler) rollbackReservations(
	ctx context.Context,
	reserved []models.CreateOrderItemRequest,
) {
	for i := len(reserved) - 1; i >= 0; i-- {
		item := reserved[i]

		cleanupCtx, cancel := context.WithTimeout(
			context.WithoutCancel(ctx),
			3*time.Second,
		)

		_, err := h.InventoryClient.ReleaseStock(
			cleanupCtx,
			item.ProductID,
			int32(item.Quantity),
		)
		cancel()

		if err != nil {
			log.Printf(
				"failed to release stock for product %d: %v",
				item.ProductID,
				err,
			)
		}
	}
}

func (h *OrderHandler) Create(w http.ResponseWriter, r *http.Request) {
	var order models.CreateOrderRequest

	if err := json.NewDecoder(r.Body).Decode(&order); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if order.CustomerID <= 0 || len(order.Items) == 0 {
		http.Error(w, "Invalid order data", http.StatusBadRequest)
		return
	}

	newOrder := models.Order{
		CustomerID: order.CustomerID,
		Status:     "pending",
		Items:      []models.OrderItem{},
	}

	// Validate all items and prices before reserving stock.
	for _, item := range order.Items {
		if item.ProductID <= 0 || item.Quantity <= 0 {
			http.Error(
				w,
				"Invalid product ID or quantity",
				http.StatusBadRequest,
			)
			return
		}

		// The gRPC contract uses int32 for quantity.
		if int64(item.Quantity) > int64(1<<31-1) {
			http.Error(w, "Quantity is too large", http.StatusBadRequest)
			return
		}

		price, err := getProductPrice(item.ProductID)
		if err != nil {
			http.Error(w, "Product not found", http.StatusBadRequest)
			return
		}

		newOrder.Items = append(newOrder.Items, models.OrderItem{
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
			UnitPrice: price,
		})
	}

	// Reserve stock for each item through Inventory Service.
	reserved := make([]models.CreateOrderItemRequest, 0, len(order.Items))

	for _, item := range order.Items {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)

		_, err := h.InventoryClient.ReserveStock(
			ctx,
			item.ProductID,
			int32(item.Quantity),
		)
		cancel()

		if err != nil {
			h.rollbackReservations(r.Context(), reserved)

			switch status.Code(err) {
			case codes.FailedPrecondition:
				http.Error(w, "Insufficient stock", http.StatusConflict)
			case codes.NotFound:
				http.Error(w, "Inventory not found", http.StatusNotFound)
			case codes.InvalidArgument:
				http.Error(w, "Invalid stock reservation request", http.StatusBadRequest)
			default:
				http.Error(
					w,
					"Inventory service unavailable",
					http.StatusBadGateway,
				)
			}
			return
		}

		reserved = append(reserved, item)
	}

	// Persist the order only after all reservations succeed.
	if err := h.Repo.CreateWithItems(r.Context(), &newOrder); err != nil {
		h.rollbackReservations(r.Context(), reserved)
		http.Error(
			w,
			"Failed to create order",
			http.StatusInternalServerError,
		)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if err := json.NewEncoder(w).Encode(newOrder); err != nil {
		log.Printf("failed to encode created order: %v", err)
	}
}

func (h *OrderHandler) GetAllOrders(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	orders, err := h.Repo.GetAll(ctx)
	if err != nil {
		http.Error(w, "Failed to fetch orders", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(orders); err != nil {
		log.Printf("failed to encode orders: %v", err)
	}
}

func (h *OrderHandler) GetOrderByID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "Invalid order ID", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	order, err := h.Repo.GetByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		http.Error(w, "Order not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Failed to fetch order", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(order); err != nil {
		log.Printf("failed to encode order: %v", err)
	}
}

func (h *OrderHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "Invalid order ID", http.StatusBadRequest)
		return
	}

	var req models.UpdateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(
			w,
			"Invalid request body: "+err.Error(),
			http.StatusBadRequest,
		)
		return
	}

	if len(req.Items) == 0 {
		http.Error(
			w,
			"Order must contain at least one item",
			http.StatusBadRequest,
		)
		return
	}

	items := make([]models.OrderItem, 0, len(req.Items))

	for _, item := range req.Items {
		if item.ProductID <= 0 || item.Quantity <= 0 {
			http.Error(w, "Invalid product ID or quantity", http.StatusBadRequest)
			return
		}

		price, err := getProductPrice(item.ProductID)
		if err != nil {
			http.Error(w, "Product not found", http.StatusBadRequest)
			return
		}

		items = append(items, models.OrderItem{
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
			UnitPrice: price,
		})
	}

	updatedOrder, err := h.Repo.UpdateItems(r.Context(), id, items)
	if err != nil {
		http.Error(w, "Failed to update order", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(updatedOrder); err != nil {
		log.Printf("failed to encode updated order: %v", err)
	}
}

func (h *OrderHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "Invalid order ID", http.StatusBadRequest)
		return
	}

	order, originalStatus, err := h.Repo.StartCancellation(r.Context(), id)

	if errors.Is(err, repository.ErrOrderNotFound) {
		http.Error(w, "Order not found", http.StatusNotFound)
		return
	}
	if errors.Is(err, repository.ErrOrderCannotBeCancelled) {
		http.Error(w, "Order cannot be cancelled in its current status", http.StatusConflict)
		return
	}
	if err != nil {
		http.Error(w, "Failed to start cancellation", http.StatusInternalServerError)
		return
	}

	released := make([]models.OrderItem, 0, len(order.Items))

	for _, item := range order.Items {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		_, err := h.InventoryClient.ReleaseStock(
			ctx,
			item.ProductID,
			int32(item.Quantity),
		)
		cancel()

		if err != nil {
			h.restoreReleasedStock(r.Context(), released)

			cleanupCtx, cleanupCancel := context.WithTimeout(
				context.WithoutCancel(r.Context()), 3*time.Second,
			)
			restoreErr := h.Repo.RestoreCancellation(
				cleanupCtx, id, originalStatus,
			)
			cleanupCancel()

			if restoreErr != nil {
				log.Printf("failed to restore order %d status: %v", id, restoreErr)
			}

			http.Error(w, "Failed to release inventory; cancellation aborted",
				http.StatusBadGateway)
			return
		}

		released = append(released, item)
	}

	if err := h.Repo.CompleteCancellation(r.Context(), id); err != nil {
		h.restoreReleasedStock(r.Context(), released)

		cleanupCtx, cleanupCancel := context.WithTimeout(
			context.WithoutCancel(r.Context()), 3*time.Second,
		)
		restoreErr := h.Repo.RestoreCancellation(
			cleanupCtx, id, originalStatus,
		)
		cleanupCancel()

		if restoreErr != nil {
			log.Printf("failed to restore order %d status: %v", id, restoreErr)
		}

		http.Error(w, "Failed to complete cancellation",
			http.StatusInternalServerError)
		return
	}

	order.Status = "cancelled"
	order.UpdatedAt = time.Now()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(order); err != nil {
		log.Printf("failed to encode cancelled order: %v", err)
	}
}

func (h *OrderHandler) restoreReleasedStock(ctx context.Context, items []models.OrderItem) {
	for i := len(items) - 1; i >= 0; i-- {
		item := items[i]

		cleanupCtx, cancel := context.WithTimeout(
			context.WithoutCancel(ctx), 3*time.Second,
		)

		_, err := h.InventoryClient.ReserveStock(
			cleanupCtx,
			item.ProductID,
			int32(item.Quantity),
		)
		cancel()

		if err != nil {
			log.Printf(
				"failed to restore stock for product %d: %v",
				item.ProductID, err,
			)
		}
	}
}
