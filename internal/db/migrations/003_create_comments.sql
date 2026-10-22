CREATE TABLE comments (
    id         UUID PRIMARY KEY,
    ticket_id  UUID        NOT NULL REFERENCES tickets (id) ON DELETE CASCADE,
    user_id    UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    body       TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX comments_ticket_id_created_at_idx ON comments (ticket_id, created_at);
