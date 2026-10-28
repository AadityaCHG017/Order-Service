CREATE TABLE inventory(
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    product_id BIGINT NOT NULL UNIQUE,
    quantity INT NOT NULL DEFAULT 0 CHECK(quantity >= 0),
    reserved_quantity INT NOT NULL DEFAULT 0 CHECK(reserved_quantity >= 0),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    CHECK (reserved_quantity <= quantity)
);