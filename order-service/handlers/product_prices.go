package handlers

import "fmt"

// Temporary price lookup for development.
// Later, Product Service will provide these prices through gRPC.
func getProductPrice(productID int64) (float64, error) {
	prices := map[int64]float64{
		1: 1000.00,
		2: 500.00,
		3: 250.00,
	}

	price, ok := prices[productID]
	if !ok {
		return 0, fmt.Errorf("product %d not found", productID)
	}

	return price, nil
}
