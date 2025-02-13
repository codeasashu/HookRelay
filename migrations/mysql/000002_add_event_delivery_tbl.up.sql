CREATE TABLE IF NOT EXISTS hookrelay.event_delivery (
    id INT PRIMARY KEY AUTO_INCREMENT NOT NULL,
    event_type VARCHAR(255) NOT NULL,
    payload TEXT,
    subscription_id VARCHAR(255) NOT NULL,
    status_code INT,
    error TEXT,
    created_at TIMESTAMP NULL DEFAULT NULL,

    -- Foreign key constraints
    CONSTRAINT fk_subscription FOREIGN KEY (subscription_id) REFERENCES hookrelay.subscription(id) ON DELETE NO ACTION
);

-- Optional: Add indexes for faster queries
CREATE INDEX idx_eventdelivery_subscription_id ON hookrelay.event_delivery (subscription_id);
CREATE INDEX idx_eventdelivery_event_type ON hookrelay.event_delivery (event_type);
