# Sourced by the sample scripts. Picks who to call as, gets a token, and sets
# TOKEN and OUT. Stops on any error.
#
# ACCOUNT is a logged-in gcloud account and QUOTA_PROJECT is the project the API
# calls are counted against. Set them as environment variables, or leave them
# out and choose from a menu. The pairs chosen before are remembered in
# out/.recent, so the next run is one number. Other values a script needs, like
# PROJECT, are asked for the same way.
#
# The response is saved to out/<script name>.json. It holds real identifiers, so
# out/ is git-ignored. Replace them with placeholders before the file goes into
# gcp/sampledata.

set -euo pipefail

SAMPLES_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUT_DIR="${SAMPLES_OUT_DIR:-${SAMPLES_DIR}/out}"
HISTORY_FILE="${OUT_DIR}/.recent"
mkdir -p "${OUT_DIR}"

die() {
  echo "$*" >&2
  exit 1
}

# pick PROMPT ITEM...: shows a numbered menu and prints the item that is chosen.
# The menu and the question go to the terminal, and only the chosen item to
# stdout, so `x="$(pick ...)"` works. For `pick "Account" a@example.com b@example.com`
# the user sees this, and types 2:
#
#   1) a@example.com
#   2) b@example.com
#   Account: 2
#
# An answer that is not one of the numbers asks again. Ctrl-D gives up, and the
# script stops with "nothing was chosen". Without a terminal it stops at once.
pick() {
  local prompt="$1"
  shift
  [ -t 0 ] || die "cannot ask \"${prompt}\": there is no terminal"
  local PS3="${prompt}: " choice
  select choice in "$@"; do
    if [ -n "${choice}" ]; then
      printf '%s\n' "${choice}"
      return 0
    fi
  done
  die "nothing was chosen"
}

# ask NAME PROMPT PATTERN: if the variable NAME is not set, asks for it on the
# terminal, and stops unless the answer matches the regular expression PATTERN.
# Nothing is shown when NAME is already set. For
# `ask OPERATION "Paste the operation name, like operations/dp.123" '^operations/...'`
# the user sees this, and pastes a name:
#
#   Paste the operation name, like operations/dp.123: operations/dp.7312384975
#
# An answer that does not match PATTERN stops the script:
#
#   Paste the operation name, like operations/dp.123: dp.7312384975
#   OPERATION does not look right: dp.7312384975
ask() {
  local name="$1" prompt="$2" pattern="$3" value
  if [ -n "${!name:-}" ]; then
    return 0
  fi
  [ -t 0 ] || die "set ${name}: there is no terminal to ask on"
  read -r -p "${prompt}: " value
  [[ "${value}" =~ ${pattern} ]] || die "${name} does not look right: ${value}"
  printf -v "${name}" '%s' "${value}"
}

# remember_identity puts the current ACCOUNT and QUOTA_PROJECT first in the
# history, and keeps the last 10 pairs.
remember_identity() {
  local pair tmp="${HISTORY_FILE}.tmp"
  pair="$(printf '%s\t%s' "${ACCOUNT}" "${QUOTA_PROJECT}")"
  {
    printf '%s\n' "${pair}"
    if [ -f "${HISTORY_FILE}" ]; then
      grep -vxF "${pair}" "${HISTORY_FILE}" || true
    fi
  } | head -n 10 >"${tmp}"
  mv "${tmp}" "${HISTORY_FILE}"
}

# choose_identity sets ACCOUNT and QUOTA_PROJECT, and asks only for what is not
# set already. Up to three questions are shown, one after the other.
#
# 1. Neither is set and there are pairs from earlier runs (out/.recent). The
#    newest pair is number 1:
#
#      1) b@example.com / proj-two
#      2) a@example.com / proj-one
#      3) another combination...
#      Call as (account / quota project): 1
#
#    Choosing a pair ends the questions. Choosing "another combination..." goes
#    on with 2 and 3. With no earlier pairs, it starts at 2.
#
# 2. ACCOUNT is not set. The accounts that are logged in (gcloud auth list):
#
#      1) a@example.com
#      2) b@example.com
#      Account: 2
#
# 3. QUOTA_PROJECT is not set. The projects that ACCOUNT can see (gcloud
#    projects list). The last item is for a project that is not in the list, or
#    for when the list is empty, for example because the login has expired:
#
#      1) proj-one
#      2) proj-two
#      3) a project ID that is not listed...
#      Quota project: 3
#      Quota project ID: my-other-project
#
#    Choosing 1 or 2 ends the question. The typed ID must look like a project ID.
#
# Whatever is chosen is remembered by remember_identity, which asks nothing.
choose_identity() {
  local recent=() a p choice other="another combination..."

  # Neither is set: offer the pairs used before.
  if [ -z "${ACCOUNT:-}" ] && [ -z "${QUOTA_PROJECT:-}" ] && [ -f "${HISTORY_FILE}" ]; then
    while IFS=$'\t' read -r a p; do
      if [ -n "${a}" ]; then
        recent+=("${a} / ${p}")
      fi
    done <"${HISTORY_FILE}"
    if [ "${#recent[@]}" -gt 0 ]; then
      choice="$(pick "Call as (account / quota project)" "${recent[@]}" "${other}")"
      if [ "${choice}" != "${other}" ]; then
        ACCOUNT="${choice%% / *}"
        QUOTA_PROJECT="${choice##* / }"
      fi
    fi
  fi

  if [ -z "${ACCOUNT:-}" ]; then
    local accounts
    mapfile -t accounts < <(gcloud auth list --format='value(account)')
    [ "${#accounts[@]}" -gt 0 ] || die "no gcloud account is logged in, run: gcloud auth login"
    ACCOUNT="$(pick "Account" "${accounts[@]}")"
  fi

  if [ -z "${QUOTA_PROJECT:-}" ]; then
    local projects typed="a project ID that is not listed..."
    mapfile -t projects < <(gcloud projects list --account="${ACCOUNT}" --format='value(projectId)' 2>/dev/null || true)
    choice="$(pick "Quota project" "${projects[@]+"${projects[@]}"}" "${typed}")"
    if [ "${choice}" = "${typed}" ]; then
      read -r -p "Quota project ID: " QUOTA_PROJECT
      [[ "${QUOTA_PROJECT}" =~ ^[a-z][a-z0-9-]{4,29}$ ]] || die "not a project ID: ${QUOTA_PROJECT}"
    else
      QUOTA_PROJECT="${choice}"
    fi
  fi

  remember_identity
}

choose_identity

TOKEN="$(gcloud auth print-access-token --account="${ACCOUNT}")" ||
  die "could not get a token for ${ACCOUNT}. Log in again with: gcloud auth login"

SAMPLE="$(basename "${BASH_SOURCE[1]}" .sh)"
OUT="${OUT_DIR}/${SAMPLE}.json"

echo "calling as ${ACCOUNT}, counted against ${QUOTA_PROJECT}" >&2
