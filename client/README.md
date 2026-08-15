# X-MEDIA 客户端（client）

Kodi 风格 10 英尺 TV 界面的 Flutter 桌面客户端。

## 运行 / 编译

```bash
flutter pub get
flutter run -d windows            # 开发运行
flutter build windows --release   # 编译 exe
# 产物：build/windows/x64/runner/Release/xmedia_client.exe
```

> Windows 桌面编译需要 Visual Studio 2022「使用 C++ 的桌面开发」工作负载。

## 结构

```
lib/
├── main.dart                入口
├── theme/app_theme.dart     Kodi 暗色主题 + 设计令牌
├── models/media.dart        数据模型
├── services/                API 客户端 / WebSocket / AppState
├── widgets/
│   ├── kodi_shell.dart      主壳（左侧栏 + 内容 + 背景 + 时钟）
│   ├── focus.dart           焦点导航（方向键）+ 焦点容器
│   ├── poster_card.dart     海报卡片（渐变占位 + 焦点高亮）
│   └── poster_wall.dart     横向榜单 / 网格
└── pages/                   首页/搜索/分类/详情/播放器/历史/订阅/设置
```

## 操作

- 鼠标：悬停高亮、点击确认
- 键盘：方向键导航、回车/空格确认、Esc 回退

默认后端地址 `http://127.0.0.1:8080`，可在「设置」页修改。
