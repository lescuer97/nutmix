"""GitHub authorization/orchestration only. Fiach is the sole AI review engine."""

import hashlib
from contextlib import contextmanager
import html
import json
import math
import os
from pathlib import Path
import re
import signal
import stat
import subprocess
import sys
import tempfile
from urllib.error import HTTPError, URLError
from urllib.parse import urlencode
from urllib.request import Request, build_opener, HTTPRedirectHandler


ROOT = Path(__file__).resolve().parents[2]
TOOL = ROOT / ".github/ai-review"
REPOSITORY = "lescuer97/nutmix"
OWNER = "lescuer97"
MODEL = "moonshotai/kimi-k3"
MAX_RESPONSE = 8 * 1024 * 1024
ARTIFACTS = ("result.json", "report.structured.json", "report.policy.json")
TEST_DIRS = (
    "api/cashu", "cmd/nutmix", "internal/mint", "internal/signer",
    "internal/database/postgresql", "internal/lightning", "internal/routes/admin",
    "internal/routes/middleware", "internal/utils", "pkg/crypto", "test/configTest", "test/setupTest",
)
# Trusted minimum, even if a PR removes jobs. Add external required contexts here.
REQUIRED_CONTEXTS = set()
REQUIRED_JOBS = {
    "workflow.yml": {"build", "build-test", "ai-review-tests", "ai-review-image-tests"}
    | {f"test ({directory})" for directory in TEST_DIRS},
    "golangci-lint.yml": {"lint"},
    "docker-build-test.yml": {"Test Docker Build"},
}


class ReviewError(Exception):
    """Safe diagnostic; do not log API response bodies or model output."""


def require(condition, message):
    if not condition:
        raise ReviewError(message)


class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def request_json(url, token=None, data=None, *, raw=False):
    headers = {"Accept": "application/vnd.github.diff" if raw else "application/json"}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    body = None
    if data is not None:
        body = json.dumps(data).encode()
        headers["Content-Type"] = "application/json"
    try:
        with build_opener(NoRedirect()).open(Request(url, data=body, headers=headers), timeout=60) as response:
            result = response.read(MAX_RESPONSE + 1)
        require(len(result) <= MAX_RESPONSE, "API response exceeded the size limit.")
        return result.decode("utf-8") if raw else json.loads(result)
    except HTTPError as error:
        error.close()
        raise ReviewError(f"API request failed (HTTP {error.code}); no report published.") from None
    except (URLError, TimeoutError, ValueError, OSError):
        raise ReviewError("API transport/JSON error; no report published.") from None


class GitHub:
    def __init__(self):
        self.token = os.environ.get("GITHUB_TOKEN", "")
        require(self.token, "GITHUB_TOKEN is missing.")

    def get(self, path):
        return request_json("https://api.github.com" + path, self.token)

    def post(self, path, data):
        return request_json("https://api.github.com" + path, self.token, data)

    def diff(self, snap):
        diff = request_json(f"https://api.github.com/repos/{REPOSITORY}/compare/"
                            f"{snap['base']}...{snap['head']}", self.token, raw=True)
        require(diff.startswith("diff --git "), "Missing pinned diff; refusing publication.")
        return diff

    def pages(self, path, key=None):
        items = []
        for page in range(1, 32):
            separator = "&" if "?" in path else "?"
            result = self.get(f"{path}{separator}per_page=100&page={page}")
            batch = result[key] if key else result
            items.extend(batch)
            if len(batch) < 100:
                return items
        raise ReviewError("GitHub pagination limit reached; refusing incomplete data.")


def authorize_execution():
    require(os.environ.get("GITHUB_REPOSITORY") == REPOSITORY
            and os.environ.get("GITHUB_ACTOR") == OWNER
            and os.environ.get("REVIEW_TRIGGERING_ACTOR") == OWNER,
            "Only lescuer97 may execute or rerun this workflow.")


def authorize(github):
    authorize_execution()
    event = json.loads(Path(os.environ["GITHUB_EVENT_PATH"]).read_text())
    comment, issue = event.get("comment", {}), event.get("issue", {})
    require(event.get("action") == "created"
            and event.get("repository", {}).get("full_name") == REPOSITORY
            and event.get("sender", {}).get("login") == OWNER
            and comment.get("user", {}).get("login") == OWNER
            and comment.get("body") == "/ai-review" and issue.get("pull_request"),
            "Only the owner's exact /ai-review PR comment is accepted.")
    current = github.get(f"/repos/{REPOSITORY}/issues/comments/{int(comment['id'])}")
    require(current.get("user", {}).get("login") == OWNER and current.get("body") == "/ai-review"
            and current.get("issue_url") == f"https://api.github.com/repos/{REPOSITORY}/issues/{int(issue['number'])}",
            "The original owner request is no longer valid.")
    return int(issue["number"])


