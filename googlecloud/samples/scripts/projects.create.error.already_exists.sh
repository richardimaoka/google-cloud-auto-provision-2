#!/usr/bin/env bash
# Create a project with the ID of a project that exists already, so the request
# is rejected with 409 and nothing is created. EXISTING_PROJECT_ID is that ID.
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
ask EXISTING_PROJECT_ID "ID of a project that exists" '^[a-z][a-z0-9-]{4,29}$'

curl -sS -X POST \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "x-goog-user-project: ${QUOTA_PROJECT}" \
  -H "Content-Type: application/json" \
  -d "{\"projectId\": \"${EXISTING_PROJECT_ID}\", \"displayName\": \"${EXISTING_PROJECT_ID}\"}" \
  -o "${OUT}" -w 'HTTP %{http_code}\n' \
  "https://cloudresourcemanager.googleapis.com/v3/projects"

echo "saved ${OUT}" >&2
