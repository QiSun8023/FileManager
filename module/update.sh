#!/system/bin/sh
DG_UPDATE_DIR=${0%/*}
. "$DG_UPDATE_DIR/hot-install.sh" || exit 1
. "$DG_UPDATE_DIR/live-install.sh" || exit 1
ui_print() { printf '%s\n' "$*"; }
set_perm() { dupeguard_set_perm "$@"; }
set_perm_recursive() { dupeguard_set_perm_recursive "$@"; }
dupeguard_deferred_update "$3" /data/adb/modules/dupeguard /data/adb/dupeguard v1.5.1 "$4" "$1" "$2"
