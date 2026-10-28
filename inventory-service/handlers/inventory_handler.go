package handlers

import (
	"encoding/json"
	"net/http"

	"ecommerce-order-management/inventory-service/models"
	"ecommerce-order-management/inventory-service/repository"
)

type InventoryHandler struct {
	Repo *repository.InventoryRepository
}

func NewInventoryHandler(repo *repository.InventoryRepository) *InventoryHandler {
	return &InventoryHandler{
		Repo: repo,
	}
}

func (h *InventoryHandler) Create(w http.ResponseWriter, r *http.Request) {
	var inventory models.Inventory

	if err := json.NewDecoder(r.Body).Decode(&inventory); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if inventory.ProductID <= 0 || inventory.Quantity < 0 {
		http.Error(w, "Invalid product ID or quantity", http.StatusBadRequest)
		return
	}

	if inventory.ReservedQuantity != 0 {
		http.Error(w, "Reserved quantity must be zero when creating inventory", http.StatusBadRequest)
		return
	}

	err := h.Repo.Create(r.Context(), &inventory)
	if err != nil {
		http.Error(w, "Failed to create inventory", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if err := json.NewEncoder(w).Encode(inventory); err != nil {
		return
	}
}
