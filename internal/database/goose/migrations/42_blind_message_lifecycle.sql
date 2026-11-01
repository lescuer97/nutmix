-- +goose Up
CREATE TABLE blind_signatures (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    signature bytea NOT NULL,
    dleq_e bytea,
    dleq_s bytea,
    created_at bigint NOT NULL,
    CONSTRAINT blind_signatures_dleq_pair CHECK ((dleq_e IS NULL) = (dleq_s IS NULL))
);
-- Signing timestamps remain Unix seconds, as used by issuance statistics.
CREATE INDEX blind_signatures_created_at ON blind_signatures (created_at);

CREATE TABLE blind_messages (
    blinded_message bytea PRIMARY KEY,
    amount numeric(20, 0) NOT NULL CHECK (amount > 0 AND amount <= 18446744073709551615),
    keyset_id text NOT NULL CHECK (keyset_id <> ''),
    state text NOT NULL CHECK (state IN ('PENDING', 'SIGNED')),
    signature_id bigint UNIQUE REFERENCES blind_signatures(id) ON DELETE RESTRICT,
    mint_quote_id text REFERENCES mint_request(quote) ON DELETE RESTRICT,
    created_at bigint NOT NULL,
    CONSTRAINT blind_messages_signature_state CHECK ((state = 'PENDING' AND signature_id IS NULL)
        OR (state = 'SIGNED' AND signature_id IS NOT NULL))
);
CREATE INDEX blind_messages_mint_quote ON blind_messages (mint_quote_id);

-- Capture the mapping once, so messages link to the correct migrated signature.
-- Invalid legacy amounts or incomplete DLEQ pairs fail the entire migration.
CREATE TEMP TABLE recovery_signature_split ON COMMIT DROP AS
SELECT row_number() OVER (ORDER BY "B_") AS signature_id,
    id, amount, "B_", "C_", created_at, dleq_e, dleq_s FROM recovery_signature;

INSERT INTO blind_signatures (id, signature, dleq_e, dleq_s, created_at)
OVERRIDING SYSTEM VALUE
SELECT signature_id, "C_", dleq_e, dleq_s, created_at FROM recovery_signature_split;

INSERT INTO blind_messages (blinded_message, amount, keyset_id, state, signature_id, created_at)
SELECT "B_", amount, id, 'SIGNED', signature_id, created_at FROM recovery_signature_split;

SELECT setval(pg_get_serial_sequence('blind_signatures', 'id'),
    COALESCE((SELECT max(id) FROM blind_signatures), 1),
    EXISTS (SELECT 1 FROM blind_signatures));

DROP TABLE recovery_signature;

-- Signed outputs are permanent recovery records. Pending ownership and attempt
-- integration are deliberately left to the separate mint-reservation change.
-- +goose StatementBegin
CREATE FUNCTION protect_signed_blind_message() RETURNS trigger LANGUAGE plpgsql AS $$
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
CREATE TRIGGER protect_signed_blind_message
BEFORE UPDATE OR DELETE ON blind_messages
FOR EACH ROW EXECUTE FUNCTION protect_signed_blind_message();

-- +goose StatementBegin
CREATE FUNCTION protect_blind_signature() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'blind signatures are immutable'
        USING ERRCODE = '23514', CONSTRAINT = 'blind_signatures_immutable', TABLE = TG_TABLE_NAME;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER protect_blind_signature
BEFORE UPDATE OR DELETE ON blind_signatures
FOR EACH ROW EXECUTE FUNCTION protect_blind_signature();
CREATE TRIGGER protect_blind_signatures_truncate
BEFORE TRUNCATE ON blind_signatures
FOR EACH STATEMENT EXECUTE FUNCTION protect_blind_signature();
CREATE TRIGGER protect_blind_messages_truncate
BEFORE TRUNCATE ON blind_messages
FOR EACH STATEMENT EXECUTE FUNCTION protect_blind_signature();

-- +goose Down
-- Refuse lossy downgrades: the previous schema cannot represent pending
-- messages, uint64 amounts above int4, or known mint-quote provenance.
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM blind_messages
        WHERE state <> 'SIGNED' OR amount > 2147483647 OR mint_quote_id IS NOT NULL)
        OR EXISTS (SELECT 1 FROM blind_signatures s
            WHERE NOT EXISTS (SELECT 1 FROM blind_messages m WHERE m.signature_id = s.id)) THEN
        RAISE EXCEPTION 'blinded-message data cannot be represented by the recovery_signature schema'
            USING ERRCODE = '55000', CONSTRAINT = 'blind_message_lifecycle_lossless_downgrade';
    END IF;
END;
$$;
-- +goose StatementEnd

CREATE TABLE recovery_signature (
    amount int4,
    id text NOT NULL,
    "B_" bytea NOT NULL,
    "C_" bytea NOT NULL,
    created_at bigint NOT NULL,
    dleq_e bytea,
    dleq_s bytea,
    CONSTRAINT unique_recovery_b_ UNIQUE ("B_")
);
INSERT INTO recovery_signature (amount, id, "B_", "C_", created_at, dleq_e, dleq_s)
SELECT m.amount::int4, m.keyset_id, m.blinded_message, s.signature, s.created_at, s.dleq_e, s.dleq_s
FROM blind_messages m JOIN blind_signatures s ON s.id = m.signature_id;

DROP TABLE blind_messages;
DROP TABLE blind_signatures;
DROP FUNCTION protect_signed_blind_message();
DROP FUNCTION protect_blind_signature();
