# Owner-requested Fiach review

Nutmix uses the **actual Fiach/Goose engine**, pinned to
`af2100c5f187b475865b2090979be13af2483939` from
<https://git.cashu.dev/thesimplekid/fiach>. This is a CI tool, not a mint runtime
dependency or background daemon. `go.mod` and Nutmix's application image are not
changed. The previous two-call Python AI reviewer has been removed; Python now
handles only GitHub authorization, CI orchestration, sandbox execution and report
formatting. There is no alternate model/reviewer fallback.

Fiach supplies repository exploration, restricted finder delegation, independent
verification, structured reporting, evidence requirements, duplicate suppression
and cost accounting. See [the small overlay](ai-review/patches/README.md) for the
two source adaptations and the offline exporter. Upstream is MIT/Apache-2.0;
both notices are retained in the built image. No dependency pins are modified.

## GitHub setup

1. Review and merge the integration into the repository's default branch. An
   `issue_comment` workflow does not run from an unmerged PR branch.
2. In **Settings → Environments**, create **`ai-review`**:
   - Set **`lescuer97` as the only required reviewer**.
   - Allow that owner to approve their own requests.
   - Restrict deployment branches to the default branch.
   - Add the environment secret **`PPQ_API_KEY`** with a dedicated, spending-limited
     PPQ key. Do **not** also expose it as an unprotected repository/organization
     secret; another workflow could consume those without this approval gate.
3. Keep GitHub Actions configured to allow the publisher job to write PR reviews.
4. Wait for every required CI job on an open, non-draft PR to pass.
5. As **`lescuer97`**, post a **new** comment containing exactly **`/ai-review`**.
6. Approve the protected environment. Authorization, revision and CI are checked
   again after image building/queueing/approval and before model execution.

Environment protection cannot be established by workflow YAML; an owner must set
it up in GitHub. Trigger checks alone are not a boundary against someone who can
edit workflows and use an unprotected secret. No secret should be pasted in chat,
source, comments, variables, build arguments or image layers.

Only `lescuer97` may request or rerun review execution/publication. Collaborator
access, PR authorship and author association do not grant access. Editing an
existing comment to add the command does not trigger it. Deleting/editing the
request revokes authorization at the next check. The request declares feature
readiness; automation cannot determine whether a feature is complete.

## Workflow and revision gating

1. **prepare** — Read-only GitHub access. Validate owner/PR/CI, confirm Kimi K3 is
   available, fetch public PPQ pricing, and record a manifest with head/base/merge
   SHAs, request identity, CI receipts and trusted configuration fingerprint.
2. **build-reviewer** — Build/test the pinned Fiach source and explicit overlay
   without provider credentials. Record the exact image ID and upload the image
   as a trusted, run-scoped artifact. CI PR-image caches are separate from this
   default-branch build cache.
3. **review** — After environment approval, revalidate the manifest, request and
   CI; run Fiach's artifact-only child mode inside the isolated container with
   read-only GitHub access and the PPQ key. Fiach independently checks the actual
   checked-out head/base before creating any model provider.
4. **publish** — No PPQ key. Download artifacts by the producer jobs' artifact IDs,
   load the same trusted image, obtain an independent pinned GitHub diff, and run
   the trusted Fiach exporter offline. Revalidate live authorization/revision/CI
   immediately before posting a commit-bound `COMMENT` review.

Required workflows are `Nutmix`, `golangci-lint`, and `Docker Build Test`, including
all build/test matrix legs and both reviewer test jobs. Missing, pending, failed,
cancelled or skipped jobs block review. Additional check runs/status contexts on
the head and current merge commit must also succeed. Maintain `REQUIRED_JOBS` and
`REQUIRED_CONTEXTS` in `.github/scripts/ai_review.py` when CI or external required
checks change; a configured required context that has not run also blocks review.
This is an explicit trusted policy, not automatic discovery of branch-protection
rules that might require broader token permissions.

**Currently all three required workflows run together only on `master` PRs.**
Other targets remain blocked until their CI triggers are deliberately expanded.
Runs must identify this PR and its current head/base. Forks are supported only
when GitHub supplies that association; ambiguous/missing association fails closed.
If the base advances, CI must test the updated base. An early request fails; it
does not automatically wait and start billing later. Check Actions logs and post
a new request after resolving the blocker.

## Provider and methodology

Fiach uses Goose's `openai` compatibility layer, explicitly configured with
`OPENAI_BASE_URL=https://api.ppq.ai/v1`, `OPENAI_BASE_PATH=v1/chat/completions`, and
`OPENAI_API_KEY` populated from the GitHub secret `PPQ_API_KEY`. Finder, delegated
lanes and verifier use **`moonshotai/kimi-k3`**. No other provider keys are forwarded;
Jev/TypeSafe lane selection is not enabled. GitHub is still used for source and
discussion access. PPQ receives repository source/context and prompts.

