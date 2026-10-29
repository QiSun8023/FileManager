#!/system/bin/sh
dupeguard_mode() {
  if [ "${KSU_HAS_HOT_INSTALL:-}" = true ] && [ "${KSU:-}" = true ]; then printf native; else printf deferred; fi
}
dupeguard_status() {
  dg_status_data=$1
  [ -d "$dg_status_data" ] && [ ! -L "$dg_status_data" ] && [ "$(readlink -f "$dg_status_data")" = "$dg_status_data" ] || return 1
  printf '%s\n%s\n' "$2" "$3" > "$dg_status_data/install-status.tmp.$$" && mv -f -- "$dg_status_data/install-status.tmp.$$" "$dg_status_data/install-status"
}
dupeguard_set_perm() {
  chown "$2:$3" "$1" && chmod "$4" "$1" && chcon u:object_r:system_file:s0 "$1"
}
dupeguard_set_perm_recursive() {
  find "$1" -type d -exec chown "$2:$3" {} \; -exec chmod "$4" {} \; -exec chcon u:object_r:system_file:s0 {} \; &&
  find "$1" -type f -exec chown "$2:$3" {} \; -exec chmod "$5" {} \; -exec chcon u:object_r:system_file:s0 {} \;
}
dupeguard_ready() {
  dg_ready_active=$1; dg_ready_data=$2; dg_ready_expected=$3
  [ -f "$dg_ready_active/webroot/index.html" ] && [ -x "$dg_ready_active/bin/dupeguard" ] || return 1
  dg_ready_info=$("$dg_ready_active/bin/dupeguard" info e30=) || return 1
  printf '%s' "$dg_ready_info" | grep -Fq '"ok":true' || return 1
  printf '%s' "$dg_ready_info" | grep -Fq "\"version\":\"$dg_ready_expected\"" || return 1
  rm -f -- "$dg_ready_active/update" || return 1
  rm -f -- "$dg_ready_active/action.sh" || return 1
  dupeguard_status "$dg_ready_data" ready "$dg_ready_expected 已就绪，请刷新 KernelSU 模块列表并打开 WebUI。"
}
dupeguard_deferred_update() (
  dg_up_stage=$1; dg_up_active=$2; dg_up_data=$3; dg_up_expected=$4; dg_up_token=$5; dg_up_installer=$6; dg_up_parent=$7
  dg_up_adb=${dg_up_data%/*}
  [ "$dg_up_data" = "$dg_up_adb/dupeguard" ] && [ "$dg_up_active" = "$dg_up_adb/modules/dupeguard" ] || exit 1
  case "$dg_up_stage" in "$dg_up_adb/modules_update/dupeguard"|"$dg_up_active") ;; *) exit 1 ;; esac
  [ "$(readlink -f "$dg_up_adb")" = "$dg_up_adb" ] && [ "$(readlink -f "$dg_up_data")" = "$dg_up_data" ] || exit 1
  case "$dg_up_token" in ''|*[!a-zA-Z0-9-]*) exit 1 ;; esac
  case "$dg_up_installer:$dg_up_parent" in *[!0-9:]*|:*) exit 1 ;; esac
  dg_up_done=0
  trap '[ "$dg_up_done" = 1 ] || dupeguard_status "$dg_up_data" error "免重启发布未完成，请查看 install.log；恢复记录保留。"' 0
  dupeguard_status "$dg_up_data" waiting "正在等待安装器完全退出。"
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
  [ "$(cat "$dg_up_stage/.dupeguard-install-token" 2>/dev/null)" = "$dg_up_token" ] || { printf 'Staging token changed, refusing stale update\n'; exit 1; }
  [ "$(sed -n 's/^version=//p' "$dg_up_stage/module.prop")" = "$dg_up_expected" ] || exit 1
  [ "$(sed -n 's/^version=//p' "$dg_up_active/module.prop" 2>/dev/null)" = "$dg_up_expected" ] || { printf 'Installer did not finish metadata publication\n'; exit 1; }
  dupeguard_status "$dg_up_data" publishing "安装器已退出，正在发布程序与 WebUI。"
  if [ "$dg_up_stage" != "$dg_up_active" ]; then
    dupeguard_install_live "$dg_up_stage" "$dg_up_active" "$dg_up_data" || exit 1
  fi
  dupeguard_ready "$dg_up_active" "$dg_up_data" "$dg_up_expected" || exit 1
  if [ "$dg_up_stage" != "$dg_up_active" ]; then
    [ "$(readlink -f "$dg_up_stage")" = "$dg_up_adb/modules_update/dupeguard" ] &&
    [ "$(cat "$dg_up_stage/.dupeguard-install-token" 2>/dev/null)" = "$dg_up_token" ] || exit 1
    rm -rf -- "$dg_up_stage" || exit 1
  fi
  dg_up_done=1
  printf '%s\n' "$dg_up_expected KernelSU/Magisk deferred hot install complete."
)
