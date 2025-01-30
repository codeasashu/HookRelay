CREATE TABLE IF NOT EXISTS hookrelay.subscription (
    id VARCHAR(255) PRIMARY KEY NOT NULL,
    owner_id VARCHAR(255) NOT NULL,
    target_method VARCHAR(255),
    target_url TEXT,
    target_params JSON,
    target_auth JSON,
    event_types JSON NOT NULL, -- JSON for storing array of event types
    status TINYINT NOT NULL CHECK (status IN (0, 1)) DEFAULT 1, -- ENUM-like constraint
    filters JSON,
    tags JSON,
    created TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    modified TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- Add a comment for the status column
ALTER TABLE hookrelay.subscription MODIFY COLUMN status TINYINT COMMENT '0=Disabled, 1=Active';

-- Optional: Add indexes for faster queries
CREATE INDEX idx_subscription_owner_id ON hookrelay.subscription (owner_id);
CREATE INDEX idx_subscription_event_types ON hookrelay.subscription ( (CAST(event_types AS CHAR(255))) );
