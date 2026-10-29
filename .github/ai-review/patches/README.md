# Fiach CI overlay

Source is fetched at the exact commit in `../upstream.json`. `fetch.py` verifies
HEAD, checks each patch before applying it, and leaves dependency pins/lockfile
unchanged. No upstream code is vendored or silently replaced with a second engine.

`ci-boundary.patch` makes only two changes to `src/review.rs`:

1. In `NUTMIX_CI` mode, validate the checkout's head/base against the host-authorized
   revision immediately after workspace preparation, before provider creation.
   Both expected revisions must be present. Normal upstream invocation is unchanged.
2. Allow CI-provided PPQ token-price fallbacks in every phase. The verifier does
   not otherwise receive explicit CLI price overrides. Real provider charges still
   take precedence in upstream cost accounting.

`export.rs` is added as a separate `nutmix-export` binary in the upstream crate.
It uses Fiach's `validate_artifact`, `publishable_findings`, and diff-anchor parser;
it does not implement a separate finding acceptance policy. It adds host-boundary
checks for manifest identity, complete verdict coverage, fixed filenames and size
limits. It never follows metadata-provided artifact paths or accepts worker anchors
as authoritative. It runs offline, without credentials, in a fresh container.

`boundary_tests.rs` adds a separate test target. `mock_test.py` drives the actual
Fiach/Goose coordinator, delegated inspection lane, verifier and exporter against
a local unpaid OpenAI-compatible fixture. Those tests use synthetic findings,
not claims about real bugs or model accuracy. Fixtures reuse an existing commit;
they do not create commits or contact GitHub/PPQ.

When updating Fiach: review the new source/dependency diff, update the exact pin,
rebase/check the overlay, run all upstream/adapter/mock tests, and rebuild the
trusted image. Do not just change the revision or use a floating branch/tag.
