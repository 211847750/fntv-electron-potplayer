# fntv-electron-potplayer

本项目基于 [QiaoKes/fntv-electron](https://github.com/QiaoKes/fntv-electron) fork，主要增加 Windows PotPlayer 外置播放器支持。

本项目不是 QiaoKes/fntv-electron 官方版本，也与飞牛影视官方无关。上游项目采用 GPL-3.0 许可证，本 fork 保留原许可证、原作者声明和上游来源。

## 项目定位

- 上游仓库：[QiaoKes/fntv-electron](https://github.com/QiaoKes/fntv-electron)
- Fork 仓库：[myczh-1/fntv-electron-potplayer](https://github.com/myczh-1/fntv-electron-potplayer)
- 当前重点：Windows PotPlayer 播放、进度读取、进度回传、记录位置恢复
- 发布包名：`FNMedia-PotPlayer_${version}_${os}_${arch}.${ext}`
- 当前版本：`0.3.6`
- License：[GPL-3.0](LICENSE)

这个 fork 主要用于 PotPlayer 支持实验与自用增强，欢迎反馈，但不承诺长期维护或持续跟进上游版本。

## PotPlayer 支持

当前 PotPlayer 支持仅面向 Windows：

- 调起 PotPlayer 播放飞牛影视视频；
- 通过内置虚拟盘向 PotPlayer 暴露视频文件；
- 自动下载飞牛外置字幕，并以同目录同名字幕文件暴露给 PotPlayer；
- 启动时按飞牛记录恢复播放位置；
- 每秒读取 PotPlayer 当前进度、总时长、播放状态；
- 将播放进度回传飞牛；
- 退出 PotPlayer 时保存最后播放进度。

实现方式：

- Electron 侧只负责飞牛业务、proxy URL、字幕下载和进度回传；
- PotPlayer 原生控制由内置 `potbridge.exe` 负责；
- `potbridge.exe` 会挂载只读虚拟盘，默认盘符为 `P:`；
- 虚拟盘中的视频文件背后通过 HTTP Range 读取，字幕文件背后读取本地已下载字幕；
- `potbridge.exe` 使用 Windows `EnumWindows` / `GetClassName` 绑定 `PotPlayer64` 或 `PotPlayer` 窗口；
- 进度读取依赖 PotPlayer 的 Windows 消息接口；
- 初始 seek 使用启动参数，并在 bridge 绑定窗口后通过消息二次 seek 兜底。

## 当前限制

- 仅验证 Windows 环境；
- 需要本机安装 PotPlayer；
- Windows 安装包会自动安装 WinFsp 运行时，用于挂载只读虚拟盘；
- 主要验证 PotPlayer 64 位版本；
- 默认使用 `P:` 作为临时虚拟盘盘符，如果本机已占用该盘符，虚拟盘挂载会失败；
- 依赖 PotPlayer 窗口类名和 Windows 消息接口，不保证所有 PotPlayer 版本行为一致；
- 字幕通过虚拟盘同名文件自动匹配，播放过程中动态切换字幕不保证支持；
- MPV 的弹幕、anime4K、脚本扩展等能力不会自动迁移到 PotPlayer；
- macOS / Linux 仍建议使用 MPV。

## 安装

前往本仓库的 [Releases](https://github.com/myczh-1/fntv-electron-potplayer/releases) 下载：

```text
FNMedia-PotPlayer_${version}_${os}_${arch}.${ext}
```

Windows 使用步骤：

1. 安装 PotPlayer。
2. 安装并启动 `FNMedia PotPlayer`。
3. 安装器会自动安装 WinFsp 运行时；如果系统已安装 WinFsp，会跳过。
4. 右键系统托盘图标。
5. 进入 `设置`，选择 `播放器: PotPlayer (Windows)`。
6. 如果程序没有自动找到 PotPlayer，点击 `设置PotPlayer路径`，选择 PotPlayer 可执行文件。
7. 回到飞牛影视页面点击播放按钮。

常见 PotPlayer 路径：

```text
C:\Program Files\DAUM\PotPlayer\PotPlayerMini64.exe
C:\Program Files\DAUM\PotPlayer\PotPlayer64.exe
C:\Program Files (x86)\DAUM\PotPlayer\PotPlayerMini.exe
```

## 从源码构建

```bash
git clone git@github.com:myczh-1/fntv-electron-potplayer.git
cd fntv-electron-potplayer
npm install
```

构建 Windows 测试包：

```bash
npm run build:win:test
```

输出文件：

```text
release/FNMedia-PotPlayer_0.3.6_win_x64.exe
```

常用命令：

```bash
npm run compile
npm run prepare:winfsp
npm run build:potbridge:win
npm run build:win
```

### 验证虚拟盘读取

播放时可以在应用日志中搜索 `[VFS]`：

```text
[VFS] mounted drive=P: entries=...
[VFS] read name="..." offset=... length=... request=... totalMiB=...
```

`read` 行表示 PotPlayer 通过虚拟盘发起的文件读取。当前实现会把每次文件读取转换为对应的 HTTP Range 请求；不会向 PotPlayer 暴露原始 HTTP URL，也不会主动发起 `bytes=0-` 这种开放范围整片请求。

## 上游能力

本 fork 继承上游 fntv-electron 的主要能力，包括：

- 基于飞牛影视 Web 端的桌面客户端体验；
- 多账户管理；
- FN Connect 远程访问；
- MPV 播放；
- 本地代理和 NAS 代理模式；
- MPV 进度回传；
- MPV 弹幕、anime4K、智能跳过等能力。

具体实现和历史说明请参考上游项目：[QiaoKes/fntv-electron](https://github.com/QiaoKes/fntv-electron)。

## 致谢

- [QiaoKes/fntv-electron](https://github.com/QiaoKes/fntv-electron) - 本 fork 的上游项目
- [fntv-mpv-config](https://github.com/QiaoKes/fntv-mpv-config) - MPV 配置
- [fnToPotplayer](https://github.com/gudqs7/fnToPotplayer) - 飞牛影视调用 PotPlayer 相关参考
- [fnos-tv](https://github.com/thshu/fnos-tv) - 飞牛影视桌面端相关参考
- [enable-chromium-hevc-hardware-decoding](https://github.com/StaZhu/enable-chromium-hevc-hardware-decoding)
- [electron-media-patch](https://github.com/5rahim/electron-media-patch)
- [uosc_danmaku](https://github.com/Tony15246/uosc_danmaku)

## 许可证

本项目采用 [GPL-3.0](LICENSE) 许可证。

Copyright (c) 2025 Tag mig hånden

本 fork 基于 QiaoKes/fntv-electron 修改，保留原项目许可证和版权声明。修改版源码与二进制分发应继续遵守 GPL-3.0。

## 免责声明

本项目为第三方 fork，与飞牛影视官方无关，也不是 QiaoKes/fntv-electron 官方发布版。使用前请确保遵守相关服务条款。
