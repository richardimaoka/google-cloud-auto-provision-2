# Sourced by the sample scripts. Sets TOKEN and OUT, and stops on any error.
#
# The API is called as the gcloud account in ACCOUNT, and counted against the
# project in QUOTA_PROJECT. The response is saved to out/<script name>.json. It
# holds real identifiers, so out/ is git-ignored. Replace them with placeholders
# before the file goes into gcp/sampledata.

set -euo pipefail

: "${ACCOUNT:?set ACCOUNT to a logged-in gcloud account, see: gcloud auth list}"
: "${QUOTA_PROJECT:?set QUOTA_PROJECT to the project the API calls are counted against}"

TOKEN="$(gcloud auth print-access-token --account="${ACCOUNT}")"

SAMPLE="$(basename "${BASH_SOURCE[1]}" .sh)"
OUT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/out/${SAMPLE}.json"
mkdir -p "$(dirname "${OUT}")"

echo "calling as ${ACCOUNT}, counted against ${QUOTA_PROJECT}" >&2
