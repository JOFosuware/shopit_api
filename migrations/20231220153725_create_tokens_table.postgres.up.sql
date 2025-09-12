CREATE TABLE tokens (
    token_id      UUID PRIMARY KEY                       DEFAULT uuid_generate_v4(),
    token_hash bytea                        NOT NULL,
    expiry TIMESTAMP WITH TIME ZONE         NOT NULL,
    user_id      UUID                       NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    created_at   TIMESTAMP WITH TIME ZONE   NOT NULL    DEFAULT NOW(),
    updated_at   TIMESTAMP WITH TIME ZONE             DEFAULT CURRENT_TIMESTAMP
)