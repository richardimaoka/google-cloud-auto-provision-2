#!/usr/bin/env bash
# Get a project that does not exist. The random suffix makes sure of that.
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

curl -sS -X GET \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "x-goog-user-project: ${QUOTA_PROJECT}" \
  -o "${OUT}" -w 'HTTP %{http_code}\n' \
  "https://cloudresourcemanager.googleapis.com/v3/projects/this-does-not-exist-${RANDOM}${RANDOM}"

echo "saved ${OUT}" >&2
