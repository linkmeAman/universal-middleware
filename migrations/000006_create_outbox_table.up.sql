-- Create outbox messages table
CREATE TABLE IF NOT EXISTS outbox_messages (
    id UUID PRIMARY KEY,
    aggregate_type VARCHAR(255) NOT NULL,
    aggregate_id VARCHAR(255) NOT NULL,
    event_type VARCHAR(255) NOT NULL,
    payload JSONB NOT NULL,
    topic VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    published_at TIMESTAMP WITH TIME ZONE,
    retry_count INTEGER NOT NULL DEFAULT 0,
    error_message TEXT,
    metadata JSONB
);

-- Create indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_outbox_messages_status ON outbox_messages(status);
CREATE INDEX IF NOT EXISTS idx_outbox_messages_created_at ON outbox_messages(created_at);
CREATE INDEX IF NOT EXISTS idx_outbox_messages_published_at ON outbox_messages(published_at);
CREATE INDEX IF NOT EXISTS idx_outbox_messages_aggregate ON outbox_messages(aggregate_type, aggregate_id);

-- Create enum for message status
DO $$ BEGIN
    CREATE TYPE message_status AS ENUM ('pending', 'published', 'failed');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;