# sd-client

A minimal Go CLI client for the [stable-diffusion.cpp](https://github.com/leejet/stable-diffusion.cpp) server.

It sends a text-to-image request to the OpenAI-compatible `POST /v1/images/generations` endpoint and saves the generated image locally as PNG or JPEG — handy for scripts and AI agents that need to generate images on the fly.

## How it works

1. Builds a JSON request: `prompt`, `negative_prompt`, `size` (`WxH`), `n`, `output_format`, `output_compression`.
2. POSTs it to `http://<server>/v1/images/generations` with `Content-Type: application/json`.
3. Decodes the response (base64 data URL, `b64_json`, or a plain image URL).
4. Saves the image to disk and prints the file path to stdout.

## Requirements

- Go 1.21+ (to build)
- A running stable-diffusion.cpp server (default: `127.0.0.1:8080`)

## Building

```
go build -o sd-client.exe .
```

## Starting the local server (example)

```bat
sd-server --listen-ip 127.0.0.1 --listen-port 8080 --seed -1 ^
 --diffusion-model qwen2\qwen-image-2.1-Q4_K_M.gguf ^
 --vae qwen2\qwen_image_2.1_vae_bf16.safetensors ^
 --llm qwen2\Qwen3-VL-8B-Instruct-UD-Q4_K_XL.gguf ^
 --steps 25 --cfg-scale 1.0 --sampling-method euler -W 256 -H 256 --diffusion-fa
```

## Parameters

| Flag | JSON field | Description | Default |
|---|---|---|---|
| `-p` | `prompt` | Text prompt (required) | — |
| `-n` | `negative_prompt` | Negative prompt | empty |
| `-W` | `size` | Image width in pixels | 256 |
| `-H` | `size` | Image height in pixels | 256 |
| `-batch` | `n` | Number of images to generate | 1 |
| `-format` | `output_format` | Output format: `png` or `jpeg` | png |
| `-quality` | `output_compression` | Compression quality, 0–100 | 100 |
| `-o` | — | Output file path; an extension is appended if missing | `generated/img_<timestamp>.png` |
| `-server` | — | Server address `host:port` | 127.0.0.1:8080 |
| `-timeout` | — | Request timeout (Go duration: `30s`, `5m`, …) | 10m |

## Usage

```
sd-client.exe -W 256 -H 256 -p "a cat on a roof" -n "blurry, low quality"
```

More examples:

```
sd-client.exe -p "cyberpunk city" -W 512 -H 512 -o city.jpg
sd-client.exe -p "watercolor fox" -batch 4
sd-client.exe -p "oil painting of a boat" -format jpeg -quality 80 -o boat
sd-client.exe -p "a rabbit" -server 192.168.0.10:8080 -timeout 30m
```

## Output

- The saved file path is printed to stdout (one line per image when `-batch > 1`).
- Without `-o`, images go to `generated/img_<YYYYMMDD_HHMMSS>.png` (`_1`, `_2`, … suffixes for batches).
- Exit code `0` on success, `1` on any error (error message goes to stderr).
