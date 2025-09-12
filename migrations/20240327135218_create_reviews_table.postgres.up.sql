CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE reviews (
    reviews_id      UUID PRIMARY KEY                       DEFAULT uuid_generate_v4(),
    name VARCHAR(100)                        NOT NULL       CHECK ( name <> '' ),
    ratings INTEGER                          NOT NULL,
    comment VARCHAR(1000)                    NOT NULL,
    product_id UUID                            NOT NULL REFERENCES products(product_id) ON DELETE CASCADE,
    user_id UUID                            NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    created_at   TIMESTAMP WITH TIME ZONE   NOT NULL    DEFAULT NOW()
)