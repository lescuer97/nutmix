import copy
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import tomllib
import unittest
from unittest.mock import Mock, patch
from urllib.error import HTTPError

import ai_review as review


IMAGE = "sha256:" + "d" * 64


def pull_request():
    return {"number": 42, "state": "open", "draft": False, "mergeable": True,
            "merge_commit_sha": "c" * 40, "head": {"sha": "a" * 40, "repo": {"full_name": "contributor/nutmix"}},
            "base": {"sha": "b" * 40}}


def manifest():
    return {"schema": 1, "repository": review.REPOSITORY, "model": review.MODEL,
            "snapshot": review.snapshot(pull_request()), "fingerprint": "trusted", "run_id": "123",
            "comment_id": 7, "pricing": {"input": 3.165, "output": 15.825}}


def owner_env():
    return {"GITHUB_REPOSITORY": review.REPOSITORY, "GITHUB_ACTOR": review.OWNER,
            "REVIEW_TRIGGERING_ACTOR": review.OWNER, "GITHUB_RUN_ID": "123", "GITHUB_EVENT_PATH": "unused"}


class AuthorizationTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.path = Path(self.temp.name) / "event.json"
        self.event = {"action": "created", "repository": {"full_name": review.REPOSITORY},
                      "sender": {"login": review.OWNER}, "issue": {"number": 42, "pull_request": {"url": "unused"}},
                      "comment": {"id": 7, "body": "/ai-review", "user": {"login": review.OWNER}}}
        self.env = {**owner_env(), "GITHUB_EVENT_PATH": str(self.path)}
        self.github = Mock()
        self.github.get.return_value = {"body": "/ai-review", "user": {"login": review.OWNER},
            "issue_url": f"https://api.github.com/repos/{review.REPOSITORY}/issues/42"}

    def authorize(self):
        self.path.write_text(json.dumps(self.event))
        with patch.dict(os.environ, self.env, clear=True):
            return review.authorize(self.github)

    def test_owner_allowed(self):
        self.assertEqual(self.authorize(), 42)

    def test_other_actor_repository_or_rerun_blocked(self):
        for key in ("GITHUB_ACTOR", "REVIEW_TRIGGERING_ACTOR", "GITHUB_REPOSITORY"):
            old = self.env[key]
            self.env[key] = "outsider"
            with self.subTest(key=key), self.assertRaises(review.ReviewError):
                self.authorize()
            self.env[key] = old
        self.github.get.assert_not_called()

    def test_forged_sender_or_author_blocked(self):
        for target in (self.event["sender"], self.event["comment"]["user"]):
            target["login"] = "outsider"
            with self.assertRaises(review.ReviewError):
                self.authorize()
            target["login"] = review.OWNER

    def test_not_a_pr_or_exact_command_blocked(self):
        for body in ("please /ai-review", "/ai-review\n", "/ai-review; curl evil"):
            self.event["comment"]["body"] = body
            with self.assertRaises(review.ReviewError):
                self.authorize()
        self.event["comment"]["body"] = "/ai-review"
        self.event["issue"].pop("pull_request")
        with self.assertRaises(review.ReviewError):
            self.authorize()

    def test_edited_deleted_or_reassigned_request_blocked(self):
        self.github.get.return_value["body"] = "cancelled"
        with self.assertRaises(review.ReviewError):
            self.authorize()

    def test_unauthorized_worker_never_runs_docker(self):
        with patch.dict(os.environ, {**self.env, "REVIEW_TRIGGERING_ACTOR": "outsider"}, clear=True), \
                patch.object(review, "run_worker") as worker:
            with self.assertRaises(review.ReviewError):
                review.run(IMAGE)
            worker.assert_not_called()


