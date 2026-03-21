# Analysis Service

Сервис бэктестинга детекторов теханализа из `pulsoats/core`. Через gRPC он запускает вычислительные «раны» по историческим свечам, хранит метаданные и кэш данных в PostgreSQL, а результат каждого прогона собирает в ZIP‑архив со свечами и найденными сигналами.

## Ключевые возможности
- создание запуска (`StartRun`) с указанием биржи, инструмента, интервала, детектора, комиссии и временного окна;
- проверка статуса (`GetRunStatus`) и получение метаданных (`GetRunMeta`);
- выгрузка готового ZIP‑архива (`GetRunResult`) с CSV свечей и сигналов;
- кэширование исторических свечей в PostgreSQL для повторного использования.

## Поток обработки
1. gRPC вызывает `StartRun`, `internal/transport/grpc` мапит protobuf в `internal/model/newRun.Request`.
2. `internal/service/newRun.Service` валидирует параметры и создаёт запись в `analysis_runs` со статусом `pending`, затем в фоне запускает вычисление.
3. Во время вычисления сервис:
   - получает нужные свечи через зарегистрированные биржевые API (`github.com/pulsoats/core/exchanges`), сначала проверяя кэш `analysis_candles`;
   - строит детектор из `pulsoats/core/detect` и гоняет его на последовательности свечей;
   - агрегирует сигналы, рассчитывает статистику (счётчик срабатываний, среднюю прибыль ppm, доходность каждого сигнала на минутных свечах);
   - сохраняет метаданные прогона и собирает ZIP (CSV со свечами/сигналами + JSON мета).
4. Финальный статус `done` или `failed` обновляется в `analysis_runs`, а архив лежит в `ANALYSIS_STORAGE_DIR`.

- `cmd/main.go` — точка входа: логгер (`zerolog`), пул PostgreSQL (`pgxpool`), регистры детекторов и бирж, gRPC сервер;
- `internal/service/newRun` — доменная логика бэктеста, работа с `detectors.Registry`, singleflight-кэш для свечей и сбор ZIP;
- `internal/infrastructure/repository/postgres` — хранилище запусков (`runs.Repository`) и локальный кэш свечей (`candles.Repository`);
- `internal/detect` — адаптер ядра `pulsoats/core/detect`;
- `internal/transport/grpc` — сервер `analysis.v1.AnalysisService`, error mapping и стриминг архива;
- `internal/utils/files` — генераторы CSV/ZIP, используются `service.newRun`.

## Конфигурация
- `POSTGRES_DSN` — обязательный DSN подключения;
- `ANALYSIS_GRPC_ADDR` — адрес gRPC (по умолчанию `:50051`);
- `ANALYSIS_STORAGE_DIR` — путь к каталогу для ZIP архивов (по умолчанию `data/runs` или `/data` в Docker).

## Данные и миграции
Миграции лежат в `migrations` и описывают схему с таблицами `analysis_runs` (метаданные запусков) и `analysis_candles` (кэш свечей). Применяются по возрастанию таймстемпа, откатываются соответствующими `down`.

## gRPC API
Контракт `analysis.v1` находится в `github.com/pulsoats/contracts`. Сервис реализует методы:
- `StartRun` — создаёт запуск и возвращает идентификатор (int64 → string). Ошибки бизнес-валидации транслируются как `InvalidArgument`.
- `GetRunStatus` — выдаёт код статуса (`pending`, `running`, `done`, `failed`) и текст сообщения репозитория. Если ID не найден, сервис возвращает `NotFound`.
- `GetRunMeta` — отдаёт исходные параметры запуска, фактические границы данных и финальную статистику.
- `GetRunResult` — потоковая выдача ZIP‑архива chunk-ами по 64 KiB. Доступен только после `STATUS_DONE`, иначе `FailedPrecondition`.

Код ID в ответах всегда строковый; клиенты должны приводить его к int64 для внутренних задач.

## Структура репозитория
```
cmd/                    входной бинарь
internal/service/newRun    бизнес-логика бэктеста
internal/transport      gRPC сервер и мапперы
internal/infrastructure доступ к PostgreSQL
internal/utils/files    генерация CSV/ZIP
migrations/             SQL-скрипты схемы
data/                   локальные артефакты (gitignored)
```
