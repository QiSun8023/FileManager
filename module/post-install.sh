#!/system/bin/sh
DG_POST_DIR=${0%/*}
. "$DG_POST_DIR/hot-install.sh" || exit 1
[ "$DG_POST_DIR" = /data/adb/modules/dupeguard ] || exit 1
if dupeguard_ready "$DG_POST_DIR" /data/adb/dupeguard v1.5.1; then
  echo "文件管理 v1.5.1 KernelSU 原生热安装完成，请刷新模块列表打开 WebUI。"
else
  dupeguard_status /data/adb/dupeguard error "KernelSU 已切换模块，但程序或 WebUI 检查失败，请查看安装输出。"
  exit 1
fi
