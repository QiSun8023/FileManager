#!/system/bin/sh
# Sourced only after publication or by the boot service. Never grants permissions.
filemanager_install_companion() (
 fm_helper_module=$1
 [ "$fm_helper_module" = /data/adb/modules/filemanager ] || exit 1
 [ ! -L "$fm_helper_module" ] && [ "$(readlink -f "$fm_helper_module")" = "$fm_helper_module" ] || exit 1
 [ "$(cat "$fm_helper_module/filemanager-origin" 2>/dev/null)" = QiSun8023/FileManager ] || exit 1
 fm_helper_data=/data/adb/dupeguard
 [ -d "$fm_helper_data" ] && [ ! -L "$fm_helper_data" ] && [ "$(readlink -f "$fm_helper_data")" = "$fm_helper_data" ] || exit 1
 fm_helper_package=io.github.qisun8023.filemanager.helper
 fm_helper_apk="$fm_helper_module/companion.apk"
 [ -f "$fm_helper_apk" ] && [ ! -L "$fm_helper_apk" ] || exit 1
 fm_helper_expected=$(tr -d '\r\n' < "$fm_helper_module/companion.sha256")
 fm_helper_actual=$(sha256sum "$fm_helper_apk"); fm_helper_actual=${fm_helper_actual%% *}
 [ "$fm_helper_actual" = "$fm_helper_expected" ] || exit 1
 fm_helper_path=$(pm path "$fm_helper_package" 2>/dev/null | sed -n '1s/^package://p')
 fm_helper_current=''
 if [ -n "$fm_helper_path" ] && [ -f "$fm_helper_path" ]; then fm_helper_current=$(sha256sum "$fm_helper_path"); fm_helper_current=${fm_helper_current%% *}; fi
 if [ "$fm_helper_current" != "$fm_helper_expected" ]; then
   fm_helper_size=$(wc -c < "$fm_helper_apk" | tr -d '[:space:]')
   if ! cat "$fm_helper_apk" | pm install -r -S "$fm_helper_size" > "$fm_helper_data/helper-install.log" 2>&1; then
     printf '%s\n' '辅助应用安装失败，请查看 helper-install.log；不会自动卸载同包名应用。' > "$fm_helper_data/helper-status"
     exit 1
   fi
 fi
 fm_helper_path=$(pm path "$fm_helper_package" 2>/dev/null | sed -n '1s/^package://p')
 [ -n "$fm_helper_path" ] && [ -f "$fm_helper_path" ] || exit 1
 fm_helper_current=$(sha256sum "$fm_helper_path"); fm_helper_current=${fm_helper_current%% *}
 [ "$fm_helper_current" = "$fm_helper_expected" ] || exit 1
 printf '%s\n' "$fm_helper_expected" > "$fm_helper_data/helper-installed-sha256"
 printf '%s\n' '辅助应用已安装；首次使用需手动允许 Root、通知与悬浮窗。' > "$fm_helper_data/helper-status"
)
filemanager_start_monitor() (
 fm_watch_module=$1
 [ "$fm_watch_module" = /data/adb/modules/filemanager ] || exit 1
 fm_watch_data=/data/adb/dupeguard
 [ -d "$fm_watch_data" ] && [ ! -L "$fm_watch_data" ] && [ "$(readlink -f "$fm_watch_data")" = "$fm_watch_data" ] || exit 1
 [ ! -L "$fm_watch_data/watch-stop" ] || exit 1
 printf 1 > "$fm_watch_data/watch-stop" || exit 1
 fm_watch_wait=0
 while [ "$fm_watch_wait" -lt 20 ]; do
  fm_watch_pid=$(cat "$fm_watch_data/watch-daemon/pid" 2>/dev/null)
  case "$fm_watch_pid" in ''|*[!0-9]*|0) break ;; esac
  kill -0 "$fm_watch_pid" 2>/dev/null || break
  fm_watch_exe=$(readlink "/proc/$fm_watch_pid/exe" 2>/dev/null)
  case "$fm_watch_exe" in "$fm_watch_module/bin/filemanager"|"$fm_watch_module/bin/filemanager (deleted)"|/data/adb/ksu/hot_install_trash/filemanager.*/bin/filemanager) ;; *) break ;; esac
  sleep 1; fm_watch_wait=$((fm_watch_wait+1))
 done
 if [ "$fm_watch_wait" -ge 20 ]; then exit 1; fi
 rm -f -- "$fm_watch_data/watch-stop" || exit 1
 "$fm_watch_module/bin/filemanager" watch-start e30= >/dev/null 2>&1
)
