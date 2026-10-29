# 文件管理

用于 KernelSU / 兼容 Root WebUI 宿主的 ARM64 Android 文件工具。内部模块 ID 固定为 `dupeguard`，显示名称为“文件管理”。

## 功能

- 浏览选择目录，按大小和完整 SHA-256 查重；建议保留无编号原文件，支持手动调整。
- 重复文件、文件修复和回收站均支持跨页选择与取消。
- 按文件名日期修正修改时间，去除 `.m4a.aac` 中的 `.m4a`。
- 回收站位于 `/storage/emulated/0/Android/FileManager/Trash/`，支持恢复及彻底删除。
- 六种强调色、明暗模式和宿主 Monet 动态配色。
- 保留 KernelSU 原生/延后热安装流程，无 `action.sh`。

适用范围：ARM64、Android 8.0 及以上。尚未进行 Android 真机验证，具体系统权限、免重启流程和 Monet 支持须真机确认。功能测试和安装模拟不等于真机验证。

## 下载和安装

从 [Releases](https://github.com/QiSun8023/FileManager/releases) 下载 `FileManager-v版本号-arm64.zip`，在 KernelSU 中安装。

**GitHub 自动生成的 Source code ZIP 及本仓库整体压缩包均不是手机安装包**：源码中不提交编译后的后端程序。安装后查看详细说明：[使用说明](module/README.md)。

旧版没有更新地址，需要先手动覆盖安装 v1.5.1 一次。以后 KernelSU 读取模块内的 `updateJson`，发现更大的 `versionCode` 时提供更新。仓库须公开，GitHub 原始文件和 Release 下载地址须可访问；这不需要额外的开机服务或 `action.sh`。

## 哪些文件需要保留

| 路径 | 用途 | 手机安装时是否使用 |
| --- | --- | --- |
| `engine/`（不含 `*_test.go`） | Go 后端源码，构建为 `bin/dupeguard` | 使用编译结果，不直接运行源码 |
| `engine/*_test.go`、`tests/` | 功能、安装流程和界面桥接测试 | 否，建议仓库保留 |
| `module/` | 模块元信息、免重启安装脚本、WebUI 和许可文件 | 是，目录内容进入安装包根目录 |
| `tools/`、`build.ps1` | 同步版本、测试、编译与打包 | 否，建议仓库保留 |
| `update.json` | KernelSU 在线版本清单 | 从 GitHub 读取，不放入模块内 |
| `CHANGELOG.md` | 更新内容，更新提示读取标签对应版本 | 从 GitHub 读取 |
| `README.md`、`LICENSE`、`.gitignore`、`.gitattributes` | 项目说明、许可、忽略构建产物、保持 LF 换行 | 仓库管理文件 |

`alive_unix.go` / `alive_windows.go`、`repair_linux.go` / `repair_windows.go` 由 Go 按系统选择，属于不同平台的实现，并非重复文件。保留它们才能同时进行 Windows 测试和 Android 构建。

`module/update.sh` 是**免重启安装辅助脚本**，根目录 `update.json` 是**在线版本清单**；两者用途不同，不要删除安装脚本。`module/GO-LICENSE` 是后端所含 Go 标准库的许可，随安装包保留。

工具链、Go 缓存、测试临时目录、旧安装包和截图均无需上传源码仓库；安装 ZIP 放在 Releases。当前仓库包已排除这些文件。

## 首次上传与发布 v1.5.1

1. 在 GitHub 创建公开仓库 **QiSun8023/FileManager**，默认分支选择 **main**。
2. 解压提供的仓库包，上传其中的内容到仓库根目录。应直接看到 `README.md`、`update.json`、`engine/` 和 `module/`，不要再套一层 FileManager 目录。
3. 在 Releases 创建新发布，标签填写 **v1.5.1**，目标为包含全部源码的提交；上传 **FileManager-v1.5.1-arm64.zip** 作为附件，说明可复制 `CHANGELOG.md`。
4. 发布为正常公开 Release；检查下面三个地址均能打开。标签、附件名、JSON 大小写必须一致。不要把 Source code ZIP 填入 `zipUrl`。
5. 手机手动覆盖安装一次该 v1.5.1 安装包。此时远端也是同版本，不显示新版本提示属于正常情况；需要后续发布更高版本才能验证更新发现。

已配置的地址：

- 更新清单：`https://raw.githubusercontent.com/QiSun8023/FileManager/main/update.json`
- 本版安装包：`https://github.com/QiSun8023/FileManager/releases/download/v1.5.1/FileManager-v1.5.1-arm64.zip`
- 本版说明：`https://raw.githubusercontent.com/QiSun8023/FileManager/v1.5.1/CHANGELOG.md`

仅上传源码不会产生 Release 附件；仍需按上述步骤发布安装附件。

## 本地构建

需要 Python 3.10+、Go 1.24+、Node.js 及 Bash（Windows 可用 Git Bash）。无第三方 Go 依赖，构建脚本不自动下载安装工具。

```powershell
# Windows（Go / Python / Node.js 已在 PATH）
./build.ps1 -Bash 'C:/Program Files/Git/bin/bash.exe'
```

```sh
# Linux
python3 tools/build.py
```

工具不在 PATH 时，可向 `tools/build.py` 传 `--go`、`--node`、`--bash` 完整路径；Windows 的 `build.ps1` 还支持 `-Python`。工具会先运行当前电脑上的测试与安装模拟，再构建 Android ARM64 程序，并输出 `dist/FileManager-v版本号-arm64.zip`、清单副本、说明及 SHA-256。发布附件仅需安装 ZIP，校验文件可同时上传。不要将 `dist/` 或 `module/bin/` 提交到源码仓库。

## 后续发版（例如 v1.5.2）

1. 完成功能修改后执行 `python tools/set_version.py v1.5.2 10502`，同步后端、前端、安装脚本、测试常量和 `update.json`。版本码必须递增；不要改模块 ID。
2. 编辑 `CHANGELOG.md`，填写本次变更，然后运行构建脚本并核对结果。
3. 为避免先公布清单却尚无下载文件，先提交新版本源码但暂不提交新 `update.json`；对这次提交创建 `v1.5.2` 标签，发布 Release 并上传 `dist/FileManager-v1.5.2-arm64.zip`。
4. 确认 Release 附件和标签下的 CHANGELOG 可访问，再将新的根目录 `update.json` 提交到 `main`。之后 KernelSU 才会发现更新。

这些步骤由维护者执行，不会自动发布。更新清单指向固定版本的 ZIP 与更新说明，避免升级时取错版本。

## 许可与参考

MIT，见 [LICENSE](LICENSE)；Go 许可见 [module/GO-LICENSE](module/GO-LICENSE)。
在线更新字段参考 [KernelSU 模块指南](https://kernelsu.org/guide/module.html) 和 [发布文档](https://developers.kernelsu.org/docs/releasing)。
