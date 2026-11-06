# Cloudreve × ImageBed 本地集成

沿用 Cloudreve Community 4.19.1 原界面。Cloudreve 只连接 ImageBed 统一存储接口，不配置 Telegram/R2/S3。实际文件存储由 ImageBed 自身的默认上传设置决定，继续使用现有 Telegram，不迁移原图片、不占用 R2_env 本地对象空间。

## 使用

1. 用户升级 ImageBed 到支持 serverSideRouting 的版本；本轮镜像 ghcr.io/haipengtian/cloudflare-imgbed:2.7.7-custom.3 已发布，手动容器检查和tag工作流均成功（https://github.com/haipengTian/CloudFlare-ImgBed/actions/runs/42013131325），可以由用户运行服务器 update.sh。操作单在 server-ops/imgbed/PRIVATE_STORAGE_UPGRADE.md。服务器仍由用户操作，不通过源码包部署。
2. 正常停止本机 Cloudreve 窗口，在停机期间完整备份 .local/runtime/，再启动候选：

   ```powershell
   & 'E:\workspace\personal\code\cloudreve\tools\local\start.ps1' -ImageBed
   ```

   脚本将构建产物复制到 .local/runtime/cloudreve-imagebed.exe，与原配置/数据库同目录，不覆盖官方 cloudreve.exe。无 -ImageBed 时仍启动官方程序。5212占用时拒绝启动，不终止其他进程。
3. 在 http://localhost:5212 管理面板新增或编辑 ImageBed 策略：地址 https://img.thpvip.xyz，专用目录 cloudreve-test，API Token 由用户本机输入（upload/manage/delete），单文件上限100 MiB。无需选择上传渠道和渠道名称。旧测试策略保留，其历史渠道字段不再参与上传路由；已有对象继续按 ImageBed 自身的元数据读取。
4. 新策略仅用于测试账号/测试组，不改已有文件或普通用户组的策略，不自动导入已有图床对象。远端操作限 cloudreve-test/。

ImageBed 的默认类型/名称在其「网页设置 → 客户端设置」维护；未设置时使用 Telegram 和首个启用渠道。Telegram 频道或超级群不能有公开用户名，私人聊天允许用户名；频道成员范围由用户管理。此前 cfr2:R2_env 环境许可可以保留，但不会覆盖 Telegram 默认路由。

## 文件合同

- 上传/读取经 Cloudreve 服务端，Token 不用于浏览器文件直链；不跟随上游重定向，不转发用户 Cookie。
- 新上传物理路径为 `cloudreve-test/{uid}/{uuid}{ext}`，例如 `cloudreve-test/7/56b627d6-d4c7-4e32-b44b-e2563d599819.html`；显示名、整理、搜索由 Cloudreve 管理。创建后根目录不变，单实例独占目录。
- 后缀含点并保留原大小写，`照片.PNG` 对应 `UUID.PNG`；按最后一个点取扩展名，`archive.tar.gz` 对应 `UUID.gz`，无扩展名仍用 UUID。为保持既有安全路径限制与任意显示名可上传的合同，后缀只保留 ASCII 字母、数字、下划线、短横线；中文或特殊字符后缀、末尾孤立的点沿用 UUID-only，Cloudreve 原文件名不变。`Content-Type` 仍单独传递。
- 新版会对现有 ImageBed 策略的新上传生效，无须重建或重新保存策略。已有无后缀对象及进行中的上传会话继续按保存的原路径读写，不迁移、不重命名；不要手动改 ImageBed 对象名，否则会破坏 Cloudreve 的路径映射。此调整只需更新本机 Cloudreve，ImageBed custom.3 无须升级。
- 每次上传前要求 capability version:1/privateUpload:true/serverSideRouting:true。旧图床明确报错，不静默降为公开上传。
- Cloudreve 单请求中转、临时落盘验证字节数；稳定会话ID支持幂等重试，HEAD核对归属。ImageBed 可在内部将超过16 MiB的Telegram文件分片，不暴露给Cloudreve。
- Telegram 私密图片以document上传，保持原始字节；支持HEAD、完整GET、后缀/开区间/跨片Range。匿名直链拒绝，读取no-store。
- Telegram 删除沿用图床语义：删除元数据及读取入口，不撤回频道消息、不保证Telegram物理擦除。图床备份不包含Telegram文档。
- 暂不支持客户端分片直传、存储加密、公开CDN和已有对象导入。原普通存储策略合同不变。

## 构建

后端基线 a8becb9f5b226024c83230bc52857c04966e2c30，前端固定19da0fe1ecd40971fafa813983d769fdce41573c，通过源码归档和 tools/frontend/imagebed.patch 构建，不升级assets gitlink。

```powershell
& 'E:\workspace\personal\code\cloudreve\tools\local\build.ps1' -GoExecutable 'C:\Users\11030\go\pkg\mod\golang.org\toolchain@v0.0.1-go1.26.5.windows-amd64\bin\go.exe'
```

