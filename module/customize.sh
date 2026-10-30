#!/system/bin/sh
[ "$BOOTMODE" = true ] || abort "标识迁移需要在已开机的 KernelSU 管理器中安装。"
[ "$ARCH" = arm64 ] || abort "本版本仅支持 ARM64 Android 设备。"
[ "$API" -ge 26 ] || abort "需要 Android 8.0 或更高版本。"
set_perm "$MODPATH/bin/filemanager" 0 0 0755
for DG_SCRIPT in update.sh post-install.sh uninstall.sh service.sh companion-install.sh; do set_perm "$MODPATH/$DG_SCRIPT" 0 0 0755 || abort "设置脚本权限失败。"; done
ui_print "文件管理 v1.8.0 · KernelSU 免重启适配"
if [ "$BOOTMODE" != true ]; then
  ui_print "当前为离线安装，正常启动手机后才能打开模块。"
else
  [ ! -e /data/adb/modules_update/dupeguard ] && [ ! -L /data/adb/modules_update/dupeguard ] || abort "旧模块仍有待安装版本，请先完成旧版安装再迁移。"
  case "$MODPATH" in /data/adb/modules_update/filemanager|/data/adb/modules/filemanager) ;; *) abort "安装路径不符合预期。" ;; esac
  [ ! -L /data/adb/dupeguard ] || abort "数据目录包含链接，已取消。"
  mkdir -p /data/adb/dupeguard || abort "不能创建数据目录。"
  [ "$(readlink -f /data/adb/dupeguard)" = /data/adb/dupeguard ] || abort "数据目录路径异常。"
  set_perm /data/adb/dupeguard 0 0 0700 || abort "不能设置数据权限。"
  if [ -d /data/adb/dupeguard/lock ]; then
    DG_BUSY_PID=$(cat /data/adb/dupeguard/lock/pid 2>/dev/null)
    case "$DG_BUSY_PID" in ''|*[!0-9]*|0) abort "操作锁状态不明，请先在旧版完成一次操作后再安装。" ;; esac
    kill -0 "$DG_BUSY_PID" 2>/dev/null && abort "文件任务正在运行，请停止或等待完成后重新安装。"
  fi
  DG_INFO=$("$MODPATH/bin/filemanager" info e30=)
  printf '%s' "$DG_INFO" | grep -Fq '"version":"v1.8.0"' || abort "新版后端不能启动或版本不匹配。"
  . "$MODPATH/hot-install.sh" || abort "缺少安装辅助脚本。"
  filemanager_check_existing "$MODPATH" /data/adb/modules/filemanager || abort "已存在其他来源的同标识模块，已取消，避免覆盖。"
  DG_TOKEN=$(cat /proc/sys/kernel/random/uuid)
  case "$DG_TOKEN" in ''|*[!a-zA-Z0-9-]*) abort "无法创建安装标识。" ;; esac
  [ ! -L /data/adb/dupeguard/install-generation ] && printf '%s' "$DG_TOKEN" > /data/adb/dupeguard/install-generation || abort "不能更新安装标识。"
  printf '%s' "$DG_TOKEN" > "$MODPATH/.filemanager-install-token" || abort "不能保存安装标识。"
  if [ "$(filemanager_mode)" = native ]; then
    MODULE_HOT_INSTALL_REQUEST=true
    MODULE_HOT_RUN_SCRIPT=post-install.sh
    filemanager_status /data/adb/dupeguard waiting "已请求 KernelSU 原生热安装，等待安装器完成。"
    ui_print "检测到 KernelSU 原生热安装支持，已请求立即启用。"
  else
    DG_BB=''
    for DG_CANDIDATE in /data/adb/ksu/bin/busybox /data/adb/magisk/busybox /data/adb/ap/bin/busybox; do
      if [ -x "$DG_CANDIDATE" ]; then DG_BB=$DG_CANDIDATE; break; fi
    done
    [ -n "$DG_BB" ] || abort "找不到管理器 BusyBox，无法启动后台发布。"
    DG_INSTALLER_PID=$$
    DG_PARENT_PID=0
    DG_PARENT_CMD=''
    if [ -r "/proc/$PPID/cmdline" ]; then DG_PARENT_CMD=$(tr '\000' ' ' 2>/dev/null < "/proc/$PPID/cmdline"); fi
    case "$DG_PARENT_CMD" in *ksud*module*install*) DG_PARENT_PID=$PPID ;; esac
    filemanager_status /data/adb/dupeguard waiting "已排队，等待安装器退出后发布；请稍后刷新模块列表。"
    "$DG_BB" nohup "$DG_BB" setsid "$DG_BB" sh "$MODPATH/update.sh" "$DG_INSTALLER_PID" "$DG_PARENT_PID" "$MODPATH" "$DG_TOKEN" </dev/null >>/data/adb/dupeguard/install.log 2>&1 &
    ui_print "已启动延后热安装：安装器结束后自动发布程序及 WebUI。"
  fi
fi
ui_print "安装完成后稍等数秒，刷新 KernelSU 模块列表再打开 WebUI。"
ui_print "状态：/data/adb/dupeguard/install-status；日志：install.log。"
ui_print "保留原有回收站与恢复记录；安装后自动安装文件管理助手。首次使用请允许 Root、通知与悬浮窗。"

ui_print "标识已改为 filemanager；沿用原私有数据目录，回收站及修复记录保留。"

ui_print "卸载本模块会彻底删除回收站文件、恢复记录和私有历史，请先恢复需要的文件。"
