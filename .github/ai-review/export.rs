//! Offline host adapter. Finding acceptance is Fiach's, not a second reviewer.
use std::{fs, io::Read, path::Path};

use anyhow::{Context, Result, bail, ensure};
use fiach::{reporting, review::CompletedReview};
use serde_json::{Value, json};

const LIMIT: u64 = 8 * 1024 * 1024;

fn read(path: &Path) -> Result<Vec<u8>> {
    let metadata = fs::symlink_metadata(path).context("Missing fixed artifact")?;
    ensure!(
        metadata.is_file() && !metadata.file_type().is_symlink(),
        "Not a regular artifact"
    );
    ensure!(metadata.len() <= LIMIT, "Oversized artifact");
    let mut bytes = Vec::new();
    fs::File::open(path)?
        .take(LIMIT + 1)
        .read_to_end(&mut bytes)?;
    ensure!(bytes.len() as u64 <= LIMIT, "Artifact grew beyond limit");
    Ok(bytes)
}

fn export(root: &Path, manifest: &Value, diff: &str) -> Result<Value> {
    let completed: CompletedReview = serde_json::from_slice(&read(&root.join("result.json"))?)?;
    let mut artifact: reporting::ReportingArtifact =
        serde_json::from_slice(&read(&root.join("report.structured.json"))?)?;
    let mut policy: reporting::DisclosurePolicy =
        serde_json::from_slice(&read(&root.join("report.policy.json"))?)?;
    reporting::validate_artifact(&mut artifact)?;
    let snap = &manifest["snapshot"];
    ensure!(
        manifest["repository"] == "lescuer97/nutmix",
        "Wrong repository"
    );
    ensure!(
        completed.metadata.repository == "lescuer97/nutmix"
            && Some(completed.metadata.pr_number) == snap["number"].as_u64()
            && Some(completed.metadata.commit_hash.as_str()) == snap["head"].as_str()
            && completed.metadata.model == "moonshotai/kimi-k3",
        "Mismatched review identity"
    );
    ensure!(
        matches!(
            completed.metadata.status,
            fiach::state::ReviewStatus::None
                | fiach::state::ReviewStatus::Rejected
                | fiach::state::ReviewStatus::AlreadyReported
                | fiach::state::ReviewStatus::Confirmed
        ),
        "Worker did not record a completed review"
    );
    ensure!(
        Some(policy.pr_context.head_commit.as_str()) == snap["head"].as_str()
            && Some(policy.pr_context.base_commit.as_str()) == snap["base"].as_str()
            && policy.pr_context.comments_allowed(),
        "Mismatched PR context"
    );
    ensure!(
        artifact.finder_complete() && !artifact.is_incomplete() && !artifact.markdown_only_fallback,
        "Incomplete review; not a clean result"
    );
    ensure!(
        artifact.findings.len() <= 64 && artifact.verdicts.len() <= 64,
        "Too many findings"
    );
    if !artifact.findings.is_empty() {
        ensure!(artifact.verifier_complete(), "Verifier did not finish");
        for finding in &artifact.findings {
            ensure!(
                artifact
                    .verdicts
                    .iter()
                    .filter(|v| v.finding_id == finding.id)
                    .count()
                    == 1,
                "Every candidate needs exactly one verdict"
            );
        }
    }
    // Never trust model-writable anchors. This patch was obtained independently
    // by the host at the authorized immutable revisions. Missing anchors are
    // downgraded to summary locations by upstream AcceptedFinding policy.
    policy.diff_anchors = reporting::parse_diff_anchors(diff);
    let findings = artifact.publishable_findings(&policy);
    for finding in &findings {
        ensure!(
            finding.title.len() <= 300 && finding.body.len() <= 6000,
            "Finding exceeds publication limits"
        );
        for location in &finding.unanchored_locations {
            let path = Path::new(&location.path);
            ensure!(
                !path.is_absolute()
                    && !path
                        .components()
                        .any(|part| matches!(part, std::path::Component::ParentDir)),
                "Unsafe location path"
            );
        }
    }
    Ok(
        json!({"schema": 1, "snapshot": snap, "fingerprint": manifest["fingerprint"],
        "status": if findings.is_empty() { "no-publishable-findings" } else { "confirmed" },
        "findings": findings}),
    )
}

