package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"ecommerce-order-management/inventory-service/handlers"
	"ecommerce-order-management/inventory-service/repository"
)

func main() {
	ctx := context.Background()

	pool, err := newDBPool(ctx)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	inventoryRepo := repository.NewInventoryRepository(pool)
	inventoryHandler := handlers.NewInventoryHandler(inventoryRepo)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("POST /inventory", inventoryHandler.Create)
	mux.HandleFunc("GET /inventory/{product_id}", inventoryHandler.GetByProductID)
	mux.HandleFunc("PATCH /inventory/{product_id}", inventoryHandler.UpdateQuantity)
	mux.HandleFunc("DELETE /inventory/{product_id}", inventoryHandler.Delete)
	mux.HandleFunc("POST /inventory/{product_id}/reserve", inventoryHandler.Reserve)
	mux.HandleFunc("POST /inventory/{product_id}/release", inventoryHandler.Release)
	mux.HandleFunc("POST /inventory/{product_id}/commit", inventoryHandler.Commit)

	server := &http.Server{
		Addr:              ":8082",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Println("Inventory service listening on port 8082")

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Inventory service failed: %v", err)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	_ = json.NewEncoder(w).Encode(map[string]string{
		"service": "inventory-service",
		"status":  "ok",
	})
}
