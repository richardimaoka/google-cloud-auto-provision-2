#!/usr/bin/env bash
# Delete a project that does not exist. The random suffix makes sure of that,
# so nothing can be deleted.
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

curl -sS -X DELETE \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "x-goog-user-project: ${QUOTA_PROJECT}" \
  -o "${OUT}" -w 'HTTP %{http_code}\n' \
  "https://cloudresourcemanager.googleapis.com/v3/projects/this-does-not-exist-${RANDOM}${RANDOM}"

echo "saved ${OUT}" >&2
