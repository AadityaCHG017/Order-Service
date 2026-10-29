package main

import (
	"context"
	"log"
	"time"

	inventoryv1 "ecommerce-order-management/contracts/gen/inventory/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	conn, err := grpc.NewClient(
		"localhost:9092",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := inventoryv1.NewInventoryServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response, err := client.ReserveStock(ctx, &inventoryv1.ReserveStockRequest{
		ProductId: 101,
		Quantity:  2,
	})
	if err != nil {
		log.Fatalf("Reservation failed: %v", err)
	}

	log.Printf(
		"Reservation successful: product=%d reserved=%d available=%d",
		response.GetProductId(),
		response.GetQuantityReserved(),
		response.GetAvailableStock(),
	)
}
