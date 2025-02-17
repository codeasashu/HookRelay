CREATE TABLE IF NOT EXISTS hookrelay.event_delivery (
    id INT PRIMARY KEY AUTO_INCREMENT NOT NULL,
    event_type VARCHAR(255) NOT NULL,
    payload TEXT,
    owner_id VARCHAR(255) NOT NULL,
    subscription_id VARCHAR(255) NOT NULL,
    status_code INT,
    error TEXT,
    start_at TIMESTAMP NULL DEFAULT NULL,
    complete_at TIMESTAMP NULL DEFAULT NULL,
);

-- Optional: Add indexes for faster queries
CREATE INDEX idx_eventdelivery_owner_id ON hookrelay.event_delivery (owner_id);
CREATE INDEX idx_eventdelivery_subscription_id ON hookrelay.event_delivery (subscription_id);
CREATE INDEX idx_eventdelivery_event_type ON hookrelay.event_delivery (event_type);
CREATE INDEX idx_eventdelivery_start_at ON hookrelay.event_delivery (start_at);
