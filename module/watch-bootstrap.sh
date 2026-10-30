#!/system/bin/sh
# Run only after the installer callback exits and KernelSU finishes its directory swap.
fm_boot_parent=$1; fm_boot_version=$2
fm_boot_module=/data/adb/modules/filemanager
fm_boot_data=/data/adb/dupeguard
case "$fm_boot_parent" in ''|*[!0-9]*|0|1) exit 1 ;; esac
case "$fm_boot_version" in v[0-9]*.[0-9]*.[0-9]*) ;; *) exit 1 ;; esac
if grep -q '0::/apps/' /proc/self/cgroup 2>/dev/null; then printf '%s' "$$" > /sys/fs/cgroup/cgroup.procs || exit 1; fi
fm_boot_wait=0
while kill -0 "$fm_boot_parent" 2>/dev/null; do
 [ "$fm_boot_wait" -lt 60 ] || exit 1
 sleep 1; fm_boot_wait=$((fm_boot_wait+1))
done
sleep 2
[ "$(sed -n 's/^version=//p' "$fm_boot_module/module.prop")" = "$fm_boot_version" ] || exit 1
[ "$(cat "$fm_boot_module/filemanager-origin")" = QiSun8023/FileManager ] || exit 1
[ ! -f "$fm_boot_module/disable" ] && [ ! -f "$fm_boot_module/remove" ] || exit 0
. "$fm_boot_module/companion-install.sh" || exit 1
filemanager_install_companion "$fm_boot_module"
filemanager_start_monitor "$fm_boot_module"
