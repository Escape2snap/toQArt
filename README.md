# toQArt

将图片生成带有自定义内容的 QR 码。

## 特性

- 通过 FFmpeg 读取图片或视频，编码为 QR 码
- **保留原图颜色** — 用原始像素颜色替换黑色模块
- **颜色色调** — 将原图颜色压暗到统一亮度，保留色调同时保证扫码可靠性
- **图片反色** — 预处理时反转图片颜色
- **图案模式** — 基于阈值和网格的装饰性图案
- 支持 WebUI 和命令行两种使用方式
- 配置与 `toqart.toml` 文件合并（优先级：CLI > toml > 默认值）
- WebUI 暗色模式，支持 Auto / Light / Dark 切换
- WebUI 请求日志（plain / JSON 格式）
- Makefile 一键构建，支持 systemd 部署

## 快速开始

### 构建

```bash
# 需要 Rust、Go、FFmpeg 开发库

# 使用 Makefile 一键构建（推荐）
make

# 或分别构建
make toqart     # Rust 后端
make webui      # Go WebUI（自动注入编译时间）
make run-webui  # 构建并启动 WebUI
make clean      # 清理构建产物

# 构建产物
# - target/release/toqart     (Rust 后端, ~6.5MB)
# - webui/toqart-webui        (Go WebUI, ~13MB)
```

### 命令行使用

```bash
# 基本用法
./target/release/toqart ./example/101798742.jpg

# 保留原图颜色
./target/release/toqart --color true ./example/101798742.jpg

# 颜色色调模式（与 --color 互斥）
./target/release/toqart --color-tint true --tint-brightness 64 ./example/101798742.jpg

# 调高灰度阈值使 QR 更密集
./target/release/toqart --threshold 180 --content "https://example.com" ./example/101798742.jpg
```

### WebUI

```bash
cd webui
./toqart-webui                              # 默认端口 5462
./toqart-webui --port 8080                  # 指定端口
./toqart-webui --toqart ./bin/toqart        # 指定后端二进制路径（远端部署用）
./toqart-webui --log                        # 启用请求日志（plain 格式）
./toqart-webui --log=json                   # JSON 格式日志
./toqart-webui --log --log-out /tmp/log     # 指定日志文件路径
```

浏览器打开 `http://localhost:5462`。

**WebUI 特性：**
- **暗色模式** — 支持 Auto / Light / Dark 切换，跟随系统或手动选择，配色持久化到 localStorage
- **上传图片自动设宽高比** — X/Y 宽高比自动按图片原始比例（约分到最简整数）设置
- **下载文件名** — 下载按钮使用服务端生成的文件名，包含 QR 参数信息
- **编译时间** — 启动时显示构建时间，便于版本追溯

**WebUI 参数：**

| 参数 | 默认 | 说明 |
|------|------|------|
| `--port` | `5462` | 监听端口 |
| `--toqart <path>` | 自动查找 | toqart 后端二进制路径，远端部署时使用 |
| `--log` / `--log=<plain\|json>` | — | 启用请求日志，`--log` 默认 plain 格式 |
| `--log-out <path>` | `./log/{时间戳}.log` | 日志输出文件路径。不指定时自动创建 |
| `--footer <path>` | — | 注入页脚 HTML 文件内容到页面底部，自动适配暗色/亮色模式 |

**日志格式：**

- plain: `[2026-05-02 20:01:30] GET | 127.0.0.1 | index page`
- json: `{"time":"2026-05-02 20:01:30","method":"GET","ip":"127.0.0.1","action":"index page"}`

不指定 `--log-out` 时，日志自动写入 `./log/{启动时间戳}.log`，同时输出到控制台。
IP 获取顺序：`X-Real-IP` → `X-Forwarded-For` → `RemoteAddr`。

### 部署（systemd）

项目附带 `toqart-webui.service`，可用于 Linux 服务器自启动管理：

```bash
# 安装服务
sudo cp toqart-webui.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now toqart-webui

# 管理
sudo systemctl status toqart-webui
sudo journalctl -u toqart-webui -f   # 查看实时日志
sudo systemctl stop|start|restart toqart-webui
```

## 参数说明

| 参数 | 范围 | 默认 | 说明 |
|------|------|------|------|
| `--content` | 文本 | `Attention Is All You Need.` | QR 码编码内容 |
| `--threshold` | 0-255 | `127` | 灰度阈值，低于此值的像素标记为"on" |
| `--qr-version` | 1-40 | `11` | QR 码版本（自动修正为奇数） |
| `--x-aspect` | ≥1 | `1` | X 方向宽高比 |
| `--y-aspect` | ≥1 | `1` | Y 方向宽高比 |
| `--pad-l` | ≥0 | `2` | 左填充（模块数） |
| `--pad-r` | ≥0 | `2` | 右填充（模块数） |
| `--use-pattern` | bool | `false` | 启用装饰性图案模式 |

### 颜色/色调参数

互斥模式（三选一）：

| 模式 | 参数 | 说明 |
|------|------|------|
| 保留原图颜色 | `--color true` | 替换"on"模块为原始像素颜色 |
| 颜色色调 | `--color-tint true` | 压暗颜色至统一亮度，保留色调 |
| 图片反色 | 预处理时反转 | 通过 `mode=invert` 在 WebUI 中启用 |

通用颜色参数：

| 参数 | 范围 | 默认 | 说明 |
|------|------|------|------|
| `--color-threshold` | 0-255 | `0` | 颜色/色调应用的最低灰度值。低于此值的像素保持纯黑 |
| `--tint-brightness` | 0-255 | `64` | 色调模式的目标亮度。越低越黑，越高颜色越明显 |

**色调模式原理**：每个像素 `RGB × (tint_brightness / gray)`，将所有"on"模块压暗到近似统一的亮度水平，同时保留原始色调。默认 64 适合扫码，值越低越接近纯黑。

### 配置文件 `toqart.toml`

```toml
[qart]
use_pattern = false
use_color = false
color_threshold = 0
use_color_tint = false
tint_brightness = 64
qr_version = 11
x_aspect = 1
y_aspect = 1
pad_l = 2
pad_r = 2
content = "Attention Is All You Need."
threshold = 127

[path]
input_path = "example/101798742.jpg"
output_path = "./toqart"
```

```bash
# 生成默认配置
./target/release/toqart --create-config
```

## 致谢

感谢 [zhengkyl/fuqr](https://github.com/zhengkyl/fuqr) 项目。本项目的核心 QR 码生成逻辑源自 fuqr 的 examples/bad_apple.rs。
