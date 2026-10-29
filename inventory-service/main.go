package main

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"time"

	inventoryv1 "ecommerce-order-management/contracts/gen/inventory/v1"
	"ecommerce-order-management/inventory-service/grpcserver"
	"ecommerce-order-management/inventory-service/handlers"
	"ecommerce-order-management/inventory-service/repository"

	"google.golang.org/grpc"
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

	// REST server
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("POST /inventory", inventoryHandler.Create)
	mux.HandleFunc("GET /inventory/{product_id}", inventoryHandler.GetByProductID)
	mux.HandleFunc("PATCH /inventory/{product_id}", inventoryHandler.UpdateQuantity)
	mux.HandleFunc("DELETE /inventory/{product_id}", inventoryHandler.Delete)
	mux.HandleFunc("POST /inventory/{product_id}/reserve", inventoryHandler.Reserve)
	mux.HandleFunc("POST /inventory/{product_id}/release", inventoryHandler.Release)
	mux.HandleFunc("POST /inventory/{product_id}/commit", inventoryHandler.Commit)

	httpServer := &http.Server{
		Addr:              ":8082",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// gRPC server
	listener, err := net.Listen("tcp", ":9092")
	if err != nil {
		log.Fatalf("Failed to listen for gRPC: %v", err)
	}

	grpcServer := grpc.NewServer()

	inventoryv1.RegisterInventoryServiceServer(
		grpcServer,
		grpcserver.NewInventoryServer(inventoryRepo),
	)

	go func() {
		log.Println("Inventory gRPC server listening on port 9092")
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatalf("gRPC server failed: %v", err)
		}
	}()

	log.Println("Inventory REST server listening on port 8082")

	if err := httpServer.ListenAndServe(); err != nil &&
		err != http.ErrServerClosed {
		log.Fatalf("REST server failed: %v", err)
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
