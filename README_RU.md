# sd-client

> [English version](README.md)

> [China version](README_CN.md)

Минималистичный Go CLI-клиент для сервера [stable-diffusion.cpp](https://github.com/leejet/stable-diffusion.cpp).

Отправляет запрос text-to-image на OpenAI-совместимый эндпоинт `POST /v1/images/generations` и сохраняет сгенерированную картинку локально в PNG или JPEG — удобно для скриптов и ИИ-агентов, которым нужно генерировать картинки «на лету».

## Как это работает

1. Формирует JSON-запрос: `prompt`, `negative_prompt`, `size` (`WxH`), `n`, `seed`, `output_format`, `output_compression`.
2. Делает POST на `http://<server>/v1/images/generations` с `Content-Type: application/json`.
3. Расшифровывает ответ (base64 data URL, `b64_json` или обычная URL картинки).
4. Сохраняет картинку на диск и выводит путь к файлу в stdout.

## Требования

- Go 1.21+ (для сборки)
- Рабочий сервер stable-diffusion.cpp (по умолчанию `127.0.0.1:8080`)

## Сборка

```
go build -o sd-client.exe .
```

## Запуск локального сервера (пример)

```bat
sd-server --listen-ip 127.0.0.1 --listen-port 8080 --seed -1 ^
 --diffusion-model qwen2\qwen-image-2.1-Q4_K_M.gguf ^
 --vae qwen2\qwen_image_2.1_vae_bf16.safetensors ^
 --llm qwen2\Qwen3-VL-8B-Instruct-UD-Q4_K_XL.gguf ^
 --steps 25 --cfg-scale 1.0 --sampling-method euler -W 256 -H 256 --diffusion-fa
```

## Параметры

| Флаг | JSON-поле | Описание | По умолчанию |
|---|---|---|---|
| `-p` | `prompt` | Промпт (обязательный) | — |
| `-n` | `negative_prompt` | Негативный промпт | пусто |
| `-W` | `size` | Ширина картинки, px | 256 |
| `-H` | `size` | Высота картинки, px | 256 |
| `-batch` | `n` | Количество картинок | 1 |
| `-seed` | `seed` | Сид генерации (-1 = случайный) | -1 |
| `-format` | `output_format` | Формат: `png` или `jpeg` | png |
| `-quality` | `output_compression` | Компрессия, 0–100 | 100 |
| `-o` | — | Путь к файлу результата; расширение добавится, если не указано | `generated/img_<timestamp>.png` |
| `-server` | — | Адрес сервера `host:port` | 127.0.0.1:8080 |
| `-timeout` | — | Таймаут запроса (Go duration: `30s`, `5m`, …) | 10m |

## Использование

```
sd-client.exe -W 256 -H 256 -p "a cat on a roof" -n "blurry, low quality"
```

Ещё примеры:

```
sd-client.exe -p "cyberpunk city" -W 512 -H 512 -o city.jpg
sd-client.exe -p "watercolor fox" -batch 4
sd-client.exe -p "oil painting of a boat" -format jpeg -quality 80 -o boat
sd-client.exe -p "a rabbit" -server 192.168.0.10:8080 -timeout 30m
```

## Результат

- В stdout выводится путь к сохранённому файлу (по строке на картинку при `-batch > 1`).
- Без `-o` картинки попадают в `generated/img_<YYYYMMDD_HHMMSS>.png` (при батче — с суффиксами `_1`, `_2`, …).
- Код выхода `0` при успехе, `1` при ошибке (сообщение об ошибке — в stderr).
