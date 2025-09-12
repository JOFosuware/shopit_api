CREATE TABLE avatar (
    public_id VARCHAR(255) PRIMARY KEY         NOT NULL,
    url VARCHAR(255)                           NOT NULL,
    user_id UUID                               NOT NULL REFERENCES users(user_id) ON DELETE CASCADE 
)