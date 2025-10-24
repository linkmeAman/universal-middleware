-- Drop indexes
DROP INDEX IF EXISTS idx_outbox_messages_status;
DROP INDEX IF EXISTS idx_outbox_messages_created_at;
DROP INDEX IF EXISTS idx_outbox_messages_published_at;
DROP INDEX IF EXISTS idx_outbox_messages_aggregate;

-- Drop table
DROP TABLE IF EXISTS outbox_messages;

-- Drop enum
DROP TYPE IF EXISTS message_status;