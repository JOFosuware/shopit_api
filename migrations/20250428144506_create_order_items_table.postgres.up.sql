CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE order_items (
    item_id      UUID PRIMARY KEY                       DEFAULT uuid_generate_v4(),
    name VARCHAR(100)                        NOT NULL       CHECK ( name <> '' ),
    quantity INTEGER                          NOT NULL,
    image VARCHAR(1000)                    NOT NULL,
    price INTEGER                          NOT NULL,
    product_id UUID                            NOT NULL REFERENCES products(product_id) ON DELETE CASCADE,
    order_id UUID                            NOT NULL REFERENCES orders(order_id) ON DELETE CASCADE,
    created_at   TIMESTAMP WITH TIME ZONE   NOT NULL    DEFAULT NOW()
)
