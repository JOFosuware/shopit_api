CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE products (
    product_id      UUID PRIMARY KEY                       DEFAULT uuid_generate_v4(),
    name VARCHAR(64)                        NOT NULL       CHECK ( name <> '' ),
    price INTEGER                           NOT NULL,
    description VARCHAR(1000)               NOT NULL,
    ratings INTEGER                         NOT NULL,
    category VARCHAR(250)                   NOT NULL,
    seller VARCHAR(250)                     NOT NULL,
    stock INTEGER                           NOT NULL,
    num_of_reviews INTEGER                  DEFAULT 0,
    user_id UUID                            NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    created_at   TIMESTAMP WITH TIME ZONE   NOT NULL    DEFAULT NOW()
)