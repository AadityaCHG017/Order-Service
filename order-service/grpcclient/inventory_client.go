package grpcclient

import (
	"context"

	inventoryv1 "ecommerce-order-management/contracts/gen/inventory/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type InventoryClient struct {
	client inventoryv1.InventoryServiceClient
	conn   *grpc.ClientConn
}

func NewInventoryClient(address string) (*InventoryClient, error) {
	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	return &InventoryClient{
		client: inventoryv1.NewInventoryServiceClient(conn),
		conn:   conn,
	}, nil
}

func (c *InventoryClient) ReserveStock(ctx context.Context, productID int64, quantity int32) (*inventoryv1.ReserveStockResponse, error) {
	return c.client.ReserveStock(ctx, &inventoryv1.ReserveStockRequest{
		ProductId: productID,
		Quantity:  quantity,
	})
}
func (c *InventoryClient) ReleaseStock(ctx context.Context, productID int64, quantity int32) (*inventoryv1.ReleaseStockResponse, error) {
	return c.client.ReleaseStock(ctx, &inventoryv1.ReleaseStockRequest{
		ProductId: productID,
		Quantity:  quantity,
	})
}
func (c *InventoryClient) Close() error {
	return c.conn.Close()
}
