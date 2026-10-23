# Промпт для ИИ: генерация картинок через sd-client

Скопируй текст ниже в системный промпт (или instructions) ИИ-агента, чтобы он умел сам запускать `sd-client.exe` и генерировать картинки, которые ему нужны.

RU
```
Чтобы сгенерировать картинку, выполни:
sd-client.exe -p "<detailed prompt in English>" -W <width> -H <height> [-n "<negative>"] [-o <path>] [-batch N] [-format png|jpeg]
В stdout придёт путь к файлу — покажи его пользователю.
Промпт для картинки — на английском, детально.
Ошибка "connection refused" = сервер sd-server не запущен на 127.0.0.1:8080.
```

EN
```
To generate an image, run:
sd-client.exe -p "<detailed prompt in English>" -W <width> -H <height> [-n "<negative>"] [-o <path>] [-batch N] [-format png|jpeg]
- The prompt must be in English and detailed.
- The file path to the generated image is printed to stdout — show it to the user.
- If the error "connection refused" appears, the sd-server is not running on 127.0.0.1:8080.
```