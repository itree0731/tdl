# 原版 TDL 的 Linux 视频封面轻量分支

本分支基于原版 TDL 的 `upstream/master`。命令名仍是 `tdl`，账号目录、备份格式及其余命令不变。没有 TMT GUI。

## 运行依赖与命令

Linux 环境需提供 `ffmpeg`；非 MP4 视频读取元信息时还需同一软件包中的 `ffprobe`。默认给视频自动选帧并生成高清封面与缩略图。封面时间不会成为视频播放起点。

```bash
tdl up -p clip.mp4
tdl up -p clip.mp4 --cover-at 12s
tdl forward --mode clone --from https://t.me/example/123 --to destination
tdl forward --mode clone --from exported.json --to destination --cover-at 12s
```

普通上传和克隆转发都可用 `--video-cover=false` 关闭自动封面。克隆模式的视频始终下载后重新上传；关闭封面只取消封面生成。直接转发成功时沿用原版 Telegram 转发行为。`--dry-run` 不下载、不上传、不生成封面。

## 另外移植的修复

- A：单项失败不阻止其他工作项；批量结束时返回可识别的非零错误。
- B：下载文件关闭、重命名及必要后处理成功后才登记完成；失败时保留可信续传记录及部分文件供检查。没有字节级续传承诺。
- D：文件已上传但 `--rm` 删除源文件失败时，明确报告“上传成功、删除失败”。

## 验证

`scripts/verify-linux-cover.sh` 运行 root（排除需要外部 Teamgram 服务的测试包）、core、extension 的测试与 vet，以及相关 race 检查，并构建 Linux 可执行文件。`scripts/e2e-video-cover.sh` 用临时账号副本向该账号的 Saved Messages 上传、克隆一段带测试标记的视频，再检查服务器返回的封面与播放时间；该测试会产生两条真实消息。

在当前可用的 `itree` 账号上，上传与克隆产生了不同的 Telegram 文件 ID，缩略图为 320×180，播放时间戳为零，但服务端没有保留高清 `VideoCover` 字段。用现有 TMT 在同一账号做差分上传也得到相同结果。该账号仍可见视频缩略图；高清 `VideoCover` 的服务端保留情况需要在其他有效账号上继续验证。旧版 `default` 账号的会话副本目前未获 Telegram 授权，没有用它声称通过。
