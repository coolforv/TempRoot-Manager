#!/system/bin/sh
# Run only from GhostLock's already-rooted worker. This hook does not escalate privileges.
set -eu
BROKER=/data/local/tmp/trm-broker
GRANTS=/data/local/tmp/temproot/grants
LOG=/data/local/tmp/temproot/broker.log
MANAGER_PACKAGE=com.temproot.manager
mkdir -p /data/local/tmp/temproot
chmod 0700 /data/local/tmp/temproot
[ "$(id -u)" = "0" ] || { echo "not UID 0" >> "$LOG"; exit 1; }
[ -x "$BROKER" ] || { echo "broker binary missing" >> "$LOG"; exit 1; }
LINE="$(cmd package list packages -U "$MANAGER_PACKAGE" 2>/dev/null | head -1)"
UID_VALUE="$(echo "$LINE" | sed -n 's/.*uid:\([0-9][0-9]*\).*/\1/p')"
[ -n "$UID_VALUE" ] || { echo "manager app not installed" >> "$LOG"; exit 1; }
nohup "$BROKER" --socket /data/local/tmp/temproot.sock --grants "$GRANTS" --manager-uid "$UID_VALUE" >>"$LOG" 2>&1 </dev/null &
echo "broker pid=$! manager_uid=$UID_VALUE" >> "$LOG"
