#!/usr/bin/env bash
# Get an ACTIVE project that has a parent. PROJECT is its number or ID.
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
ask PROJECT "Number or ID of an ACTIVE project with a parent" '^([0-9]+|[a-z][a-z0-9-]{4,29})$'

curl -sS -X GET \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "x-goog-user-project: ${QUOTA_PROJECT}" \
  -o "${OUT}" -w 'HTTP %{http_code}\n' \
  "https://cloudresourcemanager.googleapis.com/v3/projects/${PROJECT}"

echo "saved ${OUT}" >&2
