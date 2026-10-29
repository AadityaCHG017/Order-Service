package grpcserver

import (
	"context"

	inventoryv1 "ecommerce-order-management/contracts/gen/inventory/v1"
	"ecommerce-order-management/inventory-service/repository"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type InventoryServer struct {
	inventoryv1.UnimplementedInventoryServiceServer
	repo *repository.InventoryRepository
}

func NewInventoryServer(repo *repository.InventoryRepository) *InventoryServer {
	return &InventoryServer{
		repo: repo,
	}
}

func (s *InventoryServer) ReserveStock(
	ctx context.Context,
	req *inventoryv1.ReserveStockRequest,
) (*inventoryv1.ReserveStockResponse, error) {

	if req == nil || req.GetProductId() <= 0 || req.GetQuantity() <= 0 {
		return nil, status.Error(
			codes.InvalidArgument,
			"product_id and quantity must be positive",
		)
	}

	inventory, err := s.repo.Reserve(
		ctx,
		req.GetProductId(),
		int(req.GetQuantity()),
	)

	if err != nil {
		switch {
		case err == repository.ErrInventoryNotFound:
			return nil, status.Error(codes.NotFound, "inventory not found")

		case err == repository.ErrInsufficientStock:
			return nil, status.Error(codes.FailedPrecondition, "insufficient stock")

		default:
			return nil, status.Error(codes.Internal, "failed to reserve stock")
		}
	}

	return &inventoryv1.ReserveStockResponse{
		ProductId:        inventory.ProductID,
		QuantityReserved: req.GetQuantity(),
		AvailableStock:   int32(inventory.AvailableStock),
	}, nil
}
