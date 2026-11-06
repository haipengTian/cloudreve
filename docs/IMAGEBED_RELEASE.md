# ImageBed 个人镜像

首版镜像：`ghcr.io/haipengtian/cloudreve:4.19.1-custom.1`。服务器安装和更新见 [server-ops/cloudreve](https://github.com/haipengTian/server-ops/tree/main/cloudreve)。本次目标为 `cloud.thpvip.xyz`，首次安装、全新数据，不迁移 Windows 试用数据。

## 内容

- Cloudreve Community 4.19.1，后端基线 `a8becb9f5b226024c83230bc52857c04966e2c30`。
- 前端固定 `19da0fe1ecd40971fafa813983d769fdce41573c`，应用 `tools/frontend/imagebed.patch`；不使用本机有其他改动的 `assets` 子模块。
- ImageBed 统一私密存取，使用图床默认存储渠道；原普通存储策略保持原路径。
- 新上传物理名 UUID + 安全原后缀；保留旧无后缀实体和进行中会话的路径。
- 官方 `cloudreve/cloudreve:4.19.1` 运行环境，保留字体、缩略图和媒体工具。此镜像仍使用原前端版本标识 `4.19.1`，个人版本和源码提交记录在 OCI 标签及后端 LastCommit 中。

## 发布

`Dockerfile.imagebed` 从源码构建前后端，不包含 `.local/`、配置、数据库、Token 或用户文件。保留 GPLv3 LICENSE；对应源代码、前端补丁和构建步骤都在个人 fork 的版本标签中。

分支 `codex/imagebed-integration` 的 Actions 运行只构建与检查。发布标签 `4.19.1-custom.N` 才在检查成功后推送固定版本 GHCR 镜像，不更新 `latest`。

检查包括前端 2 项、后端 DBFS/驱动/内容代理/策略四包测试，以及 amd64 容器注册、上传、下载字节和 Range 验证；移除容器后挂回同一数据卷，重新登录并读取原文件。最终构建 `linux/amd64` 和 `linux/arm64`，arm64 未进行运行时烟测。

本机 Docker 引擎未运行，实际容器检查由 GitHub Actions 执行。发布完成以对应标签的工作流成功和可匿名拉取的 GHCR 清单为准。

HTML 隔离交互预览、Office 服务和真实 Telegram 端到端验收不包含在镜像发布检查内；现有真实上传及源码回读记录见 [IMAGEBED_INTEGRATION.md](IMAGEBED_INTEGRATION.md)。
