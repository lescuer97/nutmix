"""Exercise the actual Fiach/Goose tool loop against a local, unpaid API fixture.

This tests integration mechanics, not model accuracy. No fixture creates commits,
contacts GitHub, or executes Nutmix code. The only verifier command prints a marker.
"""

import argparse
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import math
import os
from pathlib import Path
import subprocess
import tempfile
import threading


MODEL = "moonshotai/kimi-k3"
CANDIDATE = {"title": "Synthetic test candidate", "severity": "high", "confidence": "high",
             "affected_locations": [{"path": "Cargo.toml", "start_line": 1}],
             "evidence": "Mock fixture only", "skills_used": ["nutmix"],
             "body_markdown": "Synthetic integration fixture, not a real repository finding."}


class Fixture:
    def __init__(self, mode):
        self.mode = mode
        self.stages = set()
        self.requests = 0

    def reply(self, data):
        assert data["model"] == MODEL
        self.requests += 1
        assert self.requests <= 40, "Unbounded provider loop"
        names = [tool["function"]["name"] for tool in data.get("tools", [])]
        calls = [call["function"]["name"] for message in data["messages"]
                 for call in message.get("tool_calls", [])]

        def tool(suffix, args):
            matches = [name for name in names if name.endswith(suffix)]
            assert len(matches) == 1, (suffix, names)
            name = matches[0]
            return {"id": f"call-{self.requests}", "type": "function",
                    "function": {"name": name, "arguments": json.dumps(args)}}

        # Reporting tools are listed in every phase but host-restricted when
        # called. Distinguish sessions by their actual execution capabilities.
        if any(name.endswith("shell") for name in names):
            self.stages.add("verifier")
            if self.mode == "incomplete":
                return "No structured verdict submitted", None
            if not any(name.endswith("shell") for name in calls):
                return None, tool("shell", {"command": "printf nutmix-mock-evidence"})
            if not any(name.endswith("submit_verdict") for name in calls):
                return None, tool("submit_verdict", {"finding_id": "F-1", "confirmed": True,
                    "introduced_by_pr": True, "present_on_pr_branch": True,
                    "present_on_base": False, "present_on_default_branch": False,
                    "disclosure_decision": "disclose", "rationale": "Synthetic integration test only",
                    "command_transcripts": [{"command": "printf nutmix-mock-evidence",
                        "branch_or_commit": "fixture", "key_output": "nutmix-mock-evidence",
                        "interpretation": "Executed marker command; not a real bug reproduction"}]})
            return "Verification fixture complete", None
        if any(name.endswith("delegate") for name in names):
            self.stages.add("coordinator")
            delegated = sum(name.endswith("delegate") for name in calls)
            lanes = ("persona", "correctness", "concurrency")
            if delegated < len(lanes):
                return None, tool("delegate", {"instructions": f"Review lane: {lanes[delegated]}\nRead .pr_diff.txt with inspect, then return synthetic fixture candidates as JSON.",
                    "async": False, "max_turns": 4, "extensions": ["fiach-inspection"]})
            suffix = "submit_no_findings" if self.mode == "clean" else "submit_finding"
            if not any(name.endswith(suffix) for name in calls):
                args = {"summary": "Synthetic no-findings fixture", "skills_used": ["nutmix"]} if self.mode == "clean" else CANDIDATE
                return None, tool(suffix, args)
            return "Coordinator fixture complete", None
        if any(name.endswith("inspect") for name in names):
            self.stages.add("lane")
            if not any(name.endswith("inspect") for name in calls):
                return None, tool("inspect", {"operation": "read", "path": ".pr_diff.txt"})
            return json.dumps({"findings": [] if self.mode == "clean" else [CANDIDATE]}), None
        if any(name.endswith("submit_duplicate_decision") for name in names):
            self.stages.add("dedupe")
            if not any(name.endswith("submit_duplicate_decision") for name in calls):
                return None, tool("submit_duplicate_decision", {"finding_id": "F-1", "already_reported": True,
                    "matching_comment_ids": [1], "confidence": "high", "rationale": "Same synthetic fixture issue"})
            return "Duplicate fixture complete", None
        raise AssertionError(f"Unexpected tool set: {names}")


