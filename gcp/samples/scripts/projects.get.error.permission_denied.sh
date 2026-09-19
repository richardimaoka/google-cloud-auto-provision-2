#!/usr/bin/env bash
# Get a project owned by someone else, that ACCOUNT cannot read.
# PROJECT is its project number or ID.
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
ask PROJECT "Number or ID of a project ACCOUNT cannot access" '^([0-9]+|[a-z][a-z0-9-]{4,29})$'

curl -sS -X GET \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "x-goog-user-project: ${QUOTA_PROJECT}" \
  -o "${OUT}" -w 'HTTP %{http_code}\n' \
  "https://cloudresourcemanager.googleapis.com/v3/projects/${PROJECT}"

echo "saved ${OUT}" >&2
