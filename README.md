# toQArt

一个将视频/图片帧生成带有自定义内容或图案的 QR 码图片的小工具。

主要特性：
- 支持通过 FFmpeg 读取并处理视频或图像（PNG、JPEG 等）。
- 命令行参数与 `toqart.toml` 配置合并（优先级：命令行 > toml > 默认值）。
- 可创建默认配置文件 `toqart.toml`。
- 可自定义二维码内容（默认为 `Attention Is All You Need.`）。

## 快速开始

1. 构建（发布版）：

```bash
cargo build --release
# 可执行文件： target/release/toqart
```

2. 生成默认配置文件：

```bash
./target/release/toqart --create-config
```

3. 使用示例：

- 使用默认参数处理图片：

```bash
./target/release/toqart ./example/101798742.jpg
```

- 指定二维码内容和输出目录：

```bash
./target/release/toqart --content "https://example.com" --output-path ./out ./example/101798742.jpg
```

- 指定部分或全部参数：

```bash
./target/release/toqart \
  --use-pattern true \
  --qr-version 12 \
  --x-aspect 1 --y-aspect 1 \
  --pad-l 2 --pad-r 2 \
  --content "Attention Is All You Need." \
  --output-path ./toqart \
  ./example/101798742.jpg
```

### 命令行参数

- `--use-pattern <true|false>`       Use pattern mode（默认：`false`）
- `--qr-version <NUM>`               QR 版本（默认：`11`）
- `--x-aspect <NUM>`                 X 宽高比（默认：`1`）
- `--y-aspect <NUM>`                 Y 宽高比（默认：`1`）
- `--pad-l <NUM>`                    左填充（默认：`2`）
- `--pad-r <NUM>`                    右填充（默认：`2`）
- `--content <TEXT>`                 二维码内容（默认：`Attention Is All You Need.`）
- `--output-path <PATH>`             输出目录（默认：`./toqart`）
- `--create-config`                  创建默认 `toqart.toml`
- `--help`                           显示帮助

### 配置文件 toqart.toml 示例

```toml
[qart]
use_pattern = false
qr_version = 11
x_aspect = 1
y_aspect = 1
pad_l = 2
pad_r = 2
content = "Attention Is All You Need."

[path]
input_path = "example/101798742.jpg"
output_path = "./toqart"
```

## 备注

- 运行时可能会看到 FFmpeg 的信息，如：
  `[swscaler @ ...] deprecated pixel format used, make sure you did set range correctly`。
  这是 swscaler 的提示，通常不会影响输出；若需抑制，可通过程序内设置 FFmpeg 日志级别（已默认设置为 `Warning`）。

- 程序仍依赖 FFmpeg（通过 `ffmpeg-next` crate）进行解码/缩放等处理，即使输入是图像也会使用 FFmpeg 流程。
