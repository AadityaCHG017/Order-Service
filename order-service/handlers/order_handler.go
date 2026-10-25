package handlers

import (
	"context"
	"ecommerce-order-management/order-service/models"
	"ecommerce-order-management/order-service/repository"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
)

type OrderHandler struct {
	Repo *repository.OrderRepository
}

func NewOrderHandler(repo *repository.OrderRepository) *OrderHandler {
	return &OrderHandler{
		Repo: repo,
	}
}
func (h *OrderHandler) Create(w http.ResponseWriter, r *http.Request) {
	var order models.CreateOrderRequest

	if err := json.NewDecoder(r.Body).Decode(&order); err != nil {
		http.Error(w, "Invalid Request Body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if order.CustomerID <= 0 || len(order.Items) == 0 {
		http.Error(w, "Invalid Order Data", http.StatusBadRequest)
		return
	}
	for _, item := range order.Items {
		if item.ProductID <= 0 || item.Quantity <= 0 {
			http.Error(w, "Invalid product ID or quantity", http.StatusBadRequest)
			return
		}
	}
	newOrder := models.Order{
		CustomerID: order.CustomerID,
		Status:     "pending",
		Items:      []models.OrderItem{},
	}

	for _, item := range order.Items {
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

	if err := h.Repo.CreateWithItems(r.Context(), &newOrder); err != nil {
		http.Error(w, "Failed to create order", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if err := json.NewEncoder(w).Encode(newOrder); err != nil {
		return
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
		return
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
		return
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
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if len(req.Items) == 0 {
		http.Error(w, "Order must contain at least one item", http.StatusBadRequest)
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
		return
	}
}

func (h *OrderHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "Invalid order ID", http.StatusBadRequest)
		return
	}

	order, err := h.Repo.Cancel(r.Context(), id)

	if errors.Is(err, repository.ErrOrderNotFound) {
		http.Error(w, "Order not found", http.StatusNotFound)
		return
	}

	if errors.Is(err, repository.ErrOrderCannotBeCancelled) {
		http.Error(w, "Order cannot be cancelled in its current status", http.StatusConflict)
		return
	}

	if err != nil {
		http.Error(w, "Failed to cancel order", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(order); err != nil {
		return
	}
}
