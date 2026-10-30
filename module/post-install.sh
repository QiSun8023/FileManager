#!/system/bin/sh
DG_POST_DIR=${0%/*}
. "$DG_POST_DIR/hot-install.sh" || exit 1
[ "$DG_POST_DIR" = /data/adb/modules/filemanager ] || exit 1
if filemanager_ready "$DG_POST_DIR" /data/adb/dupeguard v1.8.0; then
  echo "文件管理 v1.8.0 KernelSU 原生热安装完成，请刷新模块列表打开 WebUI。"
else
  filemanager_status /data/adb/dupeguard error "新版检查或旧模块退休未完成，原记录保留，请查看安装输出。"
  exit 1
fi
