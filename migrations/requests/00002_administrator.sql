-- +goose Up
CREATE TABLE requests.admin_users (
    username text PRIMARY KEY CHECK (username ~ '^[a-z0-9_.-]{1,80}$'),
    password_hash text NOT NULL,
    active boolean NOT NULL DEFAULT true,
    role text NOT NULL DEFAULT 'admin' CHECK (role = 'admin')
);
CREATE TABLE requests.admin_sessions (
    token_hash bytea PRIMARY KEY CHECK (octet_length(token_hash) = 32),
    username text NOT NULL REFERENCES requests.admin_users(username),
    expires_at timestamptz NOT NULL
);
CREATE INDEX ON requests.admin_sessions (expires_at);
CREATE TABLE requests.request_history (
    event_id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    request_id uuid NOT NULL REFERENCES requests.requests(request_id),
    actor text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    comment text NOT NULL,
    before_snapshot jsonb NOT NULL,
    after_snapshot jsonb NOT NULL
);
-- +goose StatementBegin
DO $$ BEGIN
    EXECUTE format('GRANT SELECT ON requests.admin_users TO %I', current_database() || '_requests');
    EXECUTE format('GRANT SELECT, INSERT, DELETE ON requests.admin_sessions TO %I', current_database() || '_requests');
    EXECUTE format('GRANT UPDATE ON requests.requests TO %I', current_database() || '_requests');
    EXECUTE format('GRANT UPDATE, DELETE ON requests.request_items TO %I', current_database() || '_requests');
    EXECUTE format('GRANT SELECT, INSERT ON requests.request_history TO %I', current_database() || '_requests');
    EXECUTE format('GRANT USAGE ON SEQUENCE requests.request_history_event_id_seq TO %I', current_database() || '_requests');
END $$;
-- +goose StatementEnd