def pull(github, number):
    pr = github.get(f"/repos/{REPOSITORY}/pulls/{number}")
    require(pr["state"] == "open" and not pr["draft"], "PR must be open and ready for review.")
    require(pr.get("mergeable") is True and pr.get("merge_commit_sha"),
            "PR mergeability is unknown or conflicted; retry after GitHub computes it.")
    require(pr["head"].get("repo"), "PR source repository is unavailable.")
    return pr


def snapshot(pr):
    return {"number": pr["number"], "head": pr["head"]["sha"],
            "base": pr["base"]["sha"], "merge": pr["merge_commit_sha"]}


def check_ci(github, pr):
    shas = {pr["head"]["sha"], pr["merge_commit_sha"]}
    receipts = []
    for workflow, required in REQUIRED_JOBS.items():
        runs = []
        for sha in shas:
            query = urlencode({"event": "pull_request", "head_sha": sha})
            runs.extend(github.pages(f"/repos/{REPOSITORY}/actions/workflows/{workflow}/runs?{query}", "workflow_runs"))
        matching = [run for run in runs if any(
            item["number"] == pr["number"] and item["head"]["sha"] == pr["head"]["sha"]
            and item["base"]["sha"] == pr["base"]["sha"] for item in run.get("pull_requests", []))]
        require(matching, f"Missing current-PR CI run: {workflow}.")
        latest = max(matching, key=lambda run: run["id"])
        require(latest["status"] == "completed" and latest["conclusion"] == "success",
                f"CI must finish successfully: {workflow}.")
        jobs = github.pages(f"/repos/{REPOSITORY}/actions/runs/{latest['id']}/jobs?filter=latest", "jobs")
        require(required <= {job["name"] for job in jobs}, f"Required jobs are missing from {workflow}.")
        require(all(job["status"] == "completed" and job["conclusion"] == "success" for job in jobs),
                f"Every job must succeed (no skipped jobs): {workflow}.")
        receipts.append({"workflow": workflow, "id": latest["id"], "attempt": latest.get("run_attempt", 1)})
    contexts = set()
    for sha in shas:
        checks = github.pages(f"/repos/{REPOSITORY}/commits/{sha}/check-runs?filter=latest", "check_runs")
        require(all(check["status"] == "completed" and check["conclusion"] == "success" for check in checks),
                "A current-revision check is pending, skipped, cancelled, or unsuccessful.")
        contexts.update(check["name"] for check in checks)
        statuses = github.pages(f"/repos/{REPOSITORY}/commits/{sha}/statuses")
        latest_statuses = {}
        for status in statuses:
            latest_statuses.setdefault(status["context"], status["state"])
        require(all(state == "success" for state in latest_statuses.values()),
                "A current-revision commit status is pending or unsuccessful.")
        contexts.update(latest_statuses)
    require(REQUIRED_CONTEXTS <= contexts, "A configured required check has not run.")
    return receipts


def fingerprint():
    digest = hashlib.sha256()
    files = sorted(path for path in TOOL.rglob("*") if path.is_file() and "__pycache__" not in path.parts)
    files += [Path(__file__).resolve(), ROOT / ".github/workflows/ai-review.yml"]
    for path in files:
        digest.update(str(path.relative_to(ROOT)).encode() + b"\0" + path.read_bytes() + b"\0")
    return digest.hexdigest()


def pricing():
    result = request_json("https://api.ppq.ai/v1/models")
    models = [model for model in result["data"] if model["id"] == MODEL]
    require(len(models) == 1, "Kimi K3 is unavailable at PPQ; no model fallback is allowed.")
    prices = models[0]["pricing"]
    values = {"input": prices["input_per_1M_tokens"], "output": prices["output_per_1M_tokens"]}
    require(all(type(value) in (int, float) and math.isfinite(value) and value > 0 for value in values.values()),
            "PPQ pricing is unavailable; refusing an unbudgeted review.")
    return values


def write_json(path, value):
    Path(path).write_text(json.dumps(value), encoding="utf-8")


