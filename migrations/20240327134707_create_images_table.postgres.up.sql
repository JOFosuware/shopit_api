CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE images (
    public_id      VARCHAR(300)                      PRIMARY KEY,
    url VARCHAR(300)                        NOT NULL,
    product_id UUID                         NOT NULL REFERENCES products(product_id) ON DELETE CASCADE,
    created_at   TIMESTAMP WITH TIME ZONE   NOT NULL    DEFAULT NOW()
)