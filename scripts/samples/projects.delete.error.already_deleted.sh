#!/usr/bin/env bash
# Delete a project that is already DELETE_REQUESTED. PROJECT is its number or ID.
# It reads the project first and stops unless it is DELETE_REQUESTED, so this
# can never delete a project that is still ACTIVE.
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
: "${PROJECT:?set PROJECT to the number or ID of a project that is already deleted}"

STATE_RESPONSE="$(curl -sS -X GET \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "x-goog-user-project: ${QUOTA_PROJECT}" \
  "https://cloudresourcemanager.googleapis.com/v3/projects/${PROJECT}")"
if ! grep -q '"state": *"DELETE_REQUESTED"' <<<"${STATE_RESPONSE}"; then
  echo "refusing: project ${PROJECT} is not in state DELETE_REQUESTED" >&2
  exit 1
fi

curl -sS -X DELETE \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "x-goog-user-project: ${QUOTA_PROJECT}" \
  -o "${OUT}" -w 'HTTP %{http_code}\n' \
  "https://cloudresourcemanager.googleapis.com/v3/projects/${PROJECT}"

echo "saved ${OUT}" >&2
