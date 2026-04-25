# Analysis app

Сервис бэктестинга детекторов теханализа из `pulsoats/core`. Он поднимает gRPC API для запуска исторических прогонов, хранит метаданные и кэш свечей в PostgreSQL и собирает результат каждого прогона в ZIP-архив.

## Ключевые возможности
- создание нового прогона через `NewRun`;
- получение метаданных прогона через `GetRun`;
- постраничный список прогонов через `ListRunsPaged`;
- публикация и удаление прогонов через `ShareRun` и `DeleteRun`;
- потоковая выгрузка готового архива через `GetRunArchive`;
- выдача каталога доступных детекторов через `Catalog/ListAvailableDetectors`;
- health-monitoring через `ServiceMonitor/Info` и `ServiceMonitor/Metrics`;
- кэширование исторических свечей в PostgreSQL для повторного использования.

## Поток обработки
1. Клиент вызывает `Analysis/NewRun`, передаёт конфиг прогона и заголовок `x-user-id`.
2. `internal/transport/xgrpc/analysis` мапит protobuf в `internal/domain/run.NewRunRequest`.
3. `internal/application/run.Application` валидирует запрос, создаёт запись в `analysis.runs` со статусом `pending` и запускает вычисление в фоне.
4. Во время вычисления сервис:
   - получает свечи через зарегистрированные exchange clients из `pulsoats/core/exchanges`, сначала проверяя кэш `analysis.candles`;
   - строит candle-detector через `pulsoats/core/detect/detectors`;
   - ищет сигналы на исходном таймфрейме и затем пересчитывает их результат на `1m` свечах;
   - сохраняет агрегированную статистику прогона;
   - собирает ZIP-архив с CSV/JSON артефактами в каталоге `RUNS_STORAGE_DIR`.
5. Финальный статус прогона обновляется в БД как `done` или `failed`.

## Архитектура
- `cmd/analysis/main.go` — точка входа, wiring приложения, инициализация PostgreSQL, registry детекторов и gRPC server.
- `internal/application/run` — основной application layer для жизненного цикла прогона.
- `internal/application/detector` — выдача списка встроенных детекторов.
- `internal/application/health` — service info и runtime metrics.
- `internal/infrastructure/repository/postgres` — репозитории прогонов, свечей, tx manager и pool.
- `internal/transport/xgrpc` — gRPC transport, registration сервисов и interceptors.
- `internal/utils/files` — генерация CSV/ZIP артефактов прогона.

## Конфигурация
- `POSTGRES_DSN` — обязательный DSN подключения к PostgreSQL.
- `RUNS_STORAGE_DIR` — каталог для ZIP-архивов прогонов. По умолчанию `data/runs`.
- `GRPC_HOST` — host для gRPC сервера. По умолчанию `0.0.0.0`.
- `GRPC_PORT` — port для gRPC сервера. По умолчанию `50051`.
- `SERVICE_SECRET_TOKEN` — обязательный service-to-service токен. Проверяется по metadata `x-service-token` для всех gRPC вызовов.
- `SERVICE_NAME` — имя сервиса для health endpoint. По умолчанию генерируется как `analysis_<uuid>`.

## gRPC API
Контракты лежат в `github.com/pulsoats/contracts`.

Сервис `analysis.v1.Analysis` реализует методы:
- `NewRun`
- `GetRun`
- `ListRunsPaged`
- `GetRunArchive`
- `ShareRun`
- `DeleteRun`

Сервис `core.v1.Catalog` реализует:
- `ListAvailableDetectors`

Сервис `health.v1.ServiceMonitor` реализует:
- `Info`
- `Metrics`

### Аутентификация и metadata
- Для всех gRPC вызовов обязателен metadata header `x-service-token`.
- Для методов `NewRun`, `ListRunsPaged`, `ShareRun`, `DeleteRun` обязателен metadata header `x-user-id`.

### Идентификаторы
- Идентификатор прогона — `UUID`.
- Пагинация `ListRunsPaged` использует `before_id` тоже в формате `UUID`.

## Данные и миграции
Миграции лежат в каталоге `migrations/`.

Текущая схема включает:
- `analysis.runs` — метаданные прогонов, статус, статистика, признак публикации;
- `analysis.candles` — кэш исторических свечей.

Актуальные особенности схемы:
- `analysis.runs.id` имеет тип `UUID`;
- в `analysis.runs` используются поля `first_candle_time` и `last_candle_time`;
- primary key таблицы `analysis.candles` строится по `(exchange, category, symbol, interval, time)`.

## Структура репозитория
```text
cmd/analysis/           входной бинарь
internal/application/   application layer
internal/domain/        доменные модели и интерфейсы
internal/infrastructure/доступ к PostgreSQL
internal/transport/xgrpc/ gRPC transport и interceptors
internal/utils/files/   генерация CSV/ZIP
migrations/             SQL-миграции
data/                   локальные артефакты прогонов
```
