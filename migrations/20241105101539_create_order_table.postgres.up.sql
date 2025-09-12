CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE orders (
    order_id      UUID PRIMARY KEY                       DEFAULT uuid_generate_v4(),
    item_price INTEGER                          NOT NULL,
    tax_price INTEGER                          NOT NULL,
    shipping_price INTEGER                          NOT NULL,
    total_price INTEGER                          NOT NULL,
    order_status      VARCHAR(100)               NOT NULL        DEFAULT 'Processing',
    paid_at TIMESTAMP WITH TIME ZONE,
    delivered_at TIMESTAMP WITH TIME ZONE,
    user_id UUID                            NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    created_at   TIMESTAMP WITH TIME ZONE   NOT NULL    DEFAULT NOW()
)