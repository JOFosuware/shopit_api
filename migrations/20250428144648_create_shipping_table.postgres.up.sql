CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE shippings (
    shipping_id      UUID PRIMARY KEY                       DEFAULT uuid_generate_v4(),
    address VARCHAR(100)                        NOT NULL       CHECK ( address <> '' ),
    city VARCHAR(100)                        NOT NULL       CHECK ( city <> '' ),
    phone VARCHAR(100)                        NOT NULL       CHECK ( phone <> '' ),
    postal VARCHAR(100)                        NOT NULL       CHECK ( postal <> '' ),
    country VARCHAR(100)                        NOT NULL       CHECK ( country <> '' ),
    order_id UUID                            NOT NULL REFERENCES orders(order_id) ON DELETE CASCADE,
    created_at   TIMESTAMP WITH TIME ZONE   NOT NULL    DEFAULT NOW()
)
