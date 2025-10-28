CREATE TABLE IF NOT EXISTS commands (
    id UUID PRIMARY KEY,
    type VARCHAR(100) NOT NULL,
    entity_id VARCHAR(255) NOT NULL,
    payload JSONB NOT NULL,
    idempotency_key VARCHAR(255),
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ,
    error TEXT,
    retry_count INTEGER DEFAULT 0
);

CREATE INDEX idx_commands_status ON commands(status, created_at);
CREATE INDEX idx_commands_entity ON commands(entity_id, created_at);
CREATE INDEX idx_commands_idempotency ON commands(idempotency_key);
