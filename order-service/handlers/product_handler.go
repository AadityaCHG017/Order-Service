package handlers

import (
	"context"
	"ecommerce-order-management/order-service/models"
	"ecommerce-order-management/order-service/repository"
	"encoding/json"
	"net/http"
	"time"
)

type ProductHandler struct {
	Repo *repository.ProductRepository
}

func NewProducthandler(repo *repository.ProductRepository) *ProductHandler {
	return &ProductHandler{
		Repo: repo,
	}
}
func (h *ProductHandler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	// Parse the request body to get product details
	var product models.Product

	if err := json.NewDecoder(r.Body).Decode(&product); err != nil {
		http.Error(w, "Invalid Request Body", http.StatusBadRequest)
		return
	}

	if product.Name == "" || product.Price < 0 || product.Stock < 0 {
		http.Error(w, "Invalid Product Data", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := h.Repo.Create(ctx, &product); err != nil {
		http.Error(w, "Failed to create Product", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if err := json.NewEncoder(w).Encode(product); err != nil {
		return
	}

}