class GateTests(unittest.TestCase):
    def setUp(self):
        self.pr = pull_request()
        self.github = Mock()
        self.runs, self.jobs = {}, {}
        self.checks, self.statuses = [], []
        for index, (workflow, required) in enumerate(review.REQUIRED_JOBS.items(), 1):
            self.runs[workflow] = [{"id": index, "status": "completed", "conclusion": "success",
                                    "run_attempt": 1, "pull_requests": [copy.deepcopy(self.pr)]}]
            self.jobs[index] = [{"name": name, "status": "completed", "conclusion": "success"} for name in required]
        self.github.pages.side_effect = self.pages

    def pages(self, path, key=None):
        if key == "workflow_runs":
            workflow = path.split("/workflows/")[1].split("/")[0]
            return self.runs[workflow] if self.pr["head"]["sha"] in path else []
        if key == "jobs":
            return self.jobs[int(path.split("/runs/")[1].split("/")[0])]
        if key == "check_runs":
            return self.checks
        if path.endswith("/statuses"):
            return self.statuses
        self.fail(f"Unexpected API call: {path}")

    def test_success_records_tested_run_receipts(self):
        self.assertEqual(len(review.check_ci(self.github, self.pr)), 3)

    def test_missing_workflow_blocks(self):
        self.runs["golangci-lint.yml"] = []
        with self.assertRaises(review.ReviewError):
            review.check_ci(self.github, self.pr)

    def test_all_unsuccessful_and_pending_states_block(self):
        run = self.runs["workflow.yml"][0]
        for conclusion in (None, "failure", "cancelled", "skipped", "neutral", "timed_out"):
            run["conclusion"] = conclusion
            with self.subTest(conclusion=conclusion), self.assertRaises(review.ReviewError):
                review.check_ci(self.github, self.pr)
        run.update(conclusion="success", status="in_progress")
        with self.assertRaises(review.ReviewError):
            review.check_ci(self.github, self.pr)

    def test_latest_failed_run_not_older_success(self):
        latest = copy.deepcopy(self.runs["workflow.yml"][0])
        latest.update(id=99, conclusion="failure")
        self.runs["workflow.yml"].append(latest)
        with self.assertRaises(review.ReviewError):
            review.check_ci(self.github, self.pr)

    def test_wrong_pr_or_stale_base_head_block(self):
        association = self.runs["workflow.yml"][0]["pull_requests"][0]
        for field in ("number", "head", "base"):
            old = association[field]
            association[field] = 99 if field == "number" else {"sha": "old"}
            with self.subTest(field=field), self.assertRaises(review.ReviewError):
                review.check_ci(self.github, self.pr)
            association[field] = old

    def test_fork_allowed_only_with_verifiable_run_association(self):
        review.check_ci(self.github, self.pr)
        self.runs["workflow.yml"][0]["pull_requests"] = []
        with self.assertRaises(review.ReviewError):
            review.check_ci(self.github, self.pr)

    def test_removed_matrix_leg_or_reviewer_test_blocks(self):
        original = self.jobs[1]
        for name in ("test (internal/mint)", "ai-review-image-tests"):
            self.jobs[1] = [job for job in original if job["name"] != name]
            with self.subTest(name=name), self.assertRaises(review.ReviewError):
                review.check_ci(self.github, self.pr)

    def test_skipped_job_in_successful_run_blocks(self):
        self.jobs[1][0]["conclusion"] = "skipped"
        with self.assertRaises(review.ReviewError):
            review.check_ci(self.github, self.pr)

    def test_additional_checks_and_statuses_block(self):
        self.checks = [{"name": "external", "status": "queued", "conclusion": None}]
        with self.assertRaises(review.ReviewError):
            review.check_ci(self.github, self.pr)
        self.checks = []
        self.statuses = [{"context": "external", "state": "pending"}]
        with self.assertRaises(review.ReviewError):
            review.check_ci(self.github, self.pr)

    def test_required_external_context_must_exist(self):
        with patch.object(review, "REQUIRED_CONTEXTS", {"external"}):
            with self.assertRaises(review.ReviewError):
                review.check_ci(self.github, self.pr)

    def test_latest_commit_status_wins(self):
        self.statuses = [{"context": "external", "state": "success"}, {"context": "external", "state": "failure"}]
        review.check_ci(self.github, self.pr)

    def test_draft_closed_unknown_and_conflicted_prs_block(self):
        for field, value in (("draft", True), ("state", "closed"), ("mergeable", None), ("mergeable", False)):
            pr = pull_request()
            pr[field] = value
            self.github.get.return_value = pr
            with self.subTest(field=field, value=value), self.assertRaises(review.ReviewError):
                review.pull(self.github, 42)


