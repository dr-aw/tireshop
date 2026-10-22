-- +goose Up
CREATE TABLE requests.requests (
    request_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_name text NOT NULL CHECK (length(btrim(customer_name)) BETWEEN 1 AND 100),
    phone text NOT NULL CHECK (phone ~ '^\+[1-9][0-9]{7,14}$'),
    comment text NOT NULL DEFAULT '' CHECK (length(comment) <= 2000),
    total_rub bigint NOT NULL CHECK (total_rub BETWEEN 0 AND 9007199254740991),
    status text NOT NULL DEFAULT 'created'
        CHECK (status IN ('created', 'delivery', 'delivered', 'cancelled')),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE requests.request_items (
    request_id uuid NOT NULL REFERENCES requests.requests (request_id),
    product_id uuid NOT NULL,
    name text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 300),
    quantity bigint NOT NULL CHECK (quantity BETWEEN 1 AND 9007199254740991),
    unit_price_rub bigint NOT NULL CHECK (unit_price_rub BETWEEN 0 AND 9007199254740991),
    total_rub bigint NOT NULL CHECK (total_rub BETWEEN 0 AND 9007199254740991),
    PRIMARY KEY (request_id, product_id),
    -- numeric prevents bigint overflow even for direct invalid SQL writes.
    CHECK (total_rub::numeric = quantity::numeric * unit_price_rub::numeric)
);
-- No FK to catalog: removing a product must not remove or invalidate history.

CREATE TABLE requests.request_submissions (
    idempotency_key uuid PRIMARY KEY,
    payload_hash bytea NOT NULL CHECK (octet_length(payload_hash) = 32),
    response jsonb,
    request_id uuid REFERENCES requests.requests (request_id),
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (response IS NULL OR jsonb_typeof(response) = 'object')
);

-- +goose StatementBegin
DO $$
BEGIN
    EXECUTE format('GRANT USAGE ON SCHEMA requests TO %I', current_database() || '_requests');
    EXECUTE format('GRANT SELECT, INSERT ON requests.requests, requests.request_items TO %I',
        current_database() || '_requests');
    EXECUTE format('GRANT SELECT, INSERT, UPDATE ON requests.request_submissions TO %I',
        current_database() || '_requests');
END $$;
-- +goose StatementEnd
