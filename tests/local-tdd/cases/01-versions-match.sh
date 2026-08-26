#!/usr/bin/env bash
# GREEN when the image self-reports the versions.yaml pins.
# RED  when it self-reports OLDER versions — the candidate is not cut yet.
# BROKEN on anything else (newer than pinned, or garbage): the ledger and the
# artifact disagree in a direction no pending build explains.
source "$(dirname "$0")/../lib.sh"

PGRDF="$(Q "SELECT extversion FROM pg_extension WHERE extname='pgrdf';")"
PGCK="$(Q "SELECT extversion FROM pg_extension WHERE extname='pgck';")"
[[ "$PGRDF" =~ ^[0-9.]+$ && "$PGCK" =~ ^[0-9.]+$ ]] || BROKEN "unreadable extversions (pgrdf='$PGRDF' pgck='$PGCK')"

if [[ "$PGRDF" == "$OCIGER_PGRDF_VERSION" && "$PGCK" == "$OCIGER_PGCK_VERSION" ]]; then
  GREEN "pgrdf $PGRDF + pgck $PGCK == versions.yaml"
fi
older() { [ "$(printf '%s\n%s' "$1" "$2" | sort -V | head -1)" = "$1" ] && [ "$1" != "$2" ]; }
if older "$PGRDF" "$OCIGER_PGRDF_VERSION" || older "$PGCK" "$OCIGER_PGCK_VERSION"; then
  RED "image at pgrdf $PGRDF / pgck $PGCK, versions.yaml wants $OCIGER_PGRDF_VERSION/$OCIGER_PGCK_VERSION — re-cut pending"
fi
BROKEN "image reports pgrdf $PGRDF / pgck $PGCK, NEWER than the pinned $OCIGER_PGRDF_VERSION/$OCIGER_PGCK_VERSION"
