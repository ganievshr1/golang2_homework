# Finance Services: PostgreSQL и Redis

Gateway принимает HTTP-запросы на :8080 и вызывает импортированный
модуль Ledger. Ledger хранит бюджеты и транзакции в PostgreSQL.
Отчёты по категориям кешируются в Redis на 30 секунд.

В gateway/go.mod настроены require и replace на ../ledger.

Gateway и импортированный Ledger работают в одном процессе.
Консольная команда ledger/cmd/ledger проверяет подключения и ожидает Ctrl+C;
отдельного HTTP-сервера Ledger в этой архитектуре нет.
Gateway самостоятельно вызывает ledger.Connect.

Контракты /api/transactions и /api/budgets сохранены.
Добавлен маршрут /api/reports/summary.

## Требования

- Go 1.23 или новее, 64-битная ОС.
- Docker Desktop с работающим движком.
- Docker Compose.
- Git и curl.
- Goose v3.24.1.

Для Windows ниже приведены команды PowerShell.

## Подготовка

Все команды выполняются из корня finance-services.

```powershell
gofmt -w ledger gateway

Push-Location ledger
go mod tidy
Pop-Location

Push-Location gateway
go mod tidy
Pop-Location

docker compose up -d --wait
```

Compose создаёт PostgreSQL, БД cashapp, пользователя postgres
и постоянный том postgres_data. Также запускается Redis.

Пароль postgres/postgres используется для локального учебного запуска.

## Переменные окружения

```powershell
$env:DATABASE_URL = 'postgres://postgres:postgres@127.0.0.1:15432/cashapp?sslmode=disable'
$env:REDIS_ADDR = 'localhost:6379'
$env:REDIS_DB = '0'
$env:REDIS_PASSWORD = ''
```

Переменные нужно повторять в каждом новом терминале.

DATABASE_URL имеет приоритет. Если переменная не задана,
DSN собирается из DB_HOST, DB_PORT, DB_USER, DB_PASS, DB_NAME.

Значения по умолчанию:

- DB_HOST=localhost;
- DB_PORT=5432;
- DB_USER=postgres;
- DB_PASS=postgres;
- DB_NAME=cashapp;
- sslmode=disable;
- REDIS_ADDR=localhost:6379;
- REDIS_DB=0;
- REDIS_PASSWORD пустой.

При запуске выполняются sql.Open("pgx", dsn), настройка пула,
PostgreSQL PingContext и Redis Ping.
Ошибка подключения завершает приложение с понятным сообщением.

Пул PostgreSQL: MaxOpenConns=10, MaxIdleConns=5,
ConnMaxLifetime=30 минут.

## Миграции Goose

Из корня проекта:

```powershell
go install github.com/pressly/goose/v3/cmd/goose@v3.24.1

$goose = Join-Path (go env GOPATH) 'bin\goose.exe'

& $goose -dir ./ledger/migrations postgres "$env:DATABASE_URL" up
if ($LASTEXITCODE -ne 0) {
    throw 'Не применились миграции'
}

& $goose -dir ./ledger/migrations postgres "$env:DATABASE_URL" status
```

Миграции применяются до запуска приложений.

В Goose направления Up и Down находятся в одном версионном SQL-файле.
В проекте две миграции: budgets и expenses.

Откат последней миграции:

```powershell
& $goose -dir ./ledger/migrations postgres "$env:DATABASE_URL" down
```

Полный откат:

```powershell
& $goose -dir ./ledger/migrations postgres "$env:DATABASE_URL" down-to 0
```

Down удаляет соответствующие таблицы и данные.
Для проверки сохранности после перезапуска откат не нужен.

## Запуск Ledger

В первом терминале после задания переменных окружения:

```powershell
cd ledger
go run ./cmd/ledger
```

Ожидаются сообщения о подключении к PostgreSQL и Redis.

Процесс остаётся запущенным до Ctrl+C.
Демонстрационные данные не добавляются.
budgets.json автоматически не загружается.

## Запуск Gateway

Во втором терминале откройте корень проекта,
повторите переменные окружения и выполните:

```powershell
cd gateway
go run .
```

Сервер слушает http://localhost:8080.
Остановка — Ctrl+C.

## Эндпоинты

| Метод | Путь | Успешный ответ |
|---|---|---|
| POST | /api/budgets | 201, созданный или обновлённый бюджет |
| GET | /api/budgets | 200, массив бюджетов |
| POST | /api/transactions | 201, транзакция с ID |
| GET | /api/transactions | 200, массив транзакций |
| GET | /api/reports/summary?from=YYYY-MM-DD&to=YYYY-MM-DD | 200, отчёт |
| GET | /ping | 200, {"message":"pong"} |

Все ответы имеют Content-Type: application/json; charset=utf-8.

Ошибки имеют формат {"error":"..."}:

- 400 — невалидный JSON, поля или период отчёта;
- 409 — budget exceeded;
- 500 — internal server error, подробности записываются в лог;
- 404 — неизвестный путь;
- 405 — неподдерживаемый метод.

Middleware логирует HTTP-метод, путь и длительность.

## Денежные суммы и бизнес-правила

amount, limit и total в HTTP и Go — целые копейки.
В PostgreSQL amount и limit_amount — рубли NUMERIC(14,2).

Пример: amount=450 в JSON сохраняется как 4.50 в PostgreSQL.

Преобразования выполняются через SQL NUMERIC без float64.
Максимальная отдельная сумма или лимит:
99 999 999 999 999 копеек.

Transaction.Validate и Budget.Validate вызываются перед сохранением.

Суммы и лимиты должны быть положительными.
Категория не должна быть пустой или состоять из пробелов.
Дата — корректная YYYY-MM-DD, год не меньше 0001.

