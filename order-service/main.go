package main

import (
	"context"
	"ecommerce-order-management/order-service/grpcclient"
	"ecommerce-order-management/order-service/handlers"
	"ecommerce-order-management/order-service/repository"
	"encoding/json"
	"log"
	"net/http"
)

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := map[string]string{
		"status":  "ok",
		"service": "order-service",
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("failed to encode health response: %v", err)
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
	orderRepo := repository.NewOrderRepository(pool)

	inventoryClient, err := grpcclient.NewInventoryClient("localhost:9092")
	if err != nil {
		log.Fatalf("Failed to create inventory client: %v", err)
	}
	defer inventoryClient.Close()

	orderHandler := handlers.NewOrderHandler(orderRepo, inventoryClient)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("POST /orders", orderHandler.Create)
	mux.HandleFunc("GET /orders", orderHandler.GetAllOrders)
	mux.HandleFunc("GET /orders/{id}", orderHandler.GetOrderByID)
	mux.HandleFunc("PUT /orders/{id}", orderHandler.Update)
	mux.HandleFunc("PATCH /orders/{id}/cancel", orderHandler.Cancel)

	handler := loggingMiddleware(mux)
	log.Println("Order service listening on port:8081")
	if err := http.ListenAndServe(":8081", handler); err != nil {
		log.Fatal(err)
	}
}
