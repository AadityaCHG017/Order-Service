CREATE TABLE order_items(
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    order_id BIGINT NOT NULL,
    product_id	BIGINT NOT NULL,
    quantity INT NOT NULL
     CHECK (quantity > 0),
    unit_price	NUMERIC(12,2) NOT NULL
     CHECK (unit_price >= 0),
    created_at	TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (order_id)
    REFERENCES orders(id)
    ON DELETE CASCADE
)