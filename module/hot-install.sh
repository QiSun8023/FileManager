#!/system/bin/sh
# Keep the historical data location so all existing indexes and records are reused.
# Retire only the verified legacy module after the new backend is ready.
filemanager_retire_legacy() (
  fm_active=$1; fm_data=$2; fm_adb=${2%/*}
  [ "$fm_active" = "$fm_adb/modules/filemanager" ] && [ "$fm_data" = "$fm_adb/dupeguard" ] || exit 1
  [ ! -L "$fm_adb" ] && [ "$(readlink -f "$fm_adb")" = "$fm_adb" ] || exit 1
  [ ! -L "$fm_adb/modules" ] && [ "$(readlink -f "$fm_adb/modules")" = "$fm_adb/modules" ] || exit 1
  for fm_path in "$fm_active" "$fm_data"; do
    [ -d "$fm_path" ] && [ ! -L "$fm_path" ] && [ "$(readlink -f "$fm_path")" = "$fm_path" ] || exit 1
  done
  [ "$(sed -n 's/^id=//p' "$fm_active/module.prop")" = filemanager ] || exit 1
  fm_old="$fm_adb/modules/dupeguard"
  [ ! -e "$fm_adb/modules_update/dupeguard" ] && [ ! -L "$fm_adb/modules_update/dupeguard" ] || exit 1
  [ ! -e "$fm_old" ] && [ ! -L "$fm_old" ] && exit 0
  [ -d "$fm_old" ] && [ ! -L "$fm_old" ] && [ "$(readlink -f "$fm_old")" = "$fm_old" ] || exit 1
  [ "$(sed -n 's/^id=//p' "$fm_old/module.prop")" = dupeguard ] || exit 1
  [ -z "$(find "$fm_old" -type l -print 2>/dev/null)" ] || exit 1
  fm_backup="$fm_data/legacy-module-dupeguard"
  [ ! -e "$fm_backup" ] && [ ! -L "$fm_backup" ] || exit 1
  # Both module generations use this same lock. Never move a running legacy job.
  if ! mkdir "$fm_data/lock" 2>/dev/null; then
    [ -d "$fm_data/lock" ] && [ ! -L "$fm_data/lock" ] && [ "$(readlink -f "$fm_data/lock")" = "$fm_data/lock" ] || exit 1
    [ ! -L "$fm_data/lock/pid" ] || exit 1
    fm_pid=$(cat "$fm_data/lock/pid" 2>/dev/null)
    case "$fm_pid" in ''|*[!0-9]*|0) exit 1 ;; esac
    kill -0 "$fm_pid" 2>/dev/null && exit 1
    rm -f -- "$fm_data/lock/pid" && rmdir -- "$fm_data/lock" && mkdir "$fm_data/lock" || exit 1
  fi
  trap 'rm -f -- "$fm_data/lock/pid"; rmdir -- "$fm_data/lock" 2>/dev/null || true' 0
  trap 'exit 1' 1 2 15
  printf '%s' "$$" > "$fm_data/lock/pid" || exit 1
  mv -- "$fm_old" "$fm_backup" || exit 1
  if ! printf '%s\n' "$fm_backup" > "$fm_data/legacy-module-backup"; then
    [ ! -e "$fm_old" ] && [ ! -L "$fm_old" ] && mv -- "$fm_backup" "$fm_old"
    exit 1
  fi
)
filemanager_mode() {
  if [ "${KSU_HAS_HOT_INSTALL:-}" = true ] && [ "${KSU:-}" = true ]; then printf native; else printf deferred; fi
}
filemanager_status() {
  dg_status_data=$1
  [ -d "$dg_status_data" ] && [ ! -L "$dg_status_data" ] && [ "$(readlink -f "$dg_status_data")" = "$dg_status_data" ] || return 1
  printf '%s\n%s\n' "$2" "$3" > "$dg_status_data/install-status.tmp.$$" && mv -f -- "$dg_status_data/install-status.tmp.$$" "$dg_status_data/install-status"
}
filemanager_set_perm() {
  chown "$2:$3" "$1" && chmod "$4" "$1" && chcon u:object_r:system_file:s0 "$1"
}
filemanager_set_perm_recursive() {
  find "$1" -type d -exec chown "$2:$3" {} \; -exec chmod "$4" {} \; -exec chcon u:object_r:system_file:s0 {} \; &&
  find "$1" -type f -exec chown "$2:$3" {} \; -exec chmod "$5" {} \; -exec chcon u:object_r:system_file:s0 {} \;
}
filemanager_ready() {
  dg_ready_active=$1; dg_ready_data=$2; dg_ready_expected=$3
  [ -f "$dg_ready_active/webroot/index.html" ] && [ -x "$dg_ready_active/bin/filemanager" ] || return 1
  dg_ready_info=$("$dg_ready_active/bin/filemanager" info e30=) || return 1
  printf '%s' "$dg_ready_info" | grep -Fq '"ok":true' || return 1
  printf '%s' "$dg_ready_info" | grep -Fq "\"version\":\"$dg_ready_expected\"" || return 1
  rm -f -- "$dg_ready_active/update" || return 1
  rm -f -- "$dg_ready_active/action.sh" || return 1
  filemanager_retire_legacy "$dg_ready_active" "$dg_ready_data" || return 1
  filemanager_status "$dg_ready_data" ready "$dg_ready_expected 已就绪，请刷新 KernelSU 模块列表并打开 WebUI。"
}
filemanager_deferred_update() (
  dg_up_stage=$1; dg_up_active=$2; dg_up_data=$3; dg_up_expected=$4; dg_up_token=$5; dg_up_installer=$6; dg_up_parent=$7
  dg_up_adb=${dg_up_data%/*}
  [ "$dg_up_data" = "$dg_up_adb/dupeguard" ] && [ "$dg_up_active" = "$dg_up_adb/modules/filemanager" ] || exit 1
  case "$dg_up_stage" in "$dg_up_adb/modules_update/filemanager"|"$dg_up_active") ;; *) exit 1 ;; esac
  [ "$(readlink -f "$dg_up_adb")" = "$dg_up_adb" ] && [ "$(readlink -f "$dg_up_data")" = "$dg_up_data" ] || exit 1
  case "$dg_up_token" in ''|*[!a-zA-Z0-9-]*) exit 1 ;; esac
  case "$dg_up_installer:$dg_up_parent" in *[!0-9:]*|:*) exit 1 ;; esac
  dg_up_done=0
  trap '[ "$dg_up_done" = 1 ] || filemanager_status "$dg_up_data" error "免重启发布未完成，请查看 install.log；恢复记录保留。"' 0
  filemanager_status "$dg_up_data" waiting "正在等待安装器完全退出。"
  dg_up_wait=0
  while :; do
    dg_up_alive=0
    for dg_up_pid in "$dg_up_installer" "$dg_up_parent"; do
      [ "$dg_up_pid" -gt 0 ] && kill -0 "$dg_up_pid" 2>/dev/null && dg_up_alive=1
    done
    [ "$dg_up_alive" = 0 ] && break
    [ "$dg_up_wait" -lt 600 ] || { printf 'Installer wait timed out\n'; exit 1; }
    sleep 0.2; dg_up_wait=$((dg_up_wait+1))
  done
  [ -d "$dg_up_stage" ] && [ ! -L "$dg_up_stage" ] && [ "$(readlink -f "$dg_up_stage")" = "$dg_up_stage" ] || exit 1
  [ "$(cat "$dg_up_stage/.filemanager-install-token" 2>/dev/null)" = "$dg_up_token" ] || { printf 'Staging token changed, refusing stale update\n'; exit 1; }
  [ "$(sed -n 's/^version=//p' "$dg_up_stage/module.prop")" = "$dg_up_expected" ] || exit 1
  [ "$(sed -n 's/^version=//p' "$dg_up_active/module.prop" 2>/dev/null)" = "$dg_up_expected" ] || { printf 'Installer did not finish metadata publication\n'; exit 1; }
  filemanager_status "$dg_up_data" publishing "安装器已退出，正在发布程序与 WebUI。"
  if [ "$dg_up_stage" != "$dg_up_active" ]; then
    filemanager_install_live "$dg_up_stage" "$dg_up_active" "$dg_up_data" || exit 1
  fi
  filemanager_ready "$dg_up_active" "$dg_up_data" "$dg_up_expected" || exit 1
  if [ "$dg_up_stage" != "$dg_up_active" ]; then
    [ "$(readlink -f "$dg_up_stage")" = "$dg_up_adb/modules_update/filemanager" ] &&
    [ "$(cat "$dg_up_stage/.filemanager-install-token" 2>/dev/null)" = "$dg_up_token" ] || exit 1
    rm -rf -- "$dg_up_stage" || exit 1
  fi
  dg_up_done=1
  printf '%s\n' "$dg_up_expected KernelSU/Magisk deferred hot install complete."
)
