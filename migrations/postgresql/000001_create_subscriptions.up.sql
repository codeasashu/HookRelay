CREATE SCHEMA IF NOT EXISTS hookrelay;

CREATE TABLE IF NOT EXISTS hookrelay.subscription (
    id varchar PRIMARY KEY NOT NULL,
    owner_id varchar NOT NULL,
    target_method varchar,
    target_url text,
    target_params JSONB,
    target_auth JSONB,
    event_types JSONB NOT NULL, -- JSONB for storing array of event types
    status INTEGER NOT NULL CHECK (status IN (0, 1)) DEFAULT 1, -- ENUM-like constraint
    filters JSONB,
    tags JSONB,
    created TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    modified TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

COMMENT ON COLUMN hookrelay.subscription.status IS '0=Disabled, 1=Active';

-- Optional: Add indexes for faster queries
CREATE INDEX idx_subscription_owner_id ON hookrelay.subscription (owner_id);
CREATE INDEX idx_subscription_event_types ON hookrelay.subscription USING GIN (event_types);