缓存和临时文件位于仓库.local/；Vite需6 GiB heap。输出.local/build/cloudreve-imagebed.exe，不自动替换正在运行的程序。旧package-lock不参与Yarn构建。

本地候选使用 `-buildvcs=false`，保留显式 `LastCommit` 标识。上游嵌入资源默认使用 Git 提交日期作为 `Last-Modified`，未提交的二开构建会沿用旧日期，导致 PWA 服务工作线程误判无需更新。此参数使本地候选使用实例启动时间，每次重启重新校验资源；不清除账号、策略或文件。

## 验证进度（2026-11-06）

- UUID + 原扩展名改动：先用旧实现运行新增路径测试，HTML/PNG/GZ 后缀断言失败；修改后 DBFS、ImageBed 驱动、管理策略、内容代理四包定向测试及前端 2 项测试通过，独立审查无阻塞问题。覆盖现有 `{uuid}` 策略、后缀大小写、无后缀、异常后缀及旧无后缀对象 HEAD/GET/删除（模拟服务，不操作用户文件）。实际生效仍需用户重启后上传新样例，并核对 ImageBed 名称；现有运行实例尚未替换。旧程序已保存在 `.local/backups/before-uuid-extension/cloudreve-imagebed.exe`，本次不改变数据库结构。
- 本次 UUID 后缀候选已完成 Vite/PWA 和 Go 主程序完整构建，输出 `.local/build/cloudreve-imagebed.exe`；启动方式不变，ImageBed 无须再次部署。
- 原本地存储8类文件、整理、下载及重启保留已验证，见LOCAL_TRIAL.md。
- 已实际进入 localhost 统一存储新版表单，策略 2 的地址、专用目录、Token 和 100 MiB 上限保留，上传渠道与渠道名称字段均已移除。截图：.local/imagebed-unified-policy.jpg。
- 本轮前端策略/上传重试2项测试通过；后端驱动、内容代理和管理策略三个包定向测试通过。
- ImageBed真实handler模拟边界6项通过，含小图及16 MiB+7字节跨片、字节比对、幂等、Range、鉴权和逻辑删除。未访问真实Telegram频道。
- 本轮 Vite 与主程序完整构建通过。本轮全量TypeScript检查候选仍127条诊断，与已有同依赖基线按文件/代码/消息对比无新增，不等于全量类型检查通过。
- 用户确认 ImageBed 已升级 custom.3。测试账号真实上传 8 类样例全部成功，HTML 源码已通过 Cloudreve 从 ImageBed 回读；底层路由沿用实例默认设置，尚未独立核对远端 Telegram 元数据。完整联调尚未完成。
- 00:26 本机已切换新候选，但浏览器入口仍为旧 `index-CkJeHBcs.js`；服务端首页实际返回新 `index-BZujK9d8.js`，`/sw.js` 的 `Last-Modified` 仍为 2026-09-18。普通刷新和释放页面后仍未更新。
- 已修复本地构建参数并复审通过，PowerShell 语法检查及 Go 重编通过；00:34 新产物为 .local/build/cloudreve-imagebed.exe（81,650,176 字节），读取构建信息确认无 `vcs.time`。用户 01:10 重启后，`/sw.js` 的 `Last-Modified` 已更新到本次启动时间，实际新 UI 已生效。
- 管理页面确认社区版每组仅能选一条存储策略，多选需 Pro。用户同意独立测试组/测试账号后，已从 User 模板新建组 4“ImageBed 测试组”，绑定策略 2，容量 1 GB，无管理员权限，关闭创建分享和获取直链；原 Admin/User 仍绑定本地默认策略。用户组列表已确认保存，截图：.local/imagebed-test-group.jpg。
- 用户已创建并登录 imagebed-test@example.com，容量显示 1 GB，并自行上传一份 HTML。已确认其 Cloudreve 显示名保留 `.html`，大小 424,701 字节；不对用户文件改名、移动或删除。
- 新建“ImageBed 联调”目录，上传 `.local/fixtures` 的 PNG/PDF/WAV/MP4/Markdown/JSON/HTML/ZIP 共 8 个样例，队列均显示已上传。HTML 样例 194 字节，Monaco 已显示其中文标题与按钮源码。HTML 仍没有隔离交互查看器，不能将源码回读称为网页渲染成功。
- 浏览器下载事件等待超时，点击下载后未取得可验证的下载产物，下载名称、字节比对和实际响应类型尚未验收。未把用户 HTML 交给 Google Docs 或 Office 公共预览服务；其余预览、Range、删除和重启检查仍待完成。

继续在cloudreve-test/完成预览、视频跳转、整理、下载、匿名拒绝、删除和重启保留验证。HTML隔离交互预览未实现，Office预览未配置，仍是后续独立工作。

回退时正常停止候选，用原脚本启动官方程序；恢复数据库使用完整停机备份，不删库试错。图床恢复按操作单处理，新私密数据不交给不支持该协议的旧镜像。
