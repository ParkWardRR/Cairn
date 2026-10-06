#!/usr/bin/env bash
# Creates one empty PUBLIC component repository with the project's standard settings.
#
#   split/publish/create-repo.sh <name> "<description>" <topic>...
#
# Standard settings: no wiki or projects, branches deleted on merge, secret scanning and
# push protection on, workflows read-only by default and unable to approve pull requests,
# approval required for every external contributor's workflow run, and a ruleset that
# blocks force-push and deletion of main. Pull requests are NOT required: the repository
# has one maintainer, and a required check would block their own pushes.
#
# Needs an authenticated `gh` with repository administration. Public means public: run
# it only when the repository is meant to be.
set -euo pipefail

name="${1:?usage: create-repo.sh <name> <description> <topic>...}"
desc="${2:?description}"
shift 2
owner="${CAIRN_OWNER:-ParkWardRR}"
repo="$owner/$name"

gh repo create "$repo" --public --disable-wiki --description "$desc" >/dev/null
for t in "$@"; do gh repo edit "$repo" --add-topic "$t" >/dev/null; done

gh api -X PATCH "repos/$repo" -F delete_branch_on_merge=true -F has_projects=false \
  -f 'security_and_analysis[secret_scanning][status]=enabled' \
  -f 'security_and_analysis[secret_scanning_push_protection][status]=enabled' >/dev/null
gh api -X PUT "repos/$repo/actions/permissions/workflow" \
  -f default_workflow_permissions=read -F can_approve_pull_request_reviews=false >/dev/null
gh api -X PUT "repos/$repo/actions/permissions/fork-pr-contributor-approval" \
  -f approval_policy=all_external_contributors >/dev/null

gh api -X POST "repos/$repo/rulesets" --input - >/dev/null <<'JSON'
{
  "name": "protect main",
  "target": "branch",
  "enforcement": "active",
  "conditions": { "ref_name": { "include": ["refs/heads/main"], "exclude": [] } },
  "rules": [ { "type": "deletion" }, { "type": "non_fast_forward" } ]
}
JSON
echo "created https://github.com/$repo"