def prepare(github):
    number = authorize(github)
    pr = pull(github, number)
    receipts = check_ci(github, pr)
    event = json.loads(Path(os.environ["GITHUB_EVENT_PATH"]).read_text())
    manifest = {"schema": 1, "repository": REPOSITORY, "model": MODEL,
                "snapshot": snapshot(pr), "comment_id": event["comment"]["id"],
                "run_id": os.environ["GITHUB_RUN_ID"], "fingerprint": fingerprint(),
                "upstream": json.loads((TOOL / "upstream.json").read_text())["revision"],
                "ci_runs": receipts, "pricing": pricing()}
    require(snapshot(pull(github, number)) == manifest["snapshot"], "PR changed during authorization; request again.")
    write_json("ai-review-input.json", manifest)
    print("Owner request and current-PR CI authorized. Fiach manifest prepared.")


def read_manifest():
    data = read_json(Path("ai-review-input.json"))
    event = json.loads(Path(os.environ["GITHUB_EVENT_PATH"]).read_text())
    require(data["schema"] == 1 and data["repository"] == REPOSITORY and data["model"] == MODEL
            and data["run_id"] == os.environ["GITHUB_RUN_ID"]
            and data["comment_id"] == event["comment"]["id"] and data["fingerprint"] == fingerprint(),
            "Manifest provenance or trusted configuration mismatch.")
    require(all(re.fullmatch(r"[a-f0-9]{40}", data["snapshot"][key]) for key in ("head", "base", "merge")),
            "Invalid authorized commit identity.")
    return data


def check(github):
    number = authorize(github)
    data = read_manifest()
    pr = pull(github, number)
    require(data["snapshot"] == snapshot(pr), "PR changed while queued; request a new review.")
    check_ci(github, pr)
    return data


def read_regular(path):
    # Open without following links, then check the opened inode (not a prior stat).
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    with os.fdopen(fd, "rb") as handle:
        metadata = os.fstat(handle.fileno())
        require(stat.S_ISREG(metadata.st_mode) and metadata.st_nlink == 1 and metadata.st_size <= MAX_RESPONSE,
                "Unsafe or oversized artifact.")
        value = handle.read(MAX_RESPONSE + 1)
    require(len(value) <= MAX_RESPONSE, "Artifact exceeded size limit.")
    return value


def read_json(path):
    return json.loads(read_regular(path))


def sandbox(image, *, network="bridge"):
    require(re.fullmatch(r"sha256:[a-f0-9]{64}", image), "Expected trusted build job's exact image identity.")
    return ["docker", "run", "--rm", "--pull=never", "--read-only", "--cap-drop=ALL",
            "--security-opt=no-new-privileges:true", "--user=10001:10001", "--pids-limit=256",
            "--memory=5g", "--cpus=2", "--log-driver=none", "--network=" + network,
            "--tmpfs=/tmp:rw,nosuid,nodev,size=2g,mode=1777"]


def run_worker(image, data, output):
    key = os.environ.get("PPQ_API_KEY", "")
    token = os.environ.get("GITHUB_TOKEN", "")
    require(key and token, "PPQ_API_KEY and read-only GITHUB_TOKEN are required.")
    prices = data["pricing"]
    require(all(type(value) in (int, float) and math.isfinite(value) and value > 0
                for value in (prices["input"], prices["output"])), "Invalid PPQ pricing.")
    env = {**os.environ, "OPENAI_API_KEY": key, "OPENAI_BASE_URL": "https://api.ppq.ai/v1",
           "OPENAI_BASE_PATH": "v1/chat/completions", "CONTEXT_FILE_NAMES": "[]",
           "GOOSE_MAX_TOKENS": "8192", "GOOSE_CONTEXT_LIMIT": "1048576",
           "NUTMIX_CI": "1", "NUTMIX_EXPECTED_HEAD": data["snapshot"]["head"],
           "NUTMIX_EXPECTED_BASE": data["snapshot"]["base"],
           "NUTMIX_INPUT_PRICE": str(prices["input"]), "NUTMIX_OUTPUT_PRICE": str(prices["output"]),
           "RUST_LOG": "error", "GIT_CONFIG_GLOBAL": "/dev/null", "GIT_CONFIG_NOSYSTEM": "1"}
    # Secrets travel by variable NAME, never command-line values. No other host
    # variables (Actions runtime tokens, other provider keys, Docker config) enter.
    command = sandbox(image)
    for name in ("OPENAI_API_KEY", "OPENAI_BASE_URL", "OPENAI_BASE_PATH", "GITHUB_TOKEN",
                 "CONTEXT_FILE_NAMES", "GOOSE_MAX_TOKENS", "GOOSE_CONTEXT_LIMIT", "NUTMIX_CI",
                 "NUTMIX_EXPECTED_HEAD", "NUTMIX_EXPECTED_BASE", "NUTMIX_INPUT_PRICE",
                 "NUTMIX_OUTPUT_PRICE", "RUST_LOG", "GIT_CONFIG_GLOBAL", "GIT_CONFIG_NOSYSTEM"):
        command += ["--env", name]
    command += ["--mount", f"type=bind,source={output},target=/output", image,
                "--config", "/etc/nutmix-review/fiach.toml", "review", "--repo", REPOSITORY,
                "--pr", str(data["snapshot"]["number"]), "--sandbox-child",
                "--result-json", "/output/result.json"]
    name = "nutmix-review-" + output.parent.name
    command[2:2] = ["--name", name]
    try:
        result = subprocess.run(command, env=env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=3000)
        require(result.returncode == 0, "Fiach worker failed; no clean-review result or report is allowed.")
    finally:
        # Killing the Docker client alone would leave the model/container running.
        subprocess.run(["docker", "rm", "--force", name], stdout=subprocess.DEVNULL,
                       stderr=subprocess.DEVNULL, timeout=30)


