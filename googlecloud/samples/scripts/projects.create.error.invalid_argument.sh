#!/usr/bin/env bash
# Create a project with a project ID that is not valid (upper case letters and
# underscores are not allowed), so the request is rejected and nothing is created.
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

curl -sS -X POST \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "x-goog-user-project: ${QUOTA_PROJECT}" \
  -H "Content-Type: application/json" \
  -d '{"projectId": "Invalid_Project_ID", "displayName": "invalid"}' \
  -o "${OUT}" -w 'HTTP %{http_code}\n' \
  "https://cloudresourcemanager.googleapis.com/v3/projects"

echo "saved ${OUT}" >&2
