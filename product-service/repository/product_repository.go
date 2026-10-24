package repository

import (
	"context"
	"ecommerce-order-management/product-service/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProductRepository struct {
	DB *pgxpool.Pool
}

func NewProductRepository(db *pgxpool.Pool) *ProductRepository {
	return &ProductRepository{
		DB: db,
	}
}

func (r *ProductRepository) Create(
	ctx context.Context,
	product *models.Product,
) error {
	query := `
	INSERT INTO Products(name, description, price, stock)
	VALUES($1,$2,$3,$4)
	RETURNING id, created_at
	`

	err := r.DB.QueryRow(
		ctx,
		query,
		product.Name,
		product.Description,
		product.Price,
		product.Stock,
	).Scan(
		&product.ID,
		&product.CreatedAt,
	)
	if err != nil {
		return err
	}
	return nil
}

func (r *ProductRepository) GetAll(ctx context.Context) ([]models.Product, error) {
	query := `SELECT id,name,description,price,stock,created_at FROM Products ORDER BY id`

	rows, err := r.DB.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []models.Product
	for rows.Next() {
		var product models.Product
		if err := rows.Scan(
			&product.ID,
			&product.Name,
			&product.Description,
			&product.Price,
			&product.Stock,
			&product.CreatedAt,
		); err != nil {
			return nil, err
		}
		products = append(products, product)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return products, nil
}

func (r *ProductRepository) GetByID(ctx context.Context, id int64) (*models.Product, error) {

	query := `SELECT id,name,description,price,stock,created_at FROM Products WHERE id=$1`

	var product models.Product

	err := r.DB.QueryRow(ctx, query, id).Scan(
		&product.ID,
		&product.Name,
		&product.Description,
		&product.Price,
		&product.Stock,
		&product.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &product, nil

}

func (r *ProductRepository) Update(
	ctx context.Context,
	product *models.Product,
) error {
	query := `
		UPDATE products
		SET name = $1,
		    description = $2,
		    price = $3,
		    stock = $4
		WHERE id = $5
		RETURNING id, name, description, price, stock, created_at
	`

	err := r.DB.QueryRow(
		ctx,
		query,
		product.Name,
		product.Description,
		product.Price,
		product.Stock,
		product.ID,
	).Scan(
		&product.ID,
		&product.Name,
		&product.Description,
		&product.Price,
		&product.Stock,
		&product.CreatedAt,
	)
	if err != nil {
		return err
	}

	return nil
}

func (r *ProductRepository) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM products WHERE id = $1`

	tag, err := r.DB.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}