@contextmanager
def bounded_output(directory):
    # Hosted Linux runner only. The mount is a unique throwaway host directory,
    # not a privileged container or a mount of runner home/workspace/secrets.
    output = Path(directory) / "output"
    output.mkdir()
    result = subprocess.run(["sudo", "-n", "mount", "-t", "tmpfs", "-o",
        "size=32m,mode=0777,nosuid,nodev,noexec", "tmpfs", str(output)],
        stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=30)
    require(result.returncode == 0, "A hosted Linux runner with sudo tmpfs mounting is required.")
    try:
        yield output
    finally:
        result = subprocess.run(["sudo", "-n", "umount", str(output)],
                                stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=30)
        require(result.returncode == 0, "Failed to unmount the disposable review output.")


def run(image):
    authorize_execution()
    def cancelled(signum, frame):
        raise ReviewError("Review cancelled; stopping the disposable worker.")
    signal.signal(signal.SIGTERM, cancelled)
    # Revalidate immediately before starting the secret-bearing worker.
    data = check(GitHub())
    destination = Path("ai-review-result")
    require(not destination.exists(), "Result destination already exists.")
    with tempfile.TemporaryDirectory(prefix="nutmix-fiach-", dir=os.environ.get("RUNNER_TEMP")) as directory:
        with bounded_output(directory) as output:
            run_worker(image, data, output)
            # Worker has exited; its namespace and all processes are gone. Copy only
            # allowlisted regular files, never recursively upload its output directory.
            safe = {name: read_regular(output / name) for name in ARTIFACTS}
        secrets = (os.environ["PPQ_API_KEY"], os.environ["GITHUB_TOKEN"])
        destination.mkdir()
        for name, raw in safe.items():
            for secret in secrets:
                raw = raw.replace(secret.encode(), b"[REDACTED]")
            (destination / name).write_bytes(raw)
    print("Fiach worker finished. Structured artifacts retained for offline host validation.")


def export_result(image, data, github):
    # Copy untrusted inputs into a private flat directory; do not mount downloaded
    # artifact directories directly or follow paths recorded in worker metadata.
    with tempfile.TemporaryDirectory(prefix="nutmix-export-", dir=os.environ.get("RUNNER_TEMP")) as directory:
        root = Path(directory)
        root.chmod(0o755)
        for name in ARTIFACTS:
            (root / name).write_bytes(read_regular(Path("ai-review-result") / name))
        write_json(root / "manifest.json", data)
        (root / "diff.txt").write_text(github.diff(data["snapshot"]))
        command = sandbox(image, network="none") + ["--mount", f"type=bind,source={root},target=/input,readonly",
            "--entrypoint", "nutmix-export", image, "/input", "/input/manifest.json", "/input/diff.txt"]
        # Output is bounded by exporter field/count limits. Never surface stderr,
        # which may contain malformed, model-controlled deserialization text.
        with tempfile.TemporaryFile() as stdout:
            result = subprocess.run(command, stdout=stdout, stderr=subprocess.DEVNULL, timeout=60)
            require(result.returncode == 0, "Fiach rejected incomplete, invalid, or mismatched review artifacts.")
            require(stdout.tell() <= MAX_RESPONSE, "Exporter response exceeded limit.")
            stdout.seek(0)
            exported = json.load(stdout)
    require(exported["schema"] == 1 and exported["snapshot"] == data["snapshot"]
            and exported["fingerprint"] == data["fingerprint"], "Export provenance mismatch.")
    return exported


