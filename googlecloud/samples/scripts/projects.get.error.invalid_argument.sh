#!/usr/bin/env bash
# Get a project whose name is not a valid project ID or number.
# The name is "ああああ", percent-encoded.
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

curl -sS -X GET \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "x-goog-user-project: ${QUOTA_PROJECT}" \
  -o "${OUT}" -w 'HTTP %{http_code}\n' \
  "https://cloudresourcemanager.googleapis.com/v3/projects/%E3%81%82%E3%81%82%E3%81%82%E3%81%82"

echo "saved ${OUT}" >&2
