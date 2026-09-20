#!/usr/bin/env bash
# Get the IAM policy of a project that has a conditional binding. PROJECT is its
# number or ID. Add the binding first, with a role that is not a basic role, and
# a condition that holds nothing real, then remove it after recording:
#
#   gcloud projects add-iam-policy-binding PROJECT --member=user:someone@example.com \
#     --role=roles/bigquery.jobUser \
#     --condition='title=sample,expression=request.time < timestamp("2030-01-01T00:00:00Z")'
#
# The request asks for policy version 3. Without it, the condition is left out and
# the role name gets a "_withcond_<hash>" suffix.
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
ask PROJECT "Number or ID of a project with a conditional binding" '^([0-9]+|[a-z][a-z0-9-]{4,29})$'

curl -sS -X POST \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "x-goog-user-project: ${QUOTA_PROJECT}" \
  -H "Content-Type: application/json" \
  -d '{"options": {"requestedPolicyVersion": 3}}' \
  -o "${OUT}" -w 'HTTP %{http_code}\n' \
  "https://cloudresourcemanager.googleapis.com/v3/projects/${PROJECT}:getIamPolicy"

if ! grep -q '"condition"' "${OUT}"; then
  echo "warning: the policy has no condition, so this is not a with_condition sample" >&2
fi
echo "saved ${OUT}" >&2