fn main() -> Result<()> {
    let args: Vec<_> = std::env::args().collect();
    if args.len() != 4 {
        bail!("Expected artifact directory, manifest and trusted diff");
    }
    let manifest: Value = serde_json::from_slice(&read(Path::new(&args[2]))?)?;
    let diff = String::from_utf8(read(Path::new(&args[3]))?)?;
    let value = export(Path::new(&args[1]), &manifest, &diff)?;
    println!("{}", serde_json::to_string(&value)?);
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    fn fixture() -> (tempfile::TempDir, Value) {
        let dir = tempfile::tempdir().unwrap();
        let head = "a".repeat(40);
        let base = "b".repeat(40);
        let manifest = json!({"repository":"lescuer97/nutmix", "fingerprint":"test",
            "snapshot":{"number":42,"head":head,"base":base}});
        let metadata = json!({"repository":"lescuer97/nutmix", "pr_number":42,
            "commit_hash":head,"model":"moonshotai/kimi-k3","timestamp":0,
            "findings_count":0,"status":"none","severity":"none",
            "pr_classification":"none","retry_count":0});
        fs::write(
            dir.path().join("result.json"),
            json!({"metadata":metadata,
            "should_notify":false,"report_path":"/do/not/follow"})
            .to_string(),
        )
        .unwrap();
        fs::write(
            dir.path().join("report.policy.json"),
            json!({"pr_context":{
            "state":"OPEN","merged":false,"base_ref_name":"master","default_branch":"master",
            "base_commit":base,"head_commit":head},"diff_anchors":{}})
            .to_string(),
        )
        .unwrap();
        artifact(
            dir.path(),
            json!({"no_findings":{"summary":"No findings","skills_used":["nutmix"]}}),
        );
        (dir, manifest)
    }

    fn artifact(dir: &Path, value: Value) {
        fs::write(dir.join("report.structured.json"), value.to_string()).unwrap();
    }

    fn candidate() -> Value {
        json!({"id":"F-1","title":"Bug","severity":"high","confidence":"high",
            "affected_locations":[{"path":"mint.go","start_line":99}],
            "evidence":"Observed path","skills_used":["nutmix"],"body_markdown":"Failure path"})
    }

    fn verdict(transcripts: bool) -> Value {
        json!({"finding_id":"F-1","confirmed":true,"introduced_by_pr":true,
            "present_on_pr_branch":true,"present_on_base":false,"present_on_default_branch":false,
            "disclosure_decision":"disclose","rationale":"Concrete change",
            "command_transcripts":if transcripts {vec![json!({"command":"git show", "branch_or_commit":"head",
                "key_output":"source", "interpretation":"Inspection, not reproduction"})]}else{vec![]}})
    }

    #[test]
    fn complete_no_findings_uses_fixed_paths() {
        let (dir, manifest) = fixture();
        assert_eq!(
            export(dir.path(), &manifest, "").unwrap()["status"],
            "no-publishable-findings"
        );
    }

    #[test]
    fn incomplete_budget_and_markdown_are_not_clean() {
        let (dir, manifest) = fixture();
        for extra in [
            json!({"verifier_failed":true}),
            json!({"budget_exhausted":"finder"}),
            json!({"markdown_only_fallback":true}),
        ] {
            let mut value = json!({"no_findings":{"summary":"none"}});
            value
                .as_object_mut()
                .unwrap()
                .extend(extra.as_object().unwrap().clone());
            artifact(dir.path(), value);
            assert!(export(dir.path(), &manifest, "").is_err());
        }
    }

    #[test]
    fn incomplete_worker_metadata_cannot_be_overridden_by_clean_artifact() {
        let (dir, manifest) = fixture();
        let path = dir.path().join("result.json");
        let mut result: Value = serde_json::from_slice(&fs::read(&path).unwrap()).unwrap();
        result["metadata"]["status"] = json!("unverified");
        fs::write(&path, result.to_string()).unwrap();
        assert!(export(dir.path(), &manifest, "").is_err());
    }

    #[test]
    fn missing_unknown_or_duplicate_verdicts_fail() {
        let (dir, manifest) = fixture();
        let mut unknown = verdict(true);
        unknown["finding_id"] = json!("unknown");
        for verdicts in [vec![], vec![unknown], vec![verdict(true), verdict(true)]] {
            artifact(
                dir.path(),
                json!({"findings":[candidate()],"verdicts":verdicts}),
            );
            assert!(export(dir.path(), &manifest, "").is_err());
        }
    }

    #[test]
    fn upstream_requires_transcripts_and_preserves_unanchored_findings() {
        let (dir, manifest) = fixture();
        artifact(
            dir.path(),
            json!({"findings":[candidate()],"verdicts":[verdict(false)]}),
        );
        assert!(
            export(dir.path(), &manifest, "").unwrap()["findings"]
                .as_array()
                .unwrap()
                .is_empty()
        );
        artifact(
            dir.path(),
            json!({"findings":[candidate()],"verdicts":[verdict(true)]}),
        );
        let value = export(dir.path(), &manifest, "").unwrap();
        assert_eq!(
            value["findings"][0]["unanchored_locations"][0]["start_line"],
            99
        );
        assert!(
            value["findings"][0]["inline_comments"]
                .as_array()
                .unwrap()
                .is_empty()
        );
    }

    #[test]
    fn forged_identity_or_symlink_rejected() {
        let (dir, mut manifest) = fixture();
        manifest["snapshot"]["head"] = json!("c".repeat(40));
        assert!(export(dir.path(), &manifest, "").is_err());
        fs::remove_file(dir.path().join("report.structured.json")).unwrap();
        std::os::unix::fs::symlink("/etc/passwd", dir.path().join("report.structured.json"))
            .unwrap();
        assert!(export(dir.path(), &manifest, "").is_err());
    }
}