class ManifestTests(unittest.TestCase):
    def test_fiach_configuration_is_explicit_and_verification_mandatory(self):
        config = tomllib.loads((review.TOOL / "fiach.toml").read_text())["review"]
        self.assertEqual(config["provider"], "openai")
        self.assertEqual(config["verifier_provider"], "openai")
        self.assertEqual(config["model"], review.MODEL)
        self.assertEqual(config["verifier_model"], review.MODEL)
        self.assertIs(config["verify_findings"], True)
        self.assertEqual(config.get("review_lane_conditions", {}), {})

    def test_reproduction_toolchain_tracks_go_mod(self):
        version = re.search(r"^go (\S+)$", (review.ROOT / "go.mod").read_text(), re.MULTILINE)[1]
        self.assertIn("FROM golang:" + version + "-bookworm@sha256:", (review.TOOL / "Dockerfile").read_text())

    def test_valid_manifest_and_invalid_commit(self):
        data = manifest()
        with patch.dict(os.environ, owner_env()), patch.object(review, "read_json", return_value=data), \
                patch.object(Path, "read_text", return_value='{"comment":{"id":7}}'), \
                patch.object(review, "fingerprint", return_value="trusted"):
            self.assertEqual(review.read_manifest(), data)
            data["snapshot"]["head"] = "HEAD; echo unsafe"
            with self.assertRaises(review.ReviewError):
                review.read_manifest()

    def test_provenance_and_commits_are_enforced(self):
        valid = manifest()
        for key, bad in (("run_id", "other"), ("comment_id", 99), ("fingerprint", "changed"),
                         ("model", "other"), ("repository", "fork/repo")):
            data = {**valid, key: bad}
            with patch.dict(os.environ, owner_env()), patch.object(review, "read_json", return_value=data), \
                    patch.object(Path, "read_text", return_value='{"comment":{"id":7}}'), \
                    patch.object(review, "fingerprint", return_value="trusted"):
                with self.subTest(key=key), self.assertRaises(review.ReviewError):
                    review.read_manifest()

    def test_changed_revision_blocks_after_approval(self):
        changed = pull_request()
        changed["base"]["sha"] = "updated"
        with patch.object(review, "authorize", return_value=42), \
                patch.object(review, "read_manifest", return_value=manifest()), \
                patch.object(review, "pull", return_value=changed), patch.object(review, "check_ci") as ci:
            with self.assertRaises(review.ReviewError):
                review.check(Mock())
            ci.assert_not_called()

    def test_ppq_model_availability_and_pricing_required(self):
        for data in ({"data": []}, {"data": [{"id": review.MODEL, "pricing": {
                "input_per_1M_tokens": float("nan"), "output_per_1M_tokens": 1}}]}):
            with patch.object(review, "request_json", return_value=data), self.assertRaises(review.ReviewError):
                review.pricing()
        response = {"data": [{"id": review.MODEL, "pricing": {"input_per_1M_tokens": 3.165, "output_per_1M_tokens": 15.825}}]}
        with patch.object(review, "request_json", return_value=response) as request:
            self.assertEqual(review.pricing(), manifest()["pricing"])
            self.assertEqual(request.call_args.args, ("https://api.ppq.ai/v1/models",))


