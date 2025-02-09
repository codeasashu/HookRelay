CREATE TABLE IF NOT EXISTS hookrelay.event_delivery (
    id VARCHAR(255) PRIMARY KEY NOT NULL,
    event_id VARCHAR(255) NOT NULL,
    owner_id VARCHAR(255) NOT NULL,
    subscription_id VARCHAR(255) NOT NULL,
    status_code INT,
    error TEXT,
    latency DOUBLE NOT NULL DEFAULT 0.0,
    started_at TIMESTAMP NULL DEFAULT NULL,
    completed_at TIMESTAMP NULL DEFAULT NULL,

    -- Foreign key constraints
    CONSTRAINT fk_event FOREIGN KEY (event_id) REFERENCES hookrelay.event(id) ON DELETE CASCADE
);

-- Optional: Add indexes for faster queries
CREATE INDEX idx_eventdelivery_owner_id ON hookrelay.event_delivery (owner_id);
CREATE INDEX idx_eventdelivery_event_id ON hookrelay.event_delivery (event_id);

-- Add a comment for latency column
ALTER TABLE hookrelay.event_delivery COMMENT = 'latency is in microseconds';
