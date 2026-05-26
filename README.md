# fntv-electron-potplayer

本项目基于 [QiaoKes/fntv-electron](https://github.com/QiaoKes/fntv-electron) fork，主要增加 Windows PotPlayer 外置播放器支持。

本项目不是 QiaoKes/fntv-electron 官方版本，也与飞牛影视官方无关。上游项目采用 GPL-3.0 许可证，本 fork 保留原许可证、原作者声明和上游来源。

## Fork 信息

- 上游仓库：[QiaoKes/fntv-electron](https://github.com/QiaoKes/fntv-electron)
- 当前 fork 重点：PotPlayer 播放、进度读取、进度回传、记录位置恢复
- License：[GPL-3.0](LICENSE)
- 发布包命名：`FNMedia-PotPlayer_${version}_${os}_${arch}.${ext}`
- 当前版本：`0.1.0`
- 维护状态：主要用于 PotPlayer 支持实验与自用增强，不保证与上游版本同步更新
- 检查更新：默认不再指向上游 release；如需启用，请在 `src/modules/updater/updateChecker.ts` 中配置你的 fork 仓库 owner/repo

## PotPlayer 支持范围

当前 PotPlayer 支持仅面向 Windows：

- 调起 PotPlayer 播放飞牛影视视频；
- 使用本地代理地址播放；
- 自动加载飞牛外置字幕；
- 启动时按飞牛记录恢复播放位置；
- 每秒读取 PotPlayer 当前进度、总时长、播放状态；
- 将播放进度回传飞牛；
- 退出 PotPlayer 时保存最后播放进度。

当前限制：

- 需要本机安装 PotPlayer；
- 主要按 `PotPlayer64` / `PotPlayer` 窗口类名绑定播放器窗口；
- 进度读取和初始 seek 依赖 PotPlayer 的 Windows 消息接口；
- 初始 seek 使用启动参数和 bridge 二次 seek 兜底，不保证所有 PotPlayer 版本行为一致；
- 仅验证 Windows 环境，macOS / Linux 仍建议使用 MPV；
- MPV 的弹幕、anime4K、脚本扩展等能力不会自动迁移到 PotPlayer。

## 原项目简介

飞牛影视桌面客户端，基于 Electron 构建，提供更好的桌面体验和增强功能。

<img src="resource/docs/switch.png" width="90%">
<img src="resource/docs/simple.png" width="90%">

[演示视频](https://www.bilibili.com/video/BV12dYXzhE6U/)

## ✨ 主要功能

- **原生桌面体验** - 基于飞牛影视Web端构建的桌面应用，提供类原生体验
- **多账户管理** - 支持自动登录，支持多账户管理，自由切换账户和服务器
- **远程访问** - 支持使用FN Connect，通过FN ID登录实现远程访问
- **硬解播放** - 使用MPV播放器，支持H264 / HEVC / VP9 / AV1等编码格式
- **直链播放** - 适配官方直链 / STRM播放，默认使用302重定向，可以在托盘处调整为nas代理模式
- **进度回传** - MPV播放器支持实时将进度回传到飞牛服务器
- **弹幕支持** - MPV播放器支持弹幕自动匹配加载，无法匹配时支持手动搜索
- **视频增强** - 内置anime4K着色器以及对应预设模式
- **智能跳过** - 可在MPV播放器界面设置。支持三种跳过片头片尾模式：章节检查，手动设置片头片尾，快捷键跳过固定时长
- **跨平台支持** - 支持windows、macos和linux

## 爱发电

<a href="https://afdian.com/a/qiaoke" target="_blank">
  <img src="resource/docs/support_aifadian.svg" alt="support_aifadian">
</a>

您的每一次 star ⭐ 和 赞助 🎁 都是我持续优化的动力。让我们一起维护这个用爱发电的项目！

## 赞助者

感谢这些来自爱发电的赞助者：

<!-- AFDIAN-ACTION:START -->

<a href="https://afdian.com/u/41cb933ef7ff11f0a9a952540025c377">
    <img src="https://pic1.afdiancdn.com/default/avatar/avatar-purple.png?imageView2/1/?imageView2/1/w/120/h/120" width="40" height="40" alt="爱发电用户_41cb9" title="爱发电用户_41cb9"/>
</a>
<a href="https://afdian.com/u/bf093216d1da11f085b852540025c377">
    <img src="https://pic1.afdiancdn.com/default/avatar/avatar-blue.png?imageView2/1/w/120/h/120" width="40" height="40" alt="爱发电用户_7kDX" title="爱发电用户_7kDX"/>
</a>
<a href="https://afdian.com/u/5251e4e2d1c611f0b83d52540025c377">
    <img src="https://pic1.afdiancdn.com/default/avatar/avatar-purple.png?imageView2/1/?imageView2/1/w/120/h/120" width="40" height="40" alt="爱发电用户_5251e" title="爱发电用户_5251e"/>
</a>
<a href="https://afdian.com/u/ec6015fcd0ca11ef8f8e52540025c377">
    <img src="https://pic1.afdiancdn.com/default/avatar/avatar-purple.png?imageView2/1/?imageView2/1/w/120/h/120" width="40" height="40" alt="爱发电用户_ec601" title="爱发电用户_ec601"/>
</a>
<a href="https://afdian.com/u/713bc866afc811f093a952540025c377">
    <img src="https://pic1.afdiancdn.com/default/avatar/avatar-purple.png?imageView2/1/w/120/h/120" width="40" height="40" alt="爱发电用户_7WxX" title="爱发电用户_7WxX"/>
</a>
<a href="https://afdian.com/u/f9548ae809f311ef805e52540025c377">
    <img src="https://pic1.afdiancdn.com/user/f9548ae809f311ef805e52540025c377/avatar/b585c02959db06c4ea614f6e94fba294_w279_h252_s65.jpg.gif?imageView2/1/w/120/h/120" width="40" height="40" alt="1" title="1"/>
</a>
<a href="https://afdian.com/u/4514cc8c9a8411f0992b52540025c377">
    <img src="https://pic1.afdiancdn.com/default/avatar/avatar-purple.png?imageView2/1/?imageView2/1/w/120/h/120" width="40" height="40" alt="爱发电用户_4514c" title="爱发电用户_4514c"/>
</a>
<a href="https://afdian.com/u/2685303096b611f0b4a652540025c377">
    <img src="https://pic1.afdiancdn.com/default/avatar/avatar-purple.png?imageView2/1/?imageView2/1/w/120/h/120" width="40" height="40" alt="嬴游仙人莫迪" title="嬴游仙人莫迪"/>
</a>
<a href="https://afdian.com/u/8a03268e8ba411f0bcbb52540025c377">
    <img src="https://pic1.afdiancdn.com/default/avatar/avatar-orange.png?imageView2/1/w/120/h/120" width="40" height="40" alt="爱发电用户_e6g3" title="爱发电用户_e6g3"/>
</a>

<details>
  <summary>点我 打开/关闭 赞助者列表</summary>

<a href="https://afdian.com/u/41cb933ef7ff11f0a9a952540025c377">
爱发电用户_41cb9
</a>
<span>( 1 次赞助, 共 ￥10 ) 留言: </span><br>
<a href="https://afdian.com/u/bf093216d1da11f085b852540025c377">
爱发电用户_7kDX
</a>
<span>( 1 次赞助, 共 ￥10 ) 留言: </span><br>
<a href="https://afdian.com/u/5251e4e2d1c611f0b83d52540025c377">
爱发电用户_5251e
</a>
<span>( 1 次赞助, 共 ￥20 ) 留言: </span><br>
<a href="https://afdian.com/u/ec6015fcd0ca11ef8f8e52540025c377">
爱发电用户_ec601
</a>
<span>( 1 次赞助, 共 ￥10 ) 留言: 不错不错</span><br>
<a href="https://afdian.com/u/713bc866afc811f093a952540025c377">
爱发电用户_7WxX
</a>
<span>( 1 次赞助, 共 ￥10 ) 留言: 加油，支持一杯蜜雪</span><br>
<a href="https://afdian.com/u/f9548ae809f311ef805e52540025c377">
1
</a>
<span>( 1 次赞助, 共 ￥10 ) 留言: fntv</span><br>
<a href="https://afdian.com/u/4514cc8c9a8411f0992b52540025c377">
爱发电用户_4514c
</a>
<span>( 1 次赞助, 共 ￥10 ) 留言: </span><br>
<a href="https://afdian.com/u/2685303096b611f0b4a652540025c377">
嬴游仙人莫迪
</a>
<span>( 1 次赞助, 共 ￥60 ) 留言: 谢谢，我是真的很喜欢...</span><br>
<a href="https://afdian.com/u/8a03268e8ba411f0bcbb52540025c377">
爱发电用户_e6g3
</a>
<span>( 1 次赞助, 共 ￥20 ) 留言: 给几个建议我是mac...</span><br>

</details>
<!-- 注意: 尽量将标签前靠,否则经测试可能被 GitHub 解析为代码块 -->

<!-- AFDIAN-ACTION:END -->

## 📦 安装方法

### 预编译版本

前往本 fork 的 Releases 页面下载 PotPlayer 修改版。

* 文件名: `FNMedia-PotPlayer_${version}_${os}_${arch}.${ext}`

1.字段含义：

- version：版本号
- os：操作系统
- arch：系统架构
- ext：文件扩展名

2.安装步骤

- Windows 直接安装即可使用；如需使用 PotPlayer，请先安装 PotPlayer 并在托盘设置中选择 `播放器: PotPlayer (Windows)`
- macos请使用brew安装mpv

```bash
brew install mpv
# 安装dmg后执行
sudo find "/Applications/飞牛影视.app" -exec xattr -d com.apple.quarantine {} \; 2>/dev/null
```

- linux请先安装mpv播放器(版本>0.37.0)再使用，插件前往[fntv-mpv](https://github.com/QiaoKes/fntv-mpv-config/releases) 自行安装，可以参考[issue#54](https://github.com/QiaoKes/fntv-electron/issues/54)

### 从源码构建

1. 克隆仓库：

```bash
git clone <your-fork-url>
cd fntv-electron-potplayer
```

2. 安装依赖：

```bash
npm i
```

3. 运行开发模式：

```bash
npm start
```

5. 构建安装包：

```bash
npm run build:win
npm run build:mac
npm run build:linux
```

## 常用问题Q&A

### 1. mpv播放器功能有点少，怎么客制化，想添加补帧滤镜等？

1. 自动方法
   克隆fntv-mpv仓库，自己改一下相关配置：[fntv-mpv-config](https://github.com/QiaoKes/fntv-mpv-config)
2. 手动方法
   打开你安装目录的third_party，只修改third_party\fntv-mpv\portable_config下面的插件，其余的不要动。其中input.conf是快捷键。

注意重新安装或者更新，会清空安装目录，注意备份你的mpv插件目录。

### 2. 能否支持potplayer？

本 fork 的 Windows 版本支持将 PotPlayer 作为可选外置播放器使用，默认仍使用 MPV。

使用步骤：

1. 先在 Windows 上安装 PotPlayer。
2. 启动飞牛影视桌面端，右键系统托盘图标。
3. 进入 `设置`，选择 `播放器: PotPlayer (Windows)`。
4. 如果程序没有自动找到 PotPlayer，点击 `设置PotPlayer路径`，选择 PotPlayer 可执行文件，例如：

```text
C:\Program Files\DAUM\PotPlayer\PotPlayerMini64.exe
C:\Program Files (x86)\DAUM\PotPlayer\PotPlayerMini.exe
```

5. 回到飞牛影视页面点击播放按钮，会使用 PotPlayer 打开当前视频。

当前 PotPlayer bridge 支持：

- 使用本地代理地址播放飞牛视频；
- 启动时按飞牛记录恢复播放进度；
- 自动下载飞牛外置字幕，并通过 PotPlayer `/sub` 参数加载；
- 后台每秒读取 PotPlayer 当前进度并回传飞牛；
- 退出播放器时保存最后播放进度。

注意事项：

- PotPlayer 支持仅限 Windows；macOS / Linux 仍使用 MPV。
- PotPlayer 没有 MPV 那样的标准 IPC，进度读取依赖 Windows 窗口消息。
- 初始 seek 会先使用 PotPlayer 启动参数，再由 bridge 绑定窗口后二次发送 seek。
- 字幕以启动时加载为主，播放过程中动态切换字幕不保证支持。
- MPV 的弹幕、anime4K、脚本扩展等能力不会自动迁移到 PotPlayer。

### 3. 弹幕相关问题？

弹幕问题查看uosc_danmaku的文档，根据文档内容调整配置。

### 4. 登录完客户端后，如果服务器连接不上登录会超时卡透明屏，无法切换或修改服务器配置，卸载重装也不行

去C:\\Users\\{你的计算机用户名}\\.fntv 下面把config.json删除了，因为连接成功后实际上加载的还是飞牛网页端，没响应当然会透明了。

### 5. 打开弹幕视频掉帧

打开弹幕时，默认开启fps平滑滤镜，比较吃性能，不需要可以去安装目录下的third_party\fntv-mpv\portable_config\script-opts下uosc_danmaku.conf关闭相关配置

### 6. 视频播放卡慢，双显卡，调用时发现使用核显

以下两种方法任选其一：

1) NVIDIA控制面板-管理3D设置-程序设置-添加飞牛影视-应用
2) 设置-系统-屏幕-图形显示-添加飞牛影视-选择高性能

## ⌨️ MPV播放器

1. 快捷键

```text
部分快捷键兼容potpolyer
查看安装目录下
third_party\fntv-mpv\portable_config\input.conf
```

2. MPV配置由以下仓库单独管理:
   [fntv-mpv-config](https://github.com/QiaoKes/fntv-mpv-config)
3. 预设着色器方案
   [mpv.conf](https://github.com/QiaoKes/fntv-mpv-config/blob/release/custom_config/mpv/mpv.conf)

## 🙏 特别感谢

本项目参考以下开源项目：

- [QiaoKes/fntv-electron](https://github.com/QiaoKes/fntv-electron) - 本 fork 的上游项目
- [enable-chromium-hevc-hardware-decoding](https://github.com/StaZhu/enable-chromium-hevc-hardware-decoding) - Chromium HEVC硬解码支持
- [electron-media-patch](https://github.com/5rahim/electron-media-patch) - Electron硬解码补丁
- [fnToPotplayer](https://github.com/gudqs7/fnToPotplayer) - 飞牛影视调用Potplayer
- [fnos-tv](https://github.com/thshu/fnos-tv) - fnos-tv 支持弹幕的飞牛影视
- [mpv弹幕插件](https://github.com/Tony15246/uosc_danmaku) - uosc_danmaku 基于uosc的弹幕插件

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=QiaoKes/fntv-electron&type=date&legend=top-left)](https://www.star-history.com/#QiaoKes/fntv-electron&type=date&legend=top-left)

## 📄 许可证

本项目采用 [GPL3.0 许可证](LICENSE)

Copyright (c) 2025 Tag mig hånden

本 fork 基于 QiaoKes/fntv-electron 修改，保留原项目许可证和版权声明。修改版源码与二进制分发应继续遵守 GPL-3.0。

---

**温馨提示**：本项目为第三方 fork，与飞牛影视官方无关，也不是 QiaoKes/fntv-electron 官方发布版。使用前请确保遵守相关服务条款。
