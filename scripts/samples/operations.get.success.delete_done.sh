#!/usr/bin/env bash
# Get a finished delete operation. OPERATION is its name, like operations/dp.123.
# Operations are deleted a few hours after they finish.
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
: "${OPERATION:?set OPERATION to the name of a delete operation, like operations/dp.123}"

curl -sS -X GET \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "x-goog-user-project: ${QUOTA_PROJECT}" \
  -o "${OUT}" -w 'HTTP %{http_code}\n' \
  "https://cloudresourcemanager.googleapis.com/v3/${OPERATION}"

echo "saved ${OUT}" >&2