def server(fixture):
    class Handler(BaseHTTPRequestHandler):
        def log_message(self, *args):
            pass

        def do_POST(self):
            assert self.path == "/v1/chat/completions"
            assert self.headers["Authorization"] == "Bearer sk-mock-only"
            data = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
            if fixture.mode == "provider-error":
                assert data["model"] == MODEL
                fixture.requests += 1
                self.send_response(400)
                self.send_header("Content-Type", "application/json")
                self.end_headers()
                self.wfile.write(b'{"error":{"message":"Synthetic request rejection","type":"invalid_request_error"}}')
                return
            text, call = fixture.reply(data)
            message = {"role": "assistant", "content": text}
            if call:
                message["tool_calls"] = [call]
            reason = "tool_calls" if call else "stop"
            usage = {"prompt_tokens": 100, "completion_tokens": 50, "total_tokens": 150}
            self.send_response(200)
            if data.get("stream"):
                self.send_header("Content-Type", "text/event-stream")
                self.end_headers()
                delta = dict(message)
                if call:
                    delta["tool_calls"] = [{"index": 0, **call}]
                chunks = [
                    {"id": "mock", "object": "chat.completion.chunk", "created": 1, "model": MODEL,
                     "choices": [{"index": 0, "delta": delta, "finish_reason": None}]},
                    {"id": "mock", "object": "chat.completion.chunk", "created": 1, "model": MODEL,
                     "choices": [{"index": 0, "delta": {}, "finish_reason": reason}], "usage": usage},
                ]
                for chunk in chunks:
                    self.wfile.write(("data: " + json.dumps(chunk) + "\n\n").encode())
                self.wfile.write(b"data: [DONE]\n\n")
            else:
                self.send_header("Content-Type", "application/json")
                self.end_headers()
                self.wfile.write(json.dumps({"id": "mock", "object": "chat.completion", "created": 1,
                    "model": MODEL, "choices": [{"index": 0, "message": message, "finish_reason": reason}],
                    "usage": usage}).encode())
    return ThreadingHTTPServer(("127.0.0.1", 0), Handler)


