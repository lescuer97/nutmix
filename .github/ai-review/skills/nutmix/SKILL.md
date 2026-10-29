---
name: nutmix
description: Review context for Nutmix's Go Cashu mint
---

Trace code rather than assume behavior from endpoint names. Relevant boundaries:

- `api/cashu`: protocol messages, validation and spending conditions.
- `internal/mint`: mint, swap, melt, quotes and state transitions.
- `internal/database/postgresql`: persistence, locks, transactions and migrations.
- `internal/lightning`: settlement outcomes, retries and ambiguous payments.
- `internal/signer` and `pkg/crypto`: keysets, signatures and cryptographic checks.
- `internal/routes`: public/admin authorization and WebSocket resource ownership.

Investigate whether an input proof can fund more than one successful operation;
whether minted value is backed by the actual received payment; whether amounts,
denominations and fees conserve value; and whether retries/crashes can produce
issuance, payment, or database state inconsistent with earlier outcomes.

Verify spending conditions, quote binding, reconciliation, async callbacks and
cleanup through their actual callers. Check the relevant protocol specification
before calling a behavior a compliance violation. Re-signing an identical blinded
message does not by itself prove additional spendable value: show a complete
attack or correctness failure, including issuance and single-spend protections.

Findings need concrete base-versus-PR evidence. A model verdict is not proof of an
executed reproduction. Missing prerequisites or unavailable context mean reject
or explicitly leave the candidate unresolved; never fabricate evidence.
