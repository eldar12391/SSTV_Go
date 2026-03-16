# sstv-go

Веб-приложение SSTV (Slow Scan Television) на Go.

Проект умеет:
- кодировать изображение -> SSTV-аудио (Martin M1)
- декодировать аудио (WAV или MP3) -> изображение
- запускаться одним бинарником со встроенным веб-интерфейсом

## Что реализовано

Режим: `Martin M1`
- размер изображения: `320x256`
- длительность: около `114 с`
- частоты пикселей: `1500..2300 Hz`

## Быстрый старт (локально)

Требования:
- Go `1.22+`
- `ffmpeg` (для поддержки MP3 encode/decode)

Запуск:

```bash
go run ./cmd/sstv
```

Открыть в браузере:

```text
http://localhost:8080
```

Сборка бинарника:

```bash
go build -o sstv ./cmd/sstv
./sstv -addr :8080
```

## Быстрый старт (только Docker)

Локально Go не требуется.

### Вариант 1: docker compose

```bash
docker compose up --build
```

Открыть `http://localhost:8080`.

### Вариант 2: docker build + run

```bash
docker build -t sstv-go .
docker run --rm -p 8080:8080 sstv-go
```

## API

### `POST /api/encode`

Поля multipart:
- обязательное: `image`
- опциональное: `sample_rate` (`8000..96000`, по умолчанию `44100`)
- опциональное: `audio_format` (`wav` или `mp3`, по умолчанию `wav`)
- опциональное: `mp3_bitrate` (`64..320`, по умолчанию `192`, только для mp3)

Ответ:
- `audio/wav` при `audio_format=wav`
- `audio/mpeg` при `audio_format=mp3`

Пример WAV:

```bash
curl -X POST \
  -F "image=@./input.png" \
  -F "audio_format=wav" \
  -o sstv_signal.wav \
  http://localhost:8080/api/encode
```

Пример MP3:

```bash
curl -X POST \
  -F "image=@./input.png" \
  -F "audio_format=mp3" \
  -F "mp3_bitrate=192" \
  -o sstv_signal.mp3 \
  http://localhost:8080/api/encode
```

### `POST /api/decode`

Поля multipart:
- обязательное: `audio` (WAV или MP3)
- опциональное: `out_width` (`64..2048`, по умолчанию `320`)
- опциональное: `out_height` (`64..2048`, по умолчанию `256`)

Ответ:
- `image/png`

Пример:

```bash
curl -X POST \
  -F "audio=@./sstv_signal.mp3" \
  -F "out_width=640" \
  -F "out_height=512" \
  -o decoded.png \
  http://localhost:8080/api/decode
```

## Настройки в UI

Панель TX:
- выходной формат: WAV или MP3
- частота дискретизации
- битрейт MP3

Панель RX:
- ширина/высота выходного изображения
- скорость отрисовки

## Структура проекта

```text
cmd/sstv/main.go                 точка входа приложения
cmd/sstv/web/                    встроенные веб-ассеты
internal/server/server.go        HTTP-обработчики
internal/codec/                  encode/decode + WAV I/O
Dockerfile                       многоступенчатая сборка контейнера
docker-compose.yml               запуск одной командой
```

## Примечания

- Поддержка MP3 зависит от наличия `ffmpeg` на стороне сервера.
- В Docker-образ `ffmpeg` уже включен.
- Для максимального качества декодирования используйте WAV.

## Лицензия

MIT