def run(binary, exporter, source, mode):
    fixture = Fixture(mode)
    api = server(fixture)
    thread = threading.Thread(target=api.serve_forever, daemon=True)
    thread.start()
    try:
        with tempfile.TemporaryDirectory(prefix="nutmix-mock-") as directory:
            root = Path(directory)
            (root / "bin").mkdir()
            trusted = root / "trusted"
            skill = trusted / ".agents/skills/nutmix"
            skill.mkdir(parents=True)
            integration = Path(__file__).resolve().parent
            (skill / "SKILL.md").write_text((integration / "skills/nutmix/SKILL.md").read_text())
            config = (integration / "fiach.toml").read_text().replace(
                "/etc/nutmix-review/personas/nutmix.md", str(integration / "personas/nutmix.md"))
            (root / "fiach.toml").write_text(config)
            output = root / "output"
            output.mkdir()
            head = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=source, text=True).strip()
            gh = root / "bin/gh"
            gh.write_text("""#!/bin/sh
set -eu
case "$1 $2" in
 'repo clone') exec git clone --quiet "$FIXTURE_SOURCE" "$4" ;;
 'pr checkout') exit 0 ;;
 'pr view') printf '{"baseRefOid":"%s","baseRefName":"master","state":"OPEN","mergedAt":null}' "$FIXTURE_HEAD" ;;
 'repo view') printf '{"defaultBranchRef":{"name":"master"}}' ;;
 'api --paginate') printf '[[{"id":1,"body":"Synthetic integration fixture already reported","user":{"login":"fixture-user"}}]]' ;;
 *) exit 1 ;;
esac
""")
            gh.chmod(0o755)
            # Only the named local fixtures are supplied, never developer secrets.
            env = {key: value for key, value in os.environ.items() if key in ("PATH", "LD_LIBRARY_PATH", "LANG")}
            env.update({"PATH": str(root / "bin") + ":" + os.environ["PATH"], "HOME": str(root),
                "XDG_CONFIG_HOME": str(root / "config"), "XDG_STATE_HOME": str(root / "state"),
                "GOOSE_PATH_ROOT": str(root / "goose"), "GOOSE_DISABLE_SESSION_NAMING": "true",
                "CONTEXT_FILE_NAMES": "[]", "GOOSE_MAX_TOKENS": "8192", "GOOSE_CONTEXT_LIMIT": "1048576",
                "OPENAI_API_KEY": "sk-mock-only", "OPENAI_BASE_URL": f"http://127.0.0.1:{api.server_port}/v1",
                "OPENAI_BASE_PATH": "v1/chat/completions", "NUTMIX_CI": "1",
                "NUTMIX_EXPECTED_HEAD": head, "NUTMIX_EXPECTED_BASE": head,
                "NUTMIX_INPUT_PRICE": "3.165", "NUTMIX_OUTPUT_PRICE": "15.825",
                "FIXTURE_SOURCE": str(source), "FIXTURE_HEAD": head, "RUST_LOG": "error"})
            # Fiach discovers trusted skills from cwd, not the cloned PR.
            command = [str(binary), "--config", str(root / "fiach.toml"), "review", "--repo", "lescuer97/nutmix", "--pr", "42",
                "--sandbox-child", "--result-json", str(output / "result.json"),
                "--output", str(output / "report.md"), "--db-path", str(root / "state.redb"),
                "--dedupe-existing-comments", "true" if mode == "dedupe" else "false",
                "--max-turns", "12", "--timeout-mins", "1", "--max-cost",
                "0.0000001" if mode == "budget" else "5"]
            result = subprocess.run(command, cwd=trusted, env=env, capture_output=True, timeout=180)
            manifest = {"repository": "lescuer97/nutmix", "fingerprint": "mock-only",
                        "snapshot": {"number": 42, "head": head, "base": head}}
            (root / "manifest.json").write_text(json.dumps(manifest))
            (root / "diff.txt").write_text("")
            exported = subprocess.run([str(exporter), str(output), str(root / "manifest.json"), str(root / "diff.txt")],
                                      capture_output=True, timeout=30)
            if mode in ("budget", "provider-error"):
                assert exported.returncode != 0, "A failed/budget-stopped review became publishable"
                if result.returncode == 0:
                    metadata = json.loads((output / "result.json").read_text())
                    assert metadata["metadata"]["status"] == "unverified", metadata
                print(f"Real Fiach mock-provider fixture passed: {mode} ({fixture.requests} requests; no clean result)")
                return
            assert result.returncode == 0, result.stderr.decode(errors="replace")[-6000:]
            metadata = json.loads((output / "result.json").read_text())["metadata"]
            assert metadata["cost_usd"] > 0, "PPQ price overrides were not accounted for"
            expected_cost = fixture.requests * (100 * 3.165 + 50 * 15.825) / 1_000_000
            assert math.isclose(metadata["cost_usd"], expected_cost, rel_tol=0.001), (metadata["cost_usd"], expected_cost)
            assert {"coordinator", "lane"} <= fixture.stages, fixture.stages
            if mode == "incomplete":
                assert "verifier" in fixture.stages and exported.returncode != 0
            else:
                assert exported.returncode == 0, exported.stderr.decode()
                report = json.loads(exported.stdout)
                assert len(report["findings"]) == (0 if mode in ("clean", "dedupe") else 1), report
                assert ("verifier" in fixture.stages) == (mode != "clean")
                assert ("dedupe" in fixture.stages) == (mode == "dedupe")
            print(f"Real Fiach mock-provider fixture passed: {mode} ({fixture.requests} requests)")
    finally:
        api.shutdown()
        api.server_close()
        thread.join()


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--binary", type=Path, required=True)
    parser.add_argument("--exporter", type=Path, required=True)
    parser.add_argument("--source", type=Path, required=True)
    args = parser.parse_args()
    for mode in ("confirmed", "clean", "incomplete", "budget", "provider-error", "dedupe"):
        run(args.binary.resolve(), args.exporter.resolve(), args.source.resolve(), mode)
