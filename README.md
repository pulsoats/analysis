# Analysis

Сервис исторического анализа торговых сигналов для `pulsoats`. Поднимает gRPC API, запускает бэктесты детекторов из `github.com/pulsoats/core`, хранит метаданные прогонов и кэш свечей в PostgreSQL/TimescaleDB, а результат каждого завершенного прогона сохраняет ZIP-архивом.

## Что делает сервис

- принимает запрос на новый прогон через `Analysis/NewRun`;
- валидирует рынок, период, интервал, детектор и параметры комиссий;
- получает исторические свечи из exchange clients `pulsoats/core/exchanges`;
- кэширует свечи в таблице `analysis.candles`;
- запускает candle-detector из registry `github.com/pulsoats/detectors`;
- пересчитывает результат сигналов на минимальном таймфрейме `1m`;
- сохраняет статус, количество сигналов и среднюю доходность в `analysis.runs`;
- собирает ZIP-архив с CSV/JSON артефактами;
- отдает метаданные прогонов, список прогонов, публикацию, удаление и stream архива по gRPC;
- отвечает на health-check запросы через стандартный `grpc.health.v1.Health`.

## Поток выполнения прогона

1. Клиент вызывает `pulsoats.analysis.v1.Analysis/NewRun` и передает metadata `x-user-id`.
2. gRPC transport мапит protobuf request в `internal/domain/run.NewRunRequest`.
3. `internal/application/run.Application` создает запись в `analysis.runs` со статусом `pending`.
4. Прогон запускается в фоне:
   - статус меняется на `running`;
   - свечи читаются из кэша PostgreSQL или догружаются с биржи;
   - детектор ищет сигналы на исходном интервале;
   - для найденных сигналов результат сделки считается на `1m` свечах;
   - строится ZIP-архив `run_<uuid>.zip` в `RUNS_STORAGE_DIR`;
   - статус меняется на `done` или `failed`.
5. Клиент может получить метаданные через `GetRun` и архив через streaming-метод `GetRunArchive`.

## Структура проекта

```text
cmd/                      точка входа, wiring зависимостей и запуск gRPC
internal/application/     application layer: run, catalog
internal/domain/          доменные модели и интерфейсы репозиториев
internal/infrastructure/  PostgreSQL pool, tx manager и репозитории
internal/transport/xgrpc/ gRPC servers, mappers и interceptors
internal/utils/files/     генерация CSV/JSON/ZIP артефактов
migrations/               SQL-миграции схемы analysis
```

## Конфигурация

Основные переменные окружения:

| Переменная | Описание | Значение по умолчанию |
| --- | --- | --- |
| `POSTGRES_DSN` | DSN подключения к PostgreSQL/TimescaleDB | обязательна |
| `RUNS_STORAGE_DIR` | каталог ZIP-архивов прогонов | `data/runs` |
| `GRPC_HOST` | host gRPC сервера | `0.0.0.0` |
| `GRPC_PORT` | port gRPC сервера | `50051` |
| `GRPC_TLS_CERT_FILE` | server certificate для gRPC TLS/mTLS | нет |
| `GRPC_TLS_KEY_FILE` | private key для gRPC TLS/mTLS | нет |
| `GRPC_TLS_CA_FILE` | CA certificate для проверки клиентов | нет |
| `GRPC_TLS_DISABLE` | локальная опция отключения TLS | `false` |
| `LOG_LEVEL` | уровень логирования: `debug`, `info`, `warn`, `error` | `info` |
| `LOG_FORMAT` | формат логов: `json` или `console` | `console` |

Пример локального файла окружения лежит в `.env.example`.

Для локальной разработки можно выставить `GRPC_TLS_DISABLE=true`, тогда gRPC сервер стартует без TLS credentials. Для production-запуска с TLS нужно передать пути к certificate, key и CA через `GRPC_TLS_CERT_FILE`, `GRPC_TLS_KEY_FILE` и `GRPC_TLS_CA_FILE`.

## Docker Compose

`docker-compose.yml` поднимает только Go-сервис `analysis`. Внешняя инфраструктура, включая PostgreSQL, миграции и выпуск TLS-сертификатов, в compose не описана.

Для сборки Docker image используется BuildKit secret `github_token`, потому что зависимости `github.com/pulsoats/*` помечены как private через `GOPRIVATE`.

Перед запуском нужны значения в `.env`:

```env
POSTGRES_DSN=postgres://user:password@host:5432/db?sslmode=disable
GRPC_PORT=50051
GRPC_TLS_DISABLE=true
```

Запуск:

```bash
GITHUB_TOKEN=... docker compose up --build
```

Сервис ожидает внешний PostgreSQL/TimescaleDB по `POSTGRES_DSN`.

## Миграции и данные

Миграции лежат в `migrations/` и создают:

- расширение `timescaledb`;
- схему `analysis`;
- hypertable `analysis.candles` для кэша свечей;
- таблицу `analysis.candles_staging`;
- таблицу `analysis.runs` для метаданных прогонов.

Ключевые поля `analysis.runs`:

- `id` — UUID прогона;
- `status_code`, `status_message` — статус выполнения;
- `exchange`, `category`, `symbol`, `interval` — рынок и таймфрейм;
- `detector_code`, `detector_label`, `detector_opts` — конфигурация детектора;
- `first_candle_time`, `last_candle_time` — фактические границы свечей;
- `signals_count`, `avg_profit_ppm`, `sum_profit_ppm` — агрегированный результат;
- `taker_fee_ppm`, `maker_fee_ppm` — комиссии прогона;
- `created_by`, `is_shared`, `shared_at` — владелец и публикация.

Primary key `analysis.candles`:

```text
(exchange, category, symbol, interval, time)
```

## gRPC API

Контракты подключаются из `github.com/pulsoats/contracts`.

`pulsoats.analysis.v1.Analysis`:

- `NewRun`
- `GetRun`
- `ListRunsPaged`
- `GetRunArchive`
- `ShareRun`
- `DeleteRun`

`pulsoats.catalog.v1.Catalog`:

- `AvailableDetectors`
- `AvailableExchanges`

`grpc.health.v1.Health`:

- `Check`
- `Watch`

### Metadata

Для пользовательских unary-методов обязателен metadata header:

```text
x-user-id: <user-id>
```

Он требуется для:

- `Analysis/NewRun`
- `Analysis/ListRunsPaged`
- `Analysis/ShareRun`
- `Analysis/DeleteRun`

`GetRun`, `GetRunArchive`, `Catalog/*` и `ServiceMonitor/*` не требуют `x-user-id` на уровне текущего interceptor.

Старый interceptor service token в текущем коде отсутствует, поэтому `x-service-token` сейчас не проверяется.

## Локальная разработка

Минимальные требования:

- Go `1.26.2`;
- PostgreSQL с TimescaleDB;
- примененные SQL-миграции из `migrations/`;
- доступ к private Go modules `github.com/pulsoats/*`;
- TLS сертификаты для gRPC или `GRPC_TLS_DISABLE=true` для локального запуска.

Полезные команды:

```bash
go test ./...
go run ./cmd/analysis
docker compose up --build
```
