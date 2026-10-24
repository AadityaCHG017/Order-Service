package handlers

import (
	"context"
	"ecommerce-order-management/order-service/models"
	"ecommerce-order-management/order-service/repository"
	"encoding/json"
	"log"
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
		log.Printf("Create product failed: %v", err)
		http.Error(w, "Failed to create Product", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if err := json.NewEncoder(w).Encode(product); err != nil {
		return
	}

}
func (h *ProductHandler) GetAllProducts(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	products, err := h.Repo.GetAll(ctx)
	if err != nil {
		http.Error(w, "Failed to fetch products", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if err := json.NewEncoder(w).Encode(products); err != nil {
		return
	}
}