class SandboxTests(unittest.TestCase):
    def test_output_mount_is_bounded_and_unmounted_after_failure(self):
        with tempfile.TemporaryDirectory() as directory, \
                patch.object(subprocess, "run", return_value=Mock(returncode=0)) as run:
            with self.assertRaises(review.ReviewError):
                with review.bounded_output(directory):
                    raise review.ReviewError("Worker failed")
        self.assertIn("size=32m,mode=0777,nosuid,nodev,noexec", run.call_args_list[0].args[0])
        self.assertEqual(run.call_args_list[-1].args[0][:3], ["sudo", "-n", "umount"])

    def test_non_pinned_image_ref_is_rejected(self):
        with self.assertRaises(review.ReviewError):
            review.sandbox("nutmix-fiach:latest")

    def test_worker_is_isolated_and_secrets_not_on_command_line(self):
        env = {"PPQ_API_KEY": "sk-ppq-secret", "GITHUB_TOKEN": "gh-read-only", "ACTIONS_RUNTIME_TOKEN": "never-forward"}
        with patch.dict(os.environ, env), patch.object(subprocess, "run", return_value=Mock(returncode=0)) as run:
            review.run_worker(IMAGE, manifest(), Path("/tmp/opencode/output"))
        command = run.call_args_list[0].args[0]
        self.assertNotIn("sk-ppq-secret", str(command))
        self.assertNotIn("gh-read-only", str(command))
        self.assertNotIn("ACTIONS_RUNTIME_TOKEN", command)
        self.assertNotIn("PPQ_API_KEY", command)
        for flag in ("--read-only", "--cap-drop=ALL", "--security-opt=no-new-privileges:true",
                     "--user=10001:10001", "--pids-limit=256", "--network=bridge", "--sandbox-child"):
            self.assertIn(flag, command)
        self.assertNotIn("docker.sock", str(command))
        child_env = run.call_args_list[0].kwargs["env"]
        self.assertEqual(child_env["OPENAI_BASE_URL"], "https://api.ppq.ai/v1")
        self.assertEqual(child_env["CONTEXT_FILE_NAMES"], "[]")
        self.assertEqual(child_env["NUTMIX_EXPECTED_HEAD"], "a" * 40)
        self.assertEqual(run.call_args_list[-1].args[0][:3], ["docker", "rm", "--force"])

    def test_timeout_also_kills_container(self):
        with patch.dict(os.environ, {"PPQ_API_KEY": "key", "GITHUB_TOKEN": "token"}), \
                patch.object(subprocess, "run", side_effect=[subprocess.TimeoutExpired("docker", 1), Mock()]) as run:
            with self.assertRaises(subprocess.TimeoutExpired):
                review.run_worker(IMAGE, manifest(), Path("/tmp/opencode/output"))
        self.assertEqual(run.call_args_list[-1].args[0][:3], ["docker", "rm", "--force"])

    def test_missing_secret_prevents_container_start(self):
        with patch.dict(os.environ, {}, clear=True), patch.object(subprocess, "run") as run:
            with self.assertRaises(review.ReviewError):
                review.run_worker(IMAGE, manifest(), Path("/tmp/opencode/output"))
            run.assert_not_called()


