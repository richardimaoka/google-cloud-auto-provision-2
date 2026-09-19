#!/usr/bin/env bash
# Get the IAM policy of a project that has no conditional binding. PROJECT is its
# number or ID. The request asks for policy version 3, like the client does.
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
ask PROJECT "Number or ID of a project with no conditional binding" '^([0-9]+|[a-z][a-z0-9-]{4,29})$'

curl -sS -X POST \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "x-goog-user-project: ${QUOTA_PROJECT}" \
  -H "Content-Type: application/json" \
  -d '{"options": {"requestedPolicyVersion": 3}}' \
  -o "${OUT}" -w 'HTTP %{http_code}\n' \
  "https://cloudresourcemanager.googleapis.com/v3/projects/${PROJECT}:getIamPolicy"

if grep -q '"condition"' "${OUT}"; then
  echo "warning: the policy has a condition, so this is not a no_condition sample" >&2
fi
echo "saved ${OUT}" >&2
