-- +goose Up
-- Also upgrade databases that already applied the reviewed migration 42:
-- CREATE OR REPLACE preserves the trigger bindings while adding error metadata.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION protect_signed_blind_message() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.state = 'SIGNED' THEN
        RAISE EXCEPTION 'signed blinded messages are immutable'
            USING ERRCODE = '23514', CONSTRAINT = 'blind_messages_signed_immutable', TABLE = TG_TABLE_NAME;
    END IF;
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION protect_blind_signature() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'blind signatures are immutable'
        USING ERRCODE = '23514', CONSTRAINT = 'blind_signatures_immutable', TABLE = TG_TABLE_NAME;
END;
$$;
-- +goose StatementEnd

ALTER TABLE blind_messages
    ALTER COLUMN amount DROP NOT NULL,
    ADD COLUMN melt_quote_id text REFERENCES melt_request(quote) ON DELETE RESTRICT,
    ADD COLUMN output_position integer,
    ADD CONSTRAINT blind_messages_change_owner CHECK (
        (melt_quote_id IS NULL AND output_position IS NULL)
        OR (melt_quote_id IS NOT NULL AND output_position IS NOT NULL
            AND output_position >= 0 AND mint_quote_id IS NULL)),
    ADD CONSTRAINT blind_messages_known_amount CHECK (
        (state = 'SIGNED' AND amount IS NOT NULL)
        OR (state = 'PENDING' AND (
            (melt_quote_id IS NOT NULL AND amount IS NULL)
            OR (melt_quote_id IS NULL AND amount IS NOT NULL))));

-- Supports ordered pending/signed change lookup and melt-quote FK checks.
CREATE UNIQUE INDEX blind_messages_melt_position
    ON blind_messages (melt_quote_id, output_position);
-- Finalization checks the complete durable input set under the quote lock.
-- Most swap proofs have no quote and do not need entries in this lookup index.
CREATE INDEX proofs_quote_lookup ON proofs (quote) WHERE quote IS NOT NULL;

-- The old table has no ordinal. Preserve all blanks, assigning a deterministic
-- order for legacy rows; the original wallet order cannot be reconstructed.
-- Duplicate points, signed-message overlap, and missing quotes fail atomically.
INSERT INTO blind_messages (blinded_message, amount, keyset_id, state,
    melt_quote_id, output_position, created_at)
SELECT "B_", NULL, id, 'PENDING', quote,
    (row_number() OVER (PARTITION BY quote ORDER BY created_at, "B_") - 1)::integer,
    created_at
FROM melt_change_message;

DROP TABLE melt_change_message;

-- +goose Down
-- Neither change ordering nor permanent signed-change provenance can be
-- represented by the previous schema. Refuse instead of discarding them.
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM blind_messages WHERE melt_quote_id IS NOT NULL) THEN
        RAISE EXCEPTION 'melt-owned blinded messages prevent a lossless downgrade'
            USING ERRCODE = '55000', CONSTRAINT = 'melt_change_lifecycle_lossless_downgrade';
    END IF;
END;
$$;
-- +goose StatementEnd

CREATE TABLE melt_change_message (
    "B_" bytea NOT NULL,
    created_at bigint NOT NULL,
    quote text NOT NULL,
    id text NOT NULL
);
DROP INDEX blind_messages_melt_position;
DROP INDEX proofs_quote_lookup;
ALTER TABLE blind_messages
    DROP CONSTRAINT blind_messages_known_amount,
    DROP CONSTRAINT blind_messages_change_owner,
    DROP COLUMN output_position,
    DROP COLUMN melt_quote_id,
    ALTER COLUMN amount SET NOT NULL;