class ArtifactTests(unittest.TestCase):
    def test_exporter_is_offline_and_uses_trusted_image_not_worker_executable(self):
        data = manifest()
        exported = {"schema": 1, "snapshot": data["snapshot"], "fingerprint": data["fingerprint"], "findings": []}
        github = Mock()
        github.diff.return_value = "diff --git a/mint.go b/mint.go\n"

        def invoke(command, *, stdout, **kwargs):
            self.assertIn("--network=none", command)
            self.assertIn("--entrypoint", command)
            self.assertIn("nutmix-export", command)
            self.assertIn(IMAGE, command)
            self.assertNotIn("--env", command)
            stdout.write(json.dumps(exported).encode())
            return Mock(returncode=0)

        with patch.object(review, "read_regular", return_value=b"{}"), patch.object(subprocess, "run", side_effect=invoke):
            self.assertEqual(review.export_result(IMAGE, data, github), exported)

    def test_regular_file_only_with_no_links_or_oversize(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "file").write_text("{}")
            self.assertEqual(review.read_json(root / "file"), {})
            (root / "symlink").symlink_to(root / "file")
            os.link(root / "file", root / "hardlink")
            for path in (root / "symlink", root / "hardlink", root):
                with self.subTest(path=path), self.assertRaises((review.ReviewError, OSError)):
                    review.read_regular(path)
            (root / "large").write_bytes(b"x" * (review.MAX_RESPONSE + 1))
            with self.assertRaises(review.ReviewError):
                review.read_regular(root / "large")

    def test_publisher_is_commit_bound_never_approval(self):
        data = manifest()
        exported = {"findings": []}
        github = Mock()
        github.pages.return_value = []
        with patch.object(review, "check", return_value=data) as check, \
                patch.object(review, "export_result", return_value=exported):
            review.publish(github, IMAGE)
        self.assertEqual(check.call_count, 2)
        self.assertEqual(github.post.call_args.args[1]["event"], "COMMENT")
        self.assertEqual(github.post.call_args.args[1]["commit_id"], data["snapshot"]["head"])

    def test_export_failure_or_final_revision_change_prevents_publication(self):
        github = Mock()
        github.pages.return_value = []
        with patch.object(review, "check", return_value=manifest()), \
                patch.object(review, "export_result", side_effect=review.ReviewError("Incomplete")):
            with self.assertRaises(review.ReviewError):
                review.publish(github, IMAGE)
        with patch.object(review, "check", side_effect=[manifest(), review.ReviewError("Changed")]), \
                patch.object(review, "export_result", return_value={"findings": []}):
            with self.assertRaises(review.ReviewError):
                review.publish(github, IMAGE)
        github.post.assert_not_called()

    def test_duplicate_report_not_posted(self):
        data = manifest()
        body, _ = review.report(data, {"findings": []})
        github = Mock()
        github.pages.return_value = [{"user": {"login": "github-actions[bot]"}, "body": body}]
        with patch.object(review, "check", return_value=data), \
                patch.object(review, "export_result", return_value={"findings": []}):
            review.publish(github, IMAGE)
        github.post.assert_not_called()

    def test_model_text_is_not_executable_markup(self):
        finding = {"title": "@lescuer97 ![img](https://evil) <script>", "severity": "high",
                   "body": "www.evil.com", "inline_comments": [], "additional_locations": [],
                   "unanchored_locations": [{"path": "mint.go", "start_line": 99}]}
        body, _ = review.report(manifest(), {"findings": [finding]})
        for value in ("@lescuer97", "<script>", "https://evil", "www.evil"):
            self.assertNotIn(value, body)
        self.assertIn("mint.go:99", body)
        self.assertEqual(review.markdown("&#64;lescuer97"), "&amp;\\#64;lescuer97")


class TransportTests(unittest.TestCase):
    def test_redirects_not_followed(self):
        self.assertIsNone(review.NoRedirect().redirect_request(None, None, 302, "Found", {}, "https://evil"))

    def test_error_bodies_do_not_leak(self):
        with patch.object(review, "build_opener") as opener:
            opener.return_value.open.side_effect = HTTPError("https://example", 401, "secret", {}, None)
            with self.assertRaises(review.ReviewError) as error:
                review.request_json("https://example", "secret")
        self.assertNotIn("secret", str(error.exception))

    def test_size_and_invalid_json_fail_closed(self):
        for raw in (b"x" * (review.MAX_RESPONSE + 1), b"not-json"):
            with patch.object(review, "build_opener") as opener:
                opener.return_value.open.return_value.__enter__.return_value.read.return_value = raw
                with self.assertRaises(review.ReviewError):
                    review.request_json("https://example")

    def test_pagination_complete_or_explicitly_rejected(self):
        with patch.dict(os.environ, {"GITHUB_TOKEN": "token"}):
            github = review.GitHub()
        with patch.object(github, "get", side_effect=[[1] * 100, [2]]):
            self.assertEqual(len(github.pages("/test")), 101)
        with patch.object(github, "get", return_value=[1] * 100):
            with self.assertRaises(review.ReviewError):
                github.pages("/test")


if __name__ == "__main__":
    unittest.main()
