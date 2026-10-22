CREATE TABLE tickets (
    id          UUID PRIMARY KEY,
    title       TEXT        NOT NULL,
    description TEXT        NOT NULL,
    category    TEXT        NOT NULL,
    priority    TEXT        NOT NULL CHECK (priority IN ('LOW', 'MEDIUM', 'HIGH', 'CRITICAL')),
    status      TEXT        NOT NULL CHECK (status IN ('OPEN', 'IN_PROGRESS', 'RESOLVED', 'CLOSED')),
    created_by  UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    assigned_to UUID        REFERENCES users (id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Listing is always ordered by created_at DESC, and the default filter is by
-- status, so these two indexes cover the hot query paths.
CREATE INDEX tickets_created_at_idx ON tickets (created_at DESC);
CREATE INDEX tickets_status_created_at_idx ON tickets (status, created_at DESC);
CREATE INDEX tickets_priority_created_at_idx ON tickets (priority, created_at DESC);
CREATE INDEX tickets_assigned_to_idx ON tickets (assigned_to);
CREATE INDEX tickets_created_by_idx ON tickets (created_by);

-- Full text search on the two fields users actually search by.
CREATE INDEX tickets_search_idx ON tickets USING GIN (to_tsvector('simple', title || ' ' || description));
