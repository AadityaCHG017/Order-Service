package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"ecommerce-order-management/inventory-service/models"
	"ecommerce-order-management/inventory-service/repository"

	"github.com/jackc/pgx/v5"
)

type InventoryHandler struct {
	Repo *repository.InventoryRepository
}

func NewInventoryHandler(repo *repository.InventoryRepository) *InventoryHandler {
	return &InventoryHandler{Repo: repo}
}

func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

func (h *InventoryHandler) Create(w http.ResponseWriter, r *http.Request) {
	var inventory models.Inventory

	if err := json.NewDecoder(r.Body).Decode(&inventory); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	if inventory.ProductID <= 0 {
		respondError(w, http.StatusBadRequest, "product_id must be greater than 0")
		return
	}

	if inventory.Quantity < 0 {
		respondError(w, http.StatusBadRequest, "quantity cannot be negative")
		return
	}

	if inventory.ReservedQuantity != 0 {
		respondError(w, http.StatusBadRequest, "reserved_quantity must be 0 when creating inventory")
		return
	}

	err := h.Repo.Create(r.Context(), &inventory)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create inventory")
		return
	}

	respondJSON(w, http.StatusCreated, inventory)
}

func (h *InventoryHandler) GetByProductID(w http.ResponseWriter, r *http.Request) {
	productID, err := strconv.ParseInt(r.PathValue("product_id"), 10, 64)
	if err != nil || productID <= 0 {
		respondError(w, http.StatusBadRequest, "Invalid product ID")
		return
	}

	inventory, err := h.Repo.GetByProductID(r.Context(), productID)
	if errors.Is(err, repository.ErrInventoryNotFound) || errors.Is(err, pgx.ErrNoRows) {
		respondError(w, http.StatusNotFound, "Inventory not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to retrieve inventory")
		return
	}

	respondJSON(w, http.StatusOK, inventory)
}

func (h *InventoryHandler) UpdateQuantity(w http.ResponseWriter, r *http.Request) {
	productID, err := strconv.ParseInt(r.PathValue("product_id"), 10, 64)
	if err != nil || productID <= 0 {
		respondError(w, http.StatusBadRequest, "Invalid product ID")
		return
	}

	var request models.UpdateInventoryRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	if request.Quantity < 0 {
		respondError(w, http.StatusBadRequest, "quantity cannot be negative")
		return
	}

	inventory, err := h.Repo.UpdateQuantity(r.Context(), productID, request.Quantity)
	if errors.Is(err, repository.ErrInventoryNotFound) || errors.Is(err, pgx.ErrNoRows) {
		respondError(w, http.StatusNotFound, "Inventory not found")
		return
	}
	if errors.Is(err, repository.ErrQuantityBelowReserved) {
		respondError(w, http.StatusConflict, "Quantity cannot be below reserved quantity")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update inventory")
		return
	}

	respondJSON(w, http.StatusOK, inventory)
}

func (h *InventoryHandler) Delete(w http.ResponseWriter, r *http.Request) {
	productID, err := strconv.ParseInt(r.PathValue("product_id"), 10, 64)
	if err != nil || productID <= 0 {
		respondError(w, http.StatusBadRequest, "Invalid product ID")
		return
	}

	err = h.Repo.Delete(r.Context(), productID)
	if errors.Is(err, repository.ErrInventoryNotFound) {
		respondError(w, http.StatusNotFound, "Inventory not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete inventory")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Inventory deleted successfully",
	})
}

func (h *InventoryHandler) Reserve(w http.ResponseWriter, r *http.Request) {
	productID, err := strconv.ParseInt(r.PathValue("product_id"), 10, 64)
	if err != nil || productID <= 0 {
		respondError(w, http.StatusBadRequest, "Invalid product ID")
		return
	}

	var request models.ReserveInventoryRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	if request.Quantity <= 0 {
		respondError(w, http.StatusBadRequest, "quantity must be greater than 0")
		return
	}

	inventory, err := h.Repo.Reserve(r.Context(), productID, request.Quantity)

	if errors.Is(err, repository.ErrInventoryNotFound) {
		respondError(w, http.StatusNotFound, "Inventory not found")
		return
	}

	if errors.Is(err, repository.ErrInsufficientStock) {
		respondError(w, http.StatusConflict, "Insufficient available stock")
		return
	}

	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to reserve inventory")
		return
	}

	respondJSON(w, http.StatusOK, inventory)
}

func (h *InventoryHandler) Release(w http.ResponseWriter, r *http.Request) {
	productID, err := strconv.ParseInt(r.PathValue("product_id"), 10, 64)
	if err != nil || productID <= 0 {
		respondError(w, http.StatusBadRequest, "Invalid product ID")
		return
	}

	var request models.ReserveInventoryRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	if request.Quantity <= 0 {
		respondError(w, http.StatusBadRequest, "quantity must be greater than 0")
		return
	}

	inventory, err := h.Repo.Release(r.Context(), productID, request.Quantity)

	if errors.Is(err, repository.ErrInventoryNotFound) {
		respondError(w, http.StatusNotFound, "Inventory not found")
		return
	}

	if err != nil {
		respondError(w, http.StatusConflict, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, inventory)
}

func (h *InventoryHandler) Commit(w http.ResponseWriter, r *http.Request) {
	productID, err := strconv.ParseInt(r.PathValue("product_id"), 10, 64)
	if err != nil || productID <= 0 {
		respondError(w, http.StatusBadRequest, "Invalid product ID")
		return
	}

	var request models.ReserveInventoryRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	if request.Quantity <= 0 {
		respondError(w, http.StatusBadRequest, "quantity must be greater than 0")
		return
	}

	inventory, err := h.Repo.Commit(r.Context(), productID, request.Quantity)

	if errors.Is(err, repository.ErrInventoryNotFound) {
		respondError(w, http.StatusNotFound, "Inventory not found")
		return
	}

	if err != nil {
		respondError(w, http.StatusConflict, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, inventory)
}
