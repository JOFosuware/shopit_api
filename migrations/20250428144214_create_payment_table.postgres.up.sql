CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE payments (
    payment_id      VARCHAR(300)                        PRIMARY KEY,
    status VARCHAR(100)                        NOT NULL,
    order_id UUID                            NOT NULL REFERENCES orders(order_id) ON DELETE CASCADE,
    created_at   TIMESTAMP WITH TIME ZONE   NOT NULL    DEFAULT NOW()
)
