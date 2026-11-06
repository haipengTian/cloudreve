# 本机试用 Cloudreve

在 PowerShell 中运行：

```powershell
& 'E:\workspace\personal\code\cloudreve\tools\local\start.ps1'
```

保持窗口打开，然后访问 <http://127.0.0.1:5212>。首次注册的账号是管理员；请使用仅用于本次试用的密码。
按 `Ctrl+C` 停止，文件和数据库仍保留，下次运行同一脚本即可继续。

## 当前准备状态

- 基线：官方社区版 **4.19.1**，源提交 `a8becb9f5b226024c83230bc52857c04966e2c30`。
- 源码：`E:\workspace\personal\code\cloudreve`，本地分支 `codex/imagebed-integration`。
- 官方 Windows amd64 程序：`.local/runtime/cloudreve.exe`。
- 启动脚本将监听地址限制为 `127.0.0.1:5212`，不向局域网开放，也不注册开机启动。
- 本机 Docker 引擎未运行，因此本次采用官方原生程序。
- 用户已手动启动并注册管理员；已在浏览器完成下述基础流程验证。
- 当前仍是本地存储试用，**没有接入 ImageBed**。

程序、运行数据和样例位于 Git 忽略目录 `.local/`，请勿将其中的数据库、配置或个人文件提交到仓库。
如需备份，先正常停止程序，再完整复制 `.local/runtime/`，不要只复制数据库。

官方程序来源：

<https://github.com/cloudreve/cloudreve/releases/download/4.19.1/cloudreve_4.19.1_windows_amd64.zip>

如果换机后程序不存在，将这个官方压缩包解压到 `.local/runtime/`，再运行启动脚本。

## 验证一条完整的收纳流程

样例文件在 `E:\workspace\personal\code\cloudreve\.local\fixtures`，包括 PNG、PDF、WAV、MP4、Markdown、JSON、HTML 和 ZIP。
其中 MP4 来自 W3C 托管的 Big Buck Bunny 预告片：<https://media.w3.org/2010/05/bunny/trailer.mp4>；其他样例为本机生成的测试内容。

1. 注册并进入文件管理页，新建“杂物库验证”文件夹。
2. 上传样例文件，检查中文名称和文件大小。
3. 打开图片、PDF、Markdown、JSON，确认真实内容能够展示；播放 WAV、MP4，并跳转视频进度。
4. 改名、移动文件，按中文文件名搜索，确认仍能找到并打开。
5. 下载文件，逐字节比较下载内容与原样例，而不是只检查请求成功。
6. 查看 ZIP 内容，确认能列出其中的文件。
7. 打开 HTML，记录默认打开方式；若不能安全运行交互页面，将其列为后续二开项。
8. 正常停止并重新启动程序，确认账号、文件夹和样例文件保留。

Office 预览尚未验收。私密文档预览优先使用自建文档服务，不向公共预览服务发送测试外的资料。

## 2026-11-04 浏览器实测结果

| 项目 | 结果 |
| --- | --- |
| 上传 | 8 个样例全部上传，中文名称正常 |
| 图片 | PNG 缩略图和完整图片正常显示 |
| PDF | 内置 PDF.js 展示 1 页正文 `Cloudreve Preview Check` |
| Markdown | 标题、中文列表、粗体和 JSON 代码块正常渲染 |
| JSON | Monaco 展示原 JSON 内容并高亮 |
| 音频 | WAV 成功加载，1 秒音频播放进度前进，无媒体错误 |
| 视频 | 33 秒 MP4 显示真实画面，可播放并跳转至约 21 秒处，无媒体错误 |
| ZIP | 列出中文 `欢迎使用.md` 和 `example.json`，大小与原文件一致 |
| 整理和搜索 | `example.json` 改名为 `示例数据.json`，移动至 `已整理` 子目录后，中文搜索仍能找到 |
| 下载 | 浏览器下载 PNG 及移动后的 JSON；与原文件逐字节一致，分别为 22,106 和 103 字节 |
| 页面刷新 | 账号会话、目录及文件仍保留 |
| 服务重启 | 用户手动重启后重新加载页面，登录会话、目录和全部 8 个样例仍保留；移动后的 JSON 可重新打开且内容正常 |
| HTML 交互预览 | 默认候选只有 Google Docs 阅读器和 Monaco，不能直接运行交互页面；需要新增隔离查看器 |
| Office | 未测试、未配置自建文档服务 |

实测目录为“我的文件 / 杂物库验证”，JSON 位于其“已整理”子目录。
实际界面截图保存在 `.local/trial-overview.jpg`；重启后打开已整理 JSON 的截图为 `.local/restart-check.jpg`。

Markdown 编辑器仅打开查看、关闭时也出现了未保存更改确认；测试选择放弃更改，没有保存样例。

## ImageBed 候选接入

社区版基础流程通过后，已实现候选存储适配和配置入口。保留 Cloudreve 原界面，第一版不增加 AI 功能。构建、切换及联调说明见 [IMAGEBED_INTEGRATION.md](IMAGEBED_INTEGRATION.md)。运行中的官方试用仍未接入图床。

Cloudreve 的 WebDAV 是对外文件访问服务，不是外部存储驱动，不能直接将 ImageBed 的 `/dav/` 地址当作现成的存储策略配置。

已经定位的扩展点：

- `pkg/filemanager/driver/handler.go`：上传、删除、下载来源、上传会话、缩略图和能力声明等存储接口。
- `pkg/filemanager/manager/fs.go`：根据存储策略创建驱动。
- `inventory/types/types.go`：存储策略类型定义。
- 前端固定原 gitlink 的源码归档，并通过 `tools/frontend/imagebed.patch` 保存策略配置入口及上传重试修正；不会升级原子模块版本。

ImageBed 接入需要保持私密读取鉴权、文件标识映射、上传失败清理和删除语义一致。共用底层 S3 Bucket 不等于经过 ImageBed，也不代表两套文件元数据能自动同步。

用户已授权后续仅在 `https://img.thpvip.xyz` 的 `cloudreve-test/` 目录联调，并同意准备支持私密上传的升级材料；服务器升级仍由用户执行。凭据由用户在本机配置，不写入代码或聊天。当前没有向该实例上传、删除文件或修改设置。

参考：

- <https://docs.cloudreve.org/overview/quickstart>
- <https://docs.cloudreve.org/usage/file-apps>
- <https://docs.cloudreve.org/usage/wopi>
