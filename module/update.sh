#!/system/bin/sh
DG_UPDATE_DIR=${0%/*}
. "$DG_UPDATE_DIR/hot-install.sh" || exit 1
. "$DG_UPDATE_DIR/live-install.sh" || exit 1
ui_print() { printf '%s\n' "$*"; }
set_perm() { filemanager_set_perm "$@"; }
set_perm_recursive() { filemanager_set_perm_recursive "$@"; }
filemanager_deferred_update "$3" /data/adb/modules/filemanager /data/adb/dupeguard v1.7.3 "$4" "$1" "$2"
