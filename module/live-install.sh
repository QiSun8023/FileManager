#!/system/bin/sh
# Sourced by customize.sh. Publish this data-only module without a reboot.
# The three paths must be siblings under the same canonical adb directory.
filemanager_install_live() (
  dg_stage=$1
  dg_active=$2
  dg_data=$3
  dg_adb=${dg_data%/*}
  dg_nonce=$$
  dg_prepared="$dg_adb/modules/.filemanager-live-$dg_nonce"
  dg_backup="$dg_data/.filemanager-backup-$dg_nonce"
  dg_owned_lock=0
  dg_published=0
  dg_owned_prepared=0
  dg_owned_backup=0

  dg_say() { ui_print "文件管理: $*"; }
  dg_canonical() { [ ! -L "$1" ] && [ "$(readlink -f "$1")" = "$1" ]; }
  dg_no_links() { [ -z "$(find "$1" -type l -print 2>/dev/null)" ]; }
  dg_remove_temp() {
    case "$1" in
      "$dg_prepared"|"$dg_backup") ;;
      *) return 1 ;;
    esac
    [ ! -e "$1" ] && [ ! -L "$1" ] && return 0
    dg_canonical "$1" || return 1
    # Only the two validated, process-specific publication directories qualify.
    rm -rf -- "$1"
  }
  dg_cleanup() {
    if [ "$dg_owned_backup" = 1 ] && [ -d "$dg_backup" ] && [ ! -e "$dg_active" ] && [ ! -L "$dg_active" ]; then
      mv -- "$dg_backup" "$dg_active" || dg_say "旧版回退失败，备份保留在 $dg_backup"
    fi
    if [ "$dg_owned_prepared" = 1 ]; then dg_remove_temp "$dg_prepared" || dg_say "临时目录保留在 $dg_prepared"; fi
    if [ "$dg_published" = 1 ] && [ "$dg_owned_backup" = 1 ]; then
      dg_remove_temp "$dg_backup" || dg_say "旧版备份保留在 $dg_backup"
    fi
    if [ "$dg_owned_lock" = 1 ]; then
      rm -f -- "$dg_data/lock/pid"
      rmdir -- "$dg_data/lock" 2>/dev/null || true
    fi
  }
  # Fail before any mutation when paths are outside the expected layout.
  [ "$dg_stage" = "$dg_adb/modules_update/filemanager" ] || exit 1
  [ "$dg_active" = "$dg_adb/modules/filemanager" ] || exit 1
  [ "$dg_data" = "$dg_adb/dupeguard" ] || exit 1
  dg_canonical "$dg_adb" && dg_canonical "$dg_stage" || exit 1
  dg_no_links "$dg_stage" || { dg_say "安装包包含符号链接，已取消。"; exit 1; }
  [ "$(sed -n 's/^id=//p' "$dg_stage/module.prop")" = filemanager ] || exit 1
  [ -f "$dg_stage/bin/filemanager" ] && [ -f "$dg_stage/webroot/index.html" ] || exit 1
  if [ -e "$dg_active" ] || [ -L "$dg_active" ]; then
    dg_canonical "$dg_active" && dg_no_links "$dg_active" || { dg_say "旧模块目录包含链接，已取消。"; exit 1; }
    [ -d "$dg_active" ] || exit 1
  fi
  mkdir -p "$dg_adb/modules" "$dg_data" || exit 1
  dg_canonical "$dg_adb/modules" && dg_canonical "$dg_data" || exit 1
  set_perm "$dg_data" 0 0 0700 || exit 1
  if ! mkdir "$dg_data/lock" 2>/dev/null; then
    dg_canonical "$dg_data/lock" || exit 1
    [ ! -L "$dg_data/lock/pid" ] || exit 1
    dg_pid=$(cat "$dg_data/lock/pid" 2>/dev/null)
    case "$dg_pid" in ''|*[!0-9]*|0) dg_say "操作锁状态不明，请在旧版完成一次操作后重试安装。"; exit 1 ;; esac
    if kill -0 "$dg_pid" 2>/dev/null; then
      dg_say "正在扫描或处理文件，请等待完成（或停止扫描）后重新安装。"
      exit 1
    fi
    rm -f -- "$dg_data/lock/pid" && rmdir -- "$dg_data/lock" && mkdir "$dg_data/lock" || exit 1
  fi
  dg_owned_lock=1
  trap dg_cleanup 0
  trap 'exit 1' 1 2 15
  printf '%s' "$$" > "$dg_data/lock/pid" || exit 1
  [ ! -e "$dg_prepared" ] && [ ! -L "$dg_prepared" ] || exit 1
  [ ! -e "$dg_backup" ] && [ ! -L "$dg_backup" ] || exit 1
  mkdir "$dg_prepared" || exit 1
  dg_owned_prepared=1
  # Preserve unrecognised module-root files; records live outside this directory.
  if [ -d "$dg_active" ]; then cp -a "$dg_active/." "$dg_prepared/" || exit 1; fi
  for dg_item in bin webroot module.prop skip_mount LICENSE GO-LICENSE filemanager-origin; do
    [ -e "$dg_stage/$dg_item" ] || exit 1
    cp -a "$dg_stage/$dg_item" "$dg_prepared/" || exit 1
  done
  for dg_item in live-install.sh hot-install.sh update.sh post-install.sh .filemanager-install-token; do
    if [ -f "$dg_stage/$dg_item" ]; then cp -a "$dg_stage/$dg_item" "$dg_prepared/" || exit 1; fi
  done
  dg_no_links "$dg_prepared" || exit 1
  rm -f -- "$dg_prepared/action.sh" || exit 1
  set_perm_recursive "$dg_prepared" 0 0 0755 0644 || exit 1
  set_perm "$dg_prepared/bin/filemanager" 0 0 0755 || exit 1
  dg_expected=$(sed -n 's/^version=//p' "$dg_stage/module.prop")
  dg_info=$("$dg_prepared/bin/filemanager" info e30=) || { dg_say "后端启动检查失败，未切换版本。"; exit 1; }
  printf '%s' "$dg_info" | grep -Fq '"ok":true' || exit 1
  printf '%s' "$dg_info" | grep -Fq "\"version\":\"$dg_expected\"" || { dg_say "后端版本不匹配，未切换版本。"; exit 1; }
  if [ -d "$dg_active" ]; then dg_owned_backup=1; mv -- "$dg_active" "$dg_backup" || exit 1; fi
  if ! mv -- "$dg_prepared" "$dg_active"; then
    dg_say "新版切换失败，将尝试回退旧版。"
    exit 1
  fi
  dg_published=1
  dg_say "已立即启用 $dg_expected，无需重启手机。"
)
