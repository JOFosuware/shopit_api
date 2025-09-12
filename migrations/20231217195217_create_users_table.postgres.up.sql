CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE users (
    user_id      UUID PRIMARY KEY                       DEFAULT uuid_generate_v4(),
    name VARCHAR(64)                        NOT NULL    CHECK ( name <> '' ),
    email VARCHAR(64) UNIQUE                NOT NULL    CHECK ( email <> ''),
    password VARCHAR(250)                   NOT NULL    CHECK ( octet_length(password) <> 0 ),
    role VARCHAR(10)                       NOT NULL    DEFAULT 'user',
    created_at   TIMESTAMP WITH TIME ZONE   NOT NULL    DEFAULT NOW()
)