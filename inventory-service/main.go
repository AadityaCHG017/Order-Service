package main

import (
	"context"
	"ecommerce-order-management/inventory-service/handlers"
	"ecommerce-order-management/inventory-service/repository"
	"encoding/json"
	"log"
	"net/http"
)

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := map[string]string{
		"status":  "ok",
		"service": "inventory-service",
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode health response: %v", err)
	}
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
func main() {
	ctx := context.Background()

	pool, err := newDBPool(ctx)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Connected to PostgreSQL")
	defer pool.Close()

	mux := http.NewServeMux()
	inventoryRepo := repository.NewInventoryRepository(pool)
	inventoryHandler := handlers.NewInventoryHandler(inventoryRepo)

	mux.HandleFunc("POST /inventory", inventoryHandler.Create)
	mux.HandleFunc("GET /inventory/{product_id}", inventoryHandler.GetByProductID)

	handler := loggingMiddleware(mux)
	mux.HandleFunc("GET /health", healthHandler)

	log.Println("Inventory service listening on port:8082")
	if err := http.ListenAndServe(":8082", handler); err != nil {
		log.Fatal(err)
	}
}
