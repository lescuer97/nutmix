//! Tests added to the upstream build without introducing dependencies.
fn main() {}

#[cfg(test)]
mod tests {
    use std::{collections::HashMap, fs, os::unix::fs::PermissionsExt, process::Command};

    use fiach::{
        disclose,
        persona::PersonaSource,
        review::{ReviewExecution, ReviewParams},
    };

    #[tokio::test]
    async fn revision_guard_stops_before_provider_creation() {
        let root = tempfile::tempdir().unwrap();
        // Reuse the existing pinned source commit; tests never create commits.
        let repository = std::env::current_dir().unwrap();
        let head = String::from_utf8(
            Command::new("git")
                .args(["rev-parse", "HEAD"])
                .current_dir(&repository)
                .output()
                .unwrap()
                .stdout,
        )
        .unwrap()
        .trim()
        .to_string();
        let bin = root.path().join("bin");
        fs::create_dir(&bin).unwrap();
        let gh = bin.join("gh");
        fs::write(&gh, r#"#!/bin/sh
set -eu
case "$1 $2" in
  'repo clone') exec git clone --quiet "$FIXTURE_REPO" "$4" ;;
  'pr checkout') exit 0 ;;
  'pr view') printf '{"baseRefOid":"%s","baseRefName":"master","state":"OPEN","mergedAt":null}' "$FIXTURE_HEAD" ;;
  'repo view') printf '{"defaultBranchRef":{"name":"master"}}' ;;
  *) exit 1 ;;
esac
"#).unwrap();
        fs::set_permissions(&gh, fs::Permissions::from_mode(0o755)).unwrap();
        // One test owns this process's environment; no provider/server is started.
        unsafe {
            std::env::set_var(
                "PATH",
                format!("{}:{}", bin.display(), std::env::var("PATH").unwrap()),
            );
            std::env::set_var("FIXTURE_REPO", &repository);
            std::env::set_var("FIXTURE_HEAD", &head);
            std::env::set_var("NUTMIX_CI", "1");
            std::env::set_var("NUTMIX_EXPECTED_HEAD", "a".repeat(40));
            std::env::set_var("NUTMIX_EXPECTED_BASE", &head);
            std::env::set_var("CONTEXT_FILE_NAMES", "[]");
            std::env::set_var("GOOSE_PATH_ROOT", root.path().join("goose"));
        }
        assert!(goose::hints::get_context_filenames().is_empty());
        let params = || ReviewParams {
            repo: "lescuer97/nutmix".into(),
            pr_number: 42,
            model: "moonshotai/kimi-k3".into(),
            provider: "intentionally-invalid-provider".into(),
            verifier_model: None,
            verifier_provider: None,
            dedupe_existing_comments: false,
            output: None,
            skill: None,
            persona: PersonaSource::BuiltinPrReview,
            review_lanes: vec![],
            review_lane_prompts: HashMap::new(),
            review_lane_conditions: HashMap::new(),
            max_review_lanes: 1,
            max_turns: 1,
            timeout_mins: 1,
            db_path: root.path().join("state"),
            review_kind: "test".into(),
            force: true,
            max_retries: 0,
            retry_delay_secs: 1,
            disclose_config: disclose::DiscloseConfig {
                mode: disclose::ReportMode::Local,
                sync_repo: None,
                notify_on_empty: false,
                reactions: disclose::ReactionConfig {
                    review_start: None,
                    no_findings: None,
                },
            },
            buzz_config: None,
            verify_findings: true,
            context_groups: HashMap::new(),
            max_cost_usd: Some(1.0),
            input_price_per_m: None,
            output_price_per_m: None,
            is_rereview: false,
            trigger_mention_node_id: None,
            execution: ReviewExecution {
                skip_state_check: true,
                persist_side_effects: false,
                result_json: None,
            },
        };
        let error = fiach::review::run_review(params(), tokio_util::sync::CancellationToken::new())
            .await
            .unwrap_err();
        assert!(
            error
                .to_string()
                .contains("PR changed since CI authorization"),
            "{error:#}"
        );
        unsafe {
            std::env::set_var("NUTMIX_EXPECTED_HEAD", &head);
            std::env::set_var("NUTMIX_EXPECTED_BASE", "b".repeat(40));
        }
        let error = fiach::review::run_review(params(), tokio_util::sync::CancellationToken::new())
            .await
            .unwrap_err();
        assert!(
            error
                .to_string()
                .contains("PR changed since CI authorization"),
            "{error:#}"
        );
        unsafe {
            std::env::remove_var("NUTMIX_EXPECTED_HEAD");
        }
        let error = fiach::review::run_review(params(), tokio_util::sync::CancellationToken::new())
            .await
            .unwrap_err();
        assert!(
            error.to_string().contains("Missing CI expected head"),
            "{error:#}"
        );
    }
}
