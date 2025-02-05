CREATE TABLE IF NOT EXISTS hookrelay.event (
    id varchar PRIMARY KEY NOT NULL,
    owner_id varchar NOT NULL,
    event_type TEXT NOT NULL,
    payload JSONB,
    idempotency_key varchar,
    accepted_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    modified_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Optional: Add indexes for faster queries
CREATE INDEX idx_event_owner_id ON hookrelay.event (owner_id);
CREATE INDEX idx_event_event_type ON hookrelay.event (event_type);
