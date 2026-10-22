CREATE TABLE ticket_histories (
    id         UUID PRIMARY KEY,
    ticket_id  UUID        NOT NULL REFERENCES tickets (id) ON DELETE CASCADE,
    actor_id   UUID        REFERENCES users (id) ON DELETE SET NULL,
    action     TEXT        NOT NULL,
    old_value  TEXT,
    new_value  TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX ticket_histories_ticket_id_created_at_idx ON ticket_histories (ticket_id, created_at);
