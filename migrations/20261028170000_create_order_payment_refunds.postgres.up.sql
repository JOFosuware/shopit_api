CREATE TABLE order_payment_refunds (
    payment_id VARCHAR(300) PRIMARY KEY,
    status VARCHAR(20) NOT NULL CHECK (status IN ('pending', 'refunded')),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);
