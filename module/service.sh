#!/system/bin/sh
FM_SERVICE_DIR=${0%/*}
[ "$FM_SERVICE_DIR" = /data/adb/modules/filemanager ] || exit 1
while [ "$(getprop sys.boot_completed)" != 1 ] || [ "$(getprop sys.user.0.ce_available)" != true ]; do sleep 3; done
[ ! -f "$FM_SERVICE_DIR/disable" ] && [ ! -f "$FM_SERVICE_DIR/remove" ] || exit 0
. "$FM_SERVICE_DIR/companion-install.sh" || exit 1
filemanager_install_companion "$FM_SERVICE_DIR"
filemanager_start_monitor "$FM_SERVICE_DIR"
