CREATE TABLE IF NOT EXISTS hookrelay.event_delivery (
    id varchar PRIMARY KEY NOT NULL,
    event_id varchar NOT NULL references hookrelay.event(id),
    owner_id varchar NOT NULL,
    subscription_id varchar NOT NULL references hookrelay.subscription(id),
    status_code int,
    error text,
    latency DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE
);

-- Optional: Add indexes for faster queries
CREATE INDEX idx_eventdelivery_owner_id ON hookrelay.event_delivery (owner_id);
CREATE INDEX idx_eventdelivery_event_id ON hookrelay.event_delivery (event_id);