Trusted Nutmix prompts/skills are baked into the image, not loaded from the PR.
Goose's automatic `.goosehints`/`AGENTS.md` loading is disabled with
`CONTEXT_FILE_NAMES=[]`; such files may be inspected only as untrusted evidence.
The finder is inspection-only. The verifier may execute bounded reproductions in
the disposable container. It has no production services, mint keys or Docker
socket, so testcontainer/integration failures requiring that infrastructure cannot
justify expanding access. Inspection transcripts are not executed reproductions.

The custom persona covers concrete correctness, accounting, state-atomicity,
authorization, Cashu handling, concurrency and lifecycle regressions. Existing
protections and unchanged callers can be inspected by Fiach rather than omitted
as in the earlier static changed-file reviewer.

## Isolation, output and limitations

- Workflow code comes from `github.workflow_sha`, never the PR. The reviewer image
  is fetched from the trusted build job, never a PR/model-produced executable.
- The worker runs as UID 10001 with read-only root/configuration, dropped Linux
  capabilities, no-new-privileges, no host network/Docker socket/home/workspace
  mount, a 256-process limit, 2 CPUs, 5 GiB RAM and 2 GiB scratch tmpfs.
- Only its dedicated 32 MiB output tmpfs is writable on the host. The hosted Linux
  runner uses noninteractive `sudo mount/umount` solely for that unique throwaway
  directory. This is not a privileged container. Docker and sudo mount access are
  required to run the full integration locally.
- The worker receives only named environment values, a read-only GitHub token and
  a provider key. **A shell-capable verifier can read those container credentials.**
  Use a dedicated PPQ key with a hard spending cap and no unrelated exposure.
  Docker bridge networking is not destination allowlisting or a guarantee that
  host-local services are unreachable. Egress confinement would need a separate
  firewall/proxy boundary; do not assume it exists here.
- Worker stdout/stderr are not printed to Actions logs. After all worker processes
  terminate, only three allowlisted regular JSON files are copied out. Symlinks,
  hardlinks, FIFOs and oversized files are rejected; literal forwarded credentials
  are redacted. This does not guarantee detection of encoded secrets or malicious
  provider responses. Artifacts are retained for one day under normal GitHub
  access/retention rules; failures do not upload arbitrary workspace/log files.
- The exporter runs in a fresh container with **no network or credentials**, fixed
  input paths and bounded reads. It uses Fiach's own typed acceptance policy and
  independently obtained anchors. Unanchored accepted findings remain in the
  summary, rather than invalidating valid findings. Model Markdown is escaped
  before publication. No model-supplied path, command, binary or report is executed.
- Incomplete, budget-exhausted, markdown-only or missing-verdict results are never
  published as clean. A complete result without publishable findings is explicitly
  not approval or proof of safety. Reports are summary PR reviews, not approvals,
  commits, fixes or merges. Same head/base/configuration reports are deduplicated.
- GitHub checks and posting are not atomic. Reports always retain the reviewed
  commit ID, even if a PR changes immediately after the final live recheck.
- Limits start at $5 observed cost, 60 turns, 15 minutes per Fiach phase, two
  concurrent lanes, and 8,192 output tokens per request. Fiach retries are disabled,
  but Goose's internal transport retries remain. PPQ prices are supplied to all
  phases; in-flight calls/parallel lanes can overshoot observed cost limits. A PPQ
  account/key cap is the billing boundary. Missing model/pricing stops the review.
- Each exported JSON file and the independently retrieved diff must fit 8 MiB.
  Oversize/incomplete inputs fail closed rather than silently becoming clean.

## Validation and upgrades

```sh
python3 -B -m unittest discover -s .github/scripts -p 'test_*.py' -v
docker build -t nutmix-fiach:test .github/ai-review
image=$(docker image inspect --format '{{.Id}}' nutmix-fiach:test)
python3 .github/scripts/ai_review.py sandbox-smoke "$image"
```

Image builds run the adapter/boundary tests, upstream reporting/inspection tests,
and a local mock OpenAI API through the **actual Fiach/Goose** coordinator, delegated
inspection lane, independent verifier and exporter. Fixtures test confirmed,
no-findings, incomplete-verifier, exhausted-budget, HTTP-error and duplicate-suppression outcomes without
paid API calls. They validate integration mechanics, not Kimi's review accuracy or
live PPQ compatibility. The normal CI workflow runs image and orchestration tests.

Upgrades require reviewing the new source/dependency diff, updating the exact
upstream/base-image pins, rechecking the overlay, and passing the same tests. See
`ai-review/upstream.json` and `ai-review/patches/README.md`. After GitHub setup, an
owner-approved budget-limited real PR review is still required as the live smoke
test. No live PPQ calls or GitHub environment changes are performed by local tests.
