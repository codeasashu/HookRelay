CREATE TABLE IF NOT EXISTS hookrelay.event (
    id VARCHAR(255) PRIMARY KEY NOT NULL,
    owner_id VARCHAR(255) NOT NULL,
    event_type VARCHAR(255) NOT NULL,
    payload JSON, -- MySQL uses JSON instead of JSONB
    idempotency_key VARCHAR(255),
    accepted_at TIMESTAMP NULL DEFAULT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    modified_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- Optional: Add indexes for faster queries
CREATE INDEX idx_event_owner_id ON hookrelay.event (owner_id);
CREATE INDEX idx_event_event_type ON hookrelay.event (event_type);
