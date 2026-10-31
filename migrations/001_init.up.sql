CREATE TABLE products (
    id         BIGSERIAL PRIMARY KEY,
    name       TEXT        NOT NULL,
    price      BIGINT      NOT NULL CHECK (price >= 0),
    stock      INTEGER     NOT NULL CHECK (stock >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE sales (
    id         BIGSERIAL PRIMARY KEY,
    total      BIGINT      NOT NULL CHECK (total >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE sale_items (
    id         BIGSERIAL PRIMARY KEY,
    sale_id    BIGINT  NOT NULL REFERENCES sales (id) ON DELETE CASCADE,
    product_id BIGINT  NOT NULL REFERENCES products (id),
    qty        INTEGER NOT NULL CHECK (qty > 0),
    price      BIGINT  NOT NULL CHECK (price >= 0)
);

CREATE INDEX idx_sales_created_at ON sales (created_at);
CREATE INDEX idx_sale_items_sale_id ON sale_items (sale_id);
