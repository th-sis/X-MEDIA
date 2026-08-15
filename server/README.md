# X-MEDIA 后端（server）

X-MEDIA 的 Go 单二进制后端，基于 [LitePan](https://github.com/Ponphil/LitePan)（PolyForm Noncommercial 1.0.0 许可）改造，新增 TMDB 代理、媒体库、四层播放引擎、Ticket 流代理与 WebSocket。

## 构建与运行

```bash
go build ./...
go run ./cmd/xmedia
```

- 默认监听 `:8080`，数据目录 `./data`（SQLite `xmedia.db`）
- 环境变量：`XMEDIA_LISTEN`、`XMEDIA_DATA_DIR`、`XMEDIA_DB_PATH`、`XMEDIA_LOG_LEVEL`
- 管理后台：`http://127.0.0.1:8080`（内嵌 Vue3，默认管理员密码 `admin`，首次启动可在后台修改）

## 目录

```
cmd/xmedia/       入口
internal/tmdb     TMDB 代理（含演示目录兜底）
internal/media    媒体库（历史/收藏/订阅/继续观看/搜索历史/可用性）
internal/resolve  四层播放引擎（P0 NAS → P1 盘搜 → P2 磁力 → P3 订阅）+ 并发限流
internal/playback Ticket 签名与 /api/stream 流代理
internal/websocket WebSocket Hub（health_check / resolve_* / capabilities）
internal/pansearch PanSou HTTP 客户端（Sidecar）
internal/indexengine 索引引擎（预留）
internal/store    SQLite 仓储（migrations 0001-0027）
drivers/          网盘驱动（115_Open/Quark/123_Open/Baidu_Open/Guangya/LocalFs…）
web/              Vue3 管理后台源码（构建产物内嵌于 internal/api/web）
```

## 演示模式

未配置 TMDB Key、网盘账号、PanSou 时，后端进入演示模式：TMDB 代理返回内置演示目录，`POST /api/resolve` 返回演示播放流（重定向到 `demo_video_url`，默认 Big Buck Bunny 示例视频）。

完整 API 契约见根目录 `README.md` 与设计文档 §12/§18。

## 许可与致谢

后端代码基于 LitePan，遵循其 [PolyForm Noncommercial 1.0.0](LICENSE) 许可；第三方依赖见 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。
