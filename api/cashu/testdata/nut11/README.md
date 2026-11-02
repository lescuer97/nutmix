# NUT-11 vectors

`vectors.json` contains all 20 JSON blocks from the pinned
[cashubtc/nuts NUT-11 test document](https://github.com/cashubtc/nuts/blob/1363de413294e2ef3a57b16950f9506af38fffc2/tests/11-test.md).
It records source-block order, a readable case name, vector kind, expected result,
and the unchanged source payload. The manifest includes the source SHA-256.

When updating vectors, retain every source block and update its metadata beside
its payload. `condition_only` marks the unsigned condition template, where
signature verification is not applicable.