Бюджет ограничивает всю историю категории.
При точном достижении лимита транзакция разрешена.
Для категории без бюджета добавление разрешено.

Проверка лимита и INSERT выполняются в одной SQL-транзакции
READ COMMITTED с блокировкой категории.
SetBudget использует ту же блокировку и UPSERT по category.

ID получает PostgreSQL через INSERT RETURNING.
Пропуски значений SERIAL допустимы.

ListBudgets сортирует по category.
ListTransactions сортирует по date DESC, id DESC.
Пустые списки возвращаются как [], а не null.

## Проверка API через cURL в PowerShell

Выполняйте в третьем терминале при работающем Gateway.

Новая категория позволяет повторять сценарий,
не удаляя предыдущие данные.

```powershell
$OutputEncoding = [System.Text.UTF8Encoding]::new($false)
$category = 'еда-' + (Get-Date -Format 'yyyyMMdd-HHmmss')
$api = 'http://localhost:8080/api'

# 200. На новой БД пустые массивы.
curl.exe -i "$api/budgets"
curl.exe -i "$api/transactions"

# 201. Бюджет 1000 копеек.
@{ category=$category; limit=1000 } | ConvertTo-Json -Compress | curl.exe -i -X POST "$api/budgets" -H 'Content-Type: application/json' --data-binary '@-'

# 201. Транзакция 450 копеек.
@{ amount=450; category=$category; description='ланч'; date='2026-10-30' } | ConvertTo-Json -Compress | curl.exe -i -X POST "$api/transactions" -H 'Content-Type: application/json' --data-binary '@-'

# 409. 450 + 600 > 1000. Запись не сохраняется.
@{ amount=600; category=$category; description='ужин'; date='2026-10-30' } | ConvertTo-Json -Compress | curl.exe -i -X POST "$api/transactions" -H 'Content-Type: application/json' --data-binary '@-'

# 400. Сумма должна быть положительной.
@{ amount=0; category=$category; date='2026-10-30' } | ConvertTo-Json -Compress | curl.exe -i -X POST "$api/transactions" -H 'Content-Type: application/json' --data-binary '@-'

curl.exe -i "$api/transactions"
curl.exe -i "$api/budgets"

# 201. Обновление существующей категории без дубля.
@{ category=$category; limit=1500 } | ConvertTo-Json -Compress | curl.exe -i -X POST "$api/budgets" -H 'Content-Type: application/json' --data-binary '@-'

# Два одинаковых запроса отчёта.
curl.exe -i "$api/reports/summary?from=2026-10-01&to=2026-10-31"
curl.exe -i "$api/reports/summary?from=2026-10-01&to=2026-10-31"
```

Для созданной категории сохраняется только транзакция на 450 копеек.
На новой БД total отчёта равен 450.
При повторении сценария total учитывает и прежние записи за период.

## Redis и отчёты

Ключ: report:summary:<from>:<to>.
TTL: 30 секунд.

Первый запрос незакешированного отчёта пишет в лог Gateway:
report cache miss.

Повторный запрос пишет:
report cache hit.

Ответ кешируется целиком в JSON.
Если ключ уже был в Redis, дождитесь истечения 30 секунд
и повторите два запроса.

После новой транзакции отчёт может оставаться прежним до истечения TTL.
Списки бюджетов и транзакций всегда читаются из PostgreSQL.

Если Redis перестал отвечать после запуска,
отчёт рассчитывается из PostgreSQL, ошибка кеша логируется.

Кеш бюджетов не добавлен: он необязателен по заданию.

Просмотр SQL-строк и TTL из корня проекта:

```powershell
docker compose exec postgres psql -U postgres -d cashapp -c 'SELECT id, category, limit_amount FROM budgets ORDER BY id;'

docker compose exec postgres psql -U postgres -d cashapp -c 'SELECT id, amount, category, description, date FROM expenses ORDER BY id;'

docker compose exec redis redis-cli -n 0 TTL report:summary:2026-10-01:2026-10-31
```

TTL положительный, пока ключ существует; -2 означает, что ключ отсутствует.

## Проверка сохранения после перезапуска

1. Создайте бюджет и транзакцию.
2. Сохраните ID из ответа.
3. Остановите Ledger и Gateway через Ctrl+C.
4. Запустите их с теми же переменными окружения.
5. Выполните GET /api/transactions и GET /api/budgets.
6. Прежние записи и ID должны сохраниться.

Для остановки контейнеров без удаления данных:
docker compose stop.

Обычный docker compose down сохраняет именованный том.
docker compose down -v удаляет том с данными;
для проверки сохранения эта команда не нужна.

## Тесты и сборка

Unit-тесты, go vet и сборка обоих модулей:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\check.ps1
```

Unit-тесты используют sqlmock, miniredis и HTTP-заглушку.
Внешние PostgreSQL и Redis для них не нужны.

Интеграционный тест с работающими PostgreSQL и Redis:

```powershell
$env:TEST_DATABASE_URL = 'postgres://postgres:postgres@127.0.0.1:15432/cashapp?sslmode=disable'
$env:REDIS_ADDR = 'localhost:6379'
$env:TEST_REDIS_DB = '15'
$env:REDIS_PASSWORD = ''

powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\check.ps1 -Integration
```

Интеграционный тест создаёт отдельную схему ledger_test_<число>.
Проверяет UPSERT, суммы, сортировку, бюджет, конкурентные вставки,
кеш, TTL и сохранение после переподключения.

В конце удаляет только свою схему.
Redis DB 15 используется для тестов.
Требуется право CREATE SCHEMA; пользователь postgres из compose им обладает.

Для проверки гонок при наличии C-компилятора:

```powershell
cd ledger
go test -race -count=1 ./...
cd ../gateway
go test -race -count=1 ./...
cd ..
```

