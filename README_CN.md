# sd-client

> [English version](README.md)

> [Russian version](README_RU.md)

一个用于 [stable-diffusion.cpp](https://github.com/leejet/stable-diffusion.cpp) 服务端的极简 Go 命令行客户端。

它向 OpenAI 兼容的 `POST /v1/images/generations` 接口发送文生图请求，并将生成的图片以 PNG 或 JPEG 格式保存到本地——非常适合需要即时生成图片的脚本和 AI 代理。

## 工作原理

1. 构造 JSON 请求：`prompt`、`negative_prompt`、`size`（`WxH`）、`n`、`seed`、`output_format`、`output_compression`。
2. 以 `Content-Type: application/json` POST 到 `http://<server>/v1/images/generations`。
3. 解析响应（base64 data URL、`b64_json` 或普通图片 URL）。
4. 将图片保存到磁盘，并把文件路径打印到 stdout。

## 环境要求

- Go 1.21+（用于编译）
- 正在运行的 stable-diffusion.cpp 服务端（默认：`127.0.0.1:8080`）

## 编译

```
go build -o sd-client.exe .
```

## 启动本地服务端（示例）

```bat
sd-server --listen-ip 127.0.0.1 --listen-port 8080 --seed -1 ^
 --diffusion-model qwen2\qwen-image-2.1-Q4_K_M.gguf ^
 --vae qwen2\qwen_image_2.1_vae_bf16.safetensors ^
 --llm qwen2\Qwen3-VL-8B-Instruct-UD-Q4_K_XL.gguf ^
 --steps 25 --cfg-scale 1.0 --sampling-method euler -W 256 -H 256 --diffusion-fa
```

## 参数

| 参数 | JSON 字段 | 说明 | 默认值 |
|---|---|---|---|
| `-p` | `prompt` | 正向提示词（必填） | — |
| `-n` | `negative_prompt` | 负向提示词 | 空 |
| `-W` | `size` | 图片宽度（像素） | 256 |
| `-H` | `size` | 图片高度（像素） | 256 |
| `-batch` | `n` | 生成图片数量 | 1 |
| `-seed` | `seed` | 生成随机种子（-1 = 随机） | -1 |
| `-format` | `output_format` | 输出格式：`png` 或 `jpeg` | png |
| `-quality` | `output_compression` | 压缩质量，0–100 | 100 |
| `-o` | — | 输出文件路径；不带扩展名时自动补充 | `generated/img_<时间戳>.png` |
| `-server` | — | 服务端地址 `host:port` | 127.0.0.1:8080 |
| `-timeout` | — | 请求超时（Go duration：`30s`、`5m`…） | 10m |

## 使用示例

```
sd-client.exe -W 256 -H 256 -p "a cat on a roof" -n "blurry, low quality"
```

更多示例：

```
sd-client.exe -p "cyberpunk city" -W 512 -H 512 -o city.jpg
sd-client.exe -p "watercolor fox" -batch 4
sd-client.exe -p "oil painting of a boat" -format jpeg -quality 80 -o boat
sd-client.exe -p "a rabbit" -server 192.168.0.10:8080 -timeout 30m
```

## 输出说明

- 成功后把保存的文件路径打印到 stdout（`-batch > 1` 时每张图片一行）。
- 未指定 `-o` 时，图片保存到 `generated/img_<YYYYMMDD_HHMMSS>.png`（批量生成时带 `_1`、`_2`… 后缀）。
- 成功退出码为 `0`，出错为 `1`（错误信息输出到 stderr）。
