#!/system/bin/sh
# Invoked by the manager. User explicitly chose to purge recycled files on uninstall.
filemanager_busybox() {
  for fm_bb in /data/adb/ksu/bin/busybox /data/adb/magisk/busybox /data/adb/ap/bin/busybox; do
    if [ -x "$fm_bb" ]; then printf '%s' "$fm_bb"; return 0; fi
  done
  return 1
}
filemanager_detach_cleanup() {
  "$1" nohup "$1" setsid "$1" sh "$2" --deferred </dev/null >>"$3" 2>&1 &
}
filemanager_queue_cleanup() {
  fm_queue_module=$1; fm_queue_data=$2
  fm_queue_dir="$fm_queue_data/uninstall-cleanup"
  fm_queue_bb=$(filemanager_busybox) || return 1
  [ ! -L "$fm_queue_dir" ] || return 1
  mkdir -p "$fm_queue_dir" || return 1
  [ "$(readlink -f "$fm_queue_dir")" = "$fm_queue_dir" ] || return 1
  for fm_queue_item in uninstall.sh module.prop filemanager-origin; do
    [ ! -L "$fm_queue_dir/$fm_queue_item" ] || return 1
    cp -f -- "$fm_queue_module/$fm_queue_item" "$fm_queue_dir/$fm_queue_item" || return 1
  done
  [ ! -L "$fm_queue_dir/generation" ] || return 1
  cat "$fm_queue_data/install-generation" > "$fm_queue_dir/generation" 2>/dev/null || printf unversioned > "$fm_queue_dir/generation"
  chmod 700 "$fm_queue_dir" "$fm_queue_dir/uninstall.sh" || return 1
  [ ! -L "$fm_queue_data/uninstall-cleanup.log" ] || return 1
  filemanager_detach_cleanup "$fm_queue_bb" "$fm_queue_dir/uninstall.sh" "$fm_queue_data/uninstall-cleanup.log"
}
filemanager_uninstall() (
  fm_module=$1; fm_adb=$2; fm_storage=$3
  fm_deferred=${4:-0}
  fm_data="$fm_adb/dupeguard"
  fm_canonical() { [ ! -L "$1" ] && [ "$(readlink -f "$1")" = "$1" ]; }
  fm_error() { printf '%s\n' "文件管理卸载清理未完成：$*" >&2; }
  if [ "$fm_deferred" = 1 ]; then
    [ "$fm_module" = "$fm_data/uninstall-cleanup" ] || exit 1
    [ ! -e "$fm_adb/modules/filemanager" ] && [ ! -L "$fm_adb/modules/filemanager" ] || exit 1
    [ ! -e "$fm_adb/modules_update/filemanager" ] && [ ! -L "$fm_adb/modules_update/filemanager" ] || exit 1
  else
    [ "$fm_module" = "$fm_adb/modules/filemanager" ] || exit 1
  fi
  for fm_dir in "$fm_adb" "$fm_adb/modules" "$fm_module"; do
    [ -d "$fm_dir" ] && fm_canonical "$fm_dir" || { fm_error "模块路径异常，已停止。"; exit 1; }
  done
  [ ! -L "$fm_module/filemanager-origin" ] && [ "$(cat "$fm_module/filemanager-origin" 2>/dev/null)" = QiSun8023/FileManager ] || exit 1
  [ "$(sed -n 's/^id=//p' "$fm_module/module.prop")" = filemanager ] || exit 1
  if [ -e "$fm_adb/modules/dupeguard" ] || [ -L "$fm_adb/modules/dupeguard" ]; then
    fm_error "旧模块仍存在，可能共用历史记录，请先完成迁移。"; exit 1
  fi
  [ ! -e "$fm_adb/modules_update/filemanager" ] && [ ! -L "$fm_adb/modules_update/filemanager" ] || { fm_error "新安装正在排队，已停止清理。"; exit 1; }
  if [ -e "$fm_data" ] || [ -L "$fm_data" ]; then
    [ -d "$fm_data" ] && fm_canonical "$fm_data" || exit 1
    if [ -e "$fm_data/lock" ] || [ -L "$fm_data/lock" ]; then
      fm_canonical "$fm_data/lock" && [ ! -L "$fm_data/lock/pid" ] || exit 1
      fm_pid=$(cat "$fm_data/lock/pid" 2>/dev/null)
      case "$fm_pid" in ''|*[!0-9]*|0) fm_error "操作锁状态不明。"; exit 1 ;; esac
      if kill -0 "$fm_pid" 2>/dev/null; then
        [ "$(readlink "/proc/$fm_pid/exe" 2>/dev/null)" = "$fm_module/bin/filemanager" ] || { fm_error "操作锁属于其他进程。"; exit 1; }
        printf 1 > "$fm_data/cancel" || exit 1
        fm_wait=0
        while kill -0 "$fm_pid" 2>/dev/null; do
          [ "$fm_wait" -lt 30 ] || { fm_error "文件任务尚未结束，记录和回收文件保留。"; exit 1; }
          sleep 1; fm_wait=$((fm_wait+1))
        done
      fi
      if [ -d "$fm_data/lock" ]; then
        [ "$(cat "$fm_data/lock/pid" 2>/dev/null)" = "$fm_pid" ] || exit 1
        rm -f -- "$fm_data/lock/pid" && rmdir -- "$fm_data/lock" || exit 1
      fi
    fi
  fi
  # Own the same exclusive lock as every file task during deletion.
  mkdir -p "$fm_data" || exit 1
  fm_canonical "$fm_data" || exit 1
  mkdir "$fm_data/lock" 2>/dev/null || { fm_error "有新的文件任务启动，请停止任务后重试。"; exit 1; }
  printf '%s' "$$" > "$fm_data/lock/pid" || exit 1
  fm_owned_lock=1
  trap 'if [ "$fm_owned_lock" = 1 ] && [ "$(cat "$fm_data/lock/pid" 2>/dev/null)" = "$$" ]; then rm -f -- "$fm_data/lock/pid"; rmdir -- "$fm_data/lock" 2>/dev/null || true; fi' 0
  trap 'exit 1' 1 2 15
  # Validate every shared-storage ancestor before deleting anything.
  fm_storage_ready=1
  [ -d "$fm_storage" ] && fm_canonical "$fm_storage" || fm_storage_ready=0
  if command -v getprop >/dev/null 2>&1; then
    [ "$(getprop sys.boot_completed 2>/dev/null)" = 1 ] && [ "$(getprop sys.user.0.ce_available 2>/dev/null)" = true ] || fm_storage_ready=0
  fi
  if [ "$fm_storage_ready" = 0 ]; then
    if [ "$fm_deferred" = 0 ] && filemanager_queue_cleanup "$fm_module" "$fm_data"; then
      printf '%s\n' '文件管理：共享存储暂不可用，已排队等待解锁后清理。'
      exit 0
    fi
    fm_error "共享存储不可用，记录保留。"; exit 1
  fi
  for fm_dir in "$fm_storage/Android" "$fm_storage/Android/FileManager" "$fm_storage/Android/FileManager/Trash" "$fm_storage/.dupeguard-trash"; do
    if [ -e "$fm_dir" ] || [ -L "$fm_dir" ]; then
      [ -d "$fm_dir" ] && fm_canonical "$fm_dir" || { fm_error "回收站路径异常，记录保留。"; exit 1; }
    fi
  done
  for fm_target in "$fm_storage/Android/FileManager/Trash" "$fm_storage/.dupeguard-trash"; do
    if [ -d "$fm_target" ]; then rm -rf -- "$fm_target" || { fm_error "不能删除回收文件，私有记录保留。"; exit 1; }; fi
  done
  # This parent is removed only when empty; other files below it are preserved.
  rmdir -- "$fm_storage/Android/FileManager" 2>/dev/null || true
  if [ -d "$fm_data" ]; then rm -rf -- "$fm_data" || { fm_error "不能删除私有记录。"; exit 1; }; fi
  fm_owned_lock=0
  printf '%s\n' '文件管理：已清理回收文件、配置、索引、历史记录、日志和旧模块备份。'
)
filemanager_uninstall_entry() {
  if [ "${1:-}" != --deferred ]; then
    filemanager_uninstall "${0%/*}" /data/adb /storage/emulated/0
    return $?
  fi
  fm_worker=/data/adb/dupeguard/uninstall-cleanup
  [ "${0%/*}" = "$fm_worker" ] && [ ! -L "$fm_worker" ] && [ "$(readlink -f "$fm_worker")" = "$fm_worker" ] || return 1
  [ "$(cat "$fm_worker/filemanager-origin" 2>/dev/null)" = QiSun8023/FileManager ] || return 1
  fm_removed=0; fm_waits=0
  while [ "$fm_waits" -lt 900 ]; do
    fm_generation=$(cat /data/adb/dupeguard/install-generation 2>/dev/null || printf unversioned)
    [ "$(cat "$fm_worker/generation" 2>/dev/null)" = "$fm_generation" ] || return 0
    if [ ! -e /data/adb/modules/filemanager ]; then fm_removed=1; fi
    if [ "$fm_removed" = 1 ]; then
      # A new installation owns the records again; never clean its data.
      if [ -e /data/adb/modules/filemanager ] || [ -e /data/adb/modules_update/filemanager ]; then return 0; fi
      if [ "$(getprop sys.boot_completed 2>/dev/null)" = 1 ] && [ "$(getprop sys.user.0.ce_available 2>/dev/null)" = true ] && [ -d /storage/emulated/0 ] && [ -r /storage/emulated/0 ] && [ -x /storage/emulated/0 ]; then
        filemanager_uninstall "$fm_worker" /data/adb /storage/emulated/0 1
        return $?
      fi
    fi
    sleep 2; fm_waits=$((fm_waits+1))
  done
  printf '%s\n' '等待共享存储解锁超时，未完成的清理记录保留。' >&2
  return 1
}
filemanager_uninstall_entry "$@"
