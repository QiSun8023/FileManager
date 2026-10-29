## v1.7.2 修正

底部四个功能导航始终显示。重复文件、修复、回收站的文件操作栏在未选择时隐藏，选中后显示，取消全部后再次隐藏。保留跨页选择，自动调整底部留白。核心文件处理、数据路径、安装与卸载清理逻辑不变。

## v1.7.1 更新

扫描结果只保留跨页全选；扫描选项和完整文件信息按需展开。目录选择和明暗模式使用居中悬浮窗口。四个功能的底部导航始终可见；文件操作栏仅在有选择时显示，列表底部自动预留操作栏与导航所需空间。修复页仍保留本页/全选。

新增 uninstall.sh。按用户明确选择，卸载将永久删除回收站中的实际文件及恢复记录，并清理私有索引、修复历史、选择、日志和旧模块备份。请先恢复需要的文件；正常目录中的原文件不清理。删除范围仅限本模块私有目录和两个新旧回收站目录，共享父目录有其他文件时保留。活动任务、异常路径或不可用存储会使清理报错并保留无法安全处理的数据；真实手机卸载时机和权限尚未验证。

# 文件管理

基于 Root WebUI 的本地文件工具，支持内容查重、文件日期/双后缀修复、跨页批量操作和回收站。v1.6.0 完成系统设置式 WebUI；v1.7.0 将内部模块标识改为 **filemanager**。

## 下载

在 [Releases](https://github.com/QiSun8023/FileManager/releases) 下载 `FileManager-v1.7.2-arm64.zip`。源码 ZIP 和独立 WebUI ZIP 不能作为模块安装。ARM64、Android 8.0+，通过已开机的 KernelSU 管理器安装。

## 从旧标识迁移

1. 停止旧版所有任务，关闭旧 WebUI，不要先卸载旧模块。
2. 手动安装一次新标识 v1.7.0 安装包。内部标识变化按一次迁移处理，不用旧标识的更新清单直接推送不同 ID 的 ZIP。
3. 新版后端和 WebUI 检查通过后，安装流程在共用操作锁保护下，将 `/data/adb/modules/dupeguard` 移到 `/data/adb/dupeguard/legacy-module-dupeguard` 留作备份。刷新模块列表，使用新标识 filemanager 的入口。
4. 原记录、索引、锁及安装日志继续使用 `/data/adb/dupeguard/`；这是为兼容保留的私有数据路径，不是新模块 ID。无需复制、合并或重新解释记录。回收站仍为 `/storage/emulated/0/Android/FileManager/Trash/`。
5. 新标识后续使用 `update-filemanager.json` 检查更新；旧标识 `update.json` 保留在 v1.6.0。新安装 ZIP 的 module.prop 已配置新清单地址。

存在运行任务、旧版待安装目录、路径链接、身份不匹配或旧备份冲突时，拒绝迁移旧模块。无法确认迁移成功时，查看私有目录 install-status / install.log；不要手动删除记录。已退休的旧模块完整留在备份目录，不自动清理。新宿主若按模块 ID 隔离网页存储，界面外观和最近目录偏好可能需要重新选择。

本次不是移动私有数据目录，旧私有路径仍然保留；模块目录和程序入口均已改成 filemanager。核心 Go 文件处理代码保持不变，仅同步版本常量与 Go 模块名称。

## 源码结构

- `engine/`：Go 后端和功能测试。
- `module/`：模块配置、免重启/迁移安装脚本、WebUI 和许可。构建后生成 `module/bin/filemanager`，不提交该程序。
- `tests/`：WebUI 桥接、主题、安装和旧模块退休模拟。
- `tools/`：构建、版本同步工具。
- `update.json`：旧标识 dupeguard 渠道，固定 v1.6.0。
- `update-filemanager.json`：新标识 filemanager 渠道。

## 构建与发布

需要 Python 3.10+、Go 1.24+、Node.js 和 Bash；Windows 可使用 Git Bash。

```powershell
./build.ps1 -Bash 'C:/Program Files/Git/bin/bash.exe'
```

Linux：`python3 tools/build.py`。工具会测试当前主机上的后端、桥接、主题、安装及迁移模拟，编译 Android ARM64 程序，输出安装 ZIP、独立版本清单和校验值到 `dist/`。构建按 module.prop 生成发布清单，不要求 main 上的在线清单已经切换到待发布版本。

后续版本例：`python tools/set_version.py v1.7.1 10701`。更新 CHANGELOG.md，构建后先发布标签和 Release 附件，确认公开下载有效，再将新 `update-filemanager.json` 上传 main。不要改动旧 `update.json` 的渠道归属。

[详细功能说明](module/README.md)。MIT，Go 许可见 module/GO-LICENSE。

尚未经过 Android 真机验证。本机测试及安装模拟不代表具体设备上的免重启、迁移、存储权限或 Monet 已验证成功。

### 卸载补充

KernelSU 实际执行卸载脚本的时机由管理器决定，不能将“点击卸载”视为清理已经完成。共享存储未解锁时，会将清理助手暂存到私有目录并后台等待，最多等待约 30 分钟；解锁后继续清理，成功时连助手本身一并删除。等待超时、权限不足或异常操作锁时保留未完成的数据并输出错误，日志为 `/data/adb/dupeguard/uninstall-cleanup.log`。等待期间重新安装会变更安装标识，使旧清理任务停止。

不会删除 Downloads、录音原目录、其他模块或宿主应用数据；WebUI 宿主自己隔离保存的网页缓存/偏好由宿主管理，不会为了清理本模块而清空整个宿主。旧版没有卸载脚本，须先升级本版才具备此清理功能。

参考：[KernelSU 卸载实现](https://github.com/tiann/KernelSU/blob/main/userspace/ksud/src/module.rs)、[Android CE 存储准备标志](https://android.googlesource.com/platform/frameworks/base/+/android16-release/services/core/java/com/android/server/pm/UserDataPreparer.java)。