def markdown(text):
    text = html.escape(text, quote=False).replace("@", "@\u200b")
    text = re.sub(r"(?i)(https?)(://)", r"\1:" + "\u200b//", text)
    text = re.sub(r"(?i)\bwww\.", "www\u200b.", text)
    return re.sub(r"([\\`*_{}\[\]()!#|])", r"\\\1", text)


def report(data, exported):
    snap = data["snapshot"]
    marker = f"<!-- nutmix-fiach:{snap['number']}:{snap['head']}:{snap['base']}:{data['fingerprint']} -->"
    body = ["## Fiach review — Kimi K3 via PPQ", f"Reviewed commit: `{snap['head']}`", ""]
    for finding in exported["findings"]:
        body += [f"### {markdown(finding['severity'].upper())}: {markdown(finding['title'])}", markdown(finding["body"])]
        locations = finding["inline_comments"] + finding["additional_locations"] + finding["unanchored_locations"]
        for location in locations[:8]:
            line = location.get("line", location.get("start_line"))
            body.append(f"Location: {markdown(location['path'])}" + (f":{line}" if line else ""))
    if not exported["findings"]:
        body.append("Review completed without new publishable findings. This is not approval or proof of safety.")
    body += ["Findings passed Fiach's independent verifier and evidence/disclosure policy. "
             "Inspection transcripts are not necessarily executed reproductions. Human review is still required.", marker]
    value = "\n\n".join(body)
    require(len(value.encode()) <= 60000, "Review exceeds GitHub's publication limit.")
    return value, marker


def publish(github, image):
    data = check(github)
    exported = export_result(image, data, github)
    body, marker = report(data, exported)
    number = data["snapshot"]["number"]
    previous = github.pages(f"/repos/{REPOSITORY}/pulls/{number}/reviews")
    if any(item["user"]["login"] == "github-actions[bot]" and marker in (item["body"] or "") for item in previous):
        print("This reviewer/revision has already been reported.")
        return
    # Final live recheck. Publication cannot be atomic with GitHub's mutable PR,
    # so always attach a COMMENT to the explicit reviewed commit, never approve.
    check(github)
    github.post(f"/repos/{REPOSITORY}/pulls/{number}/reviews", {
        "commit_id": data["snapshot"]["head"], "event": "COMMENT", "body": body})
    print("Published the commit-bound Fiach review.")


def main():
    command = sys.argv[1] if len(sys.argv) > 1 else ""
    if command == "sandbox-smoke" and len(sys.argv) == 3:
        subprocess.run(sandbox(sys.argv[2], network="none") + ["--entrypoint", "/bin/sh", sys.argv[2], "-ec",
            'test "$(id -u)" = 10001; test ! -e /var/run/docker.sock; '
            'test "$(stat -c %a /var/tmp)" = 1777; ! touch /var/tmp/nutmix-write-test 2>/dev/null; '
            'grep -Eq "^CapEff:[[:space:]]+0000000000000000$" /proc/self/status; '
            'grep -Eq "^NoNewPrivs:[[:space:]]+1$" /proc/self/status; '
            'test "$(ls /sys/class/net)" = lo; printf ok >/tmp/test; go version'], check=True, timeout=60)
    elif command == "prepare":
        prepare(GitHub())
    elif command == "run" and len(sys.argv) == 3:
        run(sys.argv[2])
    elif command == "publish" and len(sys.argv) == 3:
        publish(GitHub(), sys.argv[2])
    else:
        raise ReviewError("Expected prepare, sandbox-smoke IMAGE, run IMAGE, or publish IMAGE.")


if __name__ == "__main__":
    try:
        main()
    except ReviewError as error:
        print(str(error), file=sys.stderr)
        sys.exit(1)
    except (KeyError, TypeError, ValueError, OSError, subprocess.SubprocessError):
        print("Review data or execution failed; no clean-review result published.", file=sys.stderr)
        sys.exit(1)
