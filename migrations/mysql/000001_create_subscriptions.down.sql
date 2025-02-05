DROP INDEX IF EXISTS idx_subscription_event_types;
DROP INDEX IF EXISTS idx_subscription_owner_id;

-- Drop the subscription table
DROP TABLE IF EXISTS hookrelay.subscription;
