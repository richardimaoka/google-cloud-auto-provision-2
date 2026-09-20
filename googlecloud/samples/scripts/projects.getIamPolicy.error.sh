#!/usr/bin/env bash
# Get the IAM policy of a project that does not exist. The random suffix makes
# sure of that. The API answers 403, as it does for a project the caller cannot use.
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

curl -sS -X POST \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "x-goog-user-project: ${QUOTA_PROJECT}" \
  -H "Content-Type: application/json" \
  -d '{"options": {"requestedPolicyVersion": 3}}' \
  -o "${OUT}" -w 'HTTP %{http_code}\n' \
  "https://cloudresourcemanager.googleapis.com/v3/projects/this-does-not-exist-${RANDOM}${RANDOM}:getIamPolicy"

echo "saved ${OUT}" >&2
