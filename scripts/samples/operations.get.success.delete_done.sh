#!/usr/bin/env bash
# Get a finished delete operation. OPERATION is its name, like operations/dp.123.
# Operations are deleted a few hours after they finish.
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
ask OPERATION "Paste the operation name, like operations/dp.123" '^operations/[A-Za-z0-9_.-]+$'

curl -sS -X GET \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "x-goog-user-project: ${QUOTA_PROJECT}" \
  -o "${OUT}" -w 'HTTP %{http_code}\n' \
  "https://cloudresourcemanager.googleapis.com/v3/${OPERATION}"

echo "saved ${OUT}" >&2
