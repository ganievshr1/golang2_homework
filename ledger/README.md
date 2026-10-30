# Ledger

Ledger содержит бизнес-правила, модели, PostgreSQL-хранилище и Redis-кеш.

Суммы в Go и HTTP — целые копейки.
В PostgreSQL amount и limit_amount — рубли NUMERIC(14,2).

Основные функции:

- Connect(ctx) error;
- Close() error;
- AddTransaction(tx) error;
- CreateTransaction(tx) (Transaction, error);
- ListTransactions() ([]Transaction, error);
- SetBudget(b) error;
- ListBudgets() ([]Budget, error);
- LoadBudgets(r) error;
- GetReportSummary(ctx, from, to) (ReportSummary, error).

Перед записью вызывается Validate.

## Подготовка БД и миграций

Нужны Go 1.23+ и Docker Desktop.

Из корня finance-services:

```powershell
docker compose up -d --wait

$env:DATABASE_URL = 'postgres://postgres:postgres@127.0.0.1:15432/cashapp?sslmode=disable'
$env:REDIS_ADDR = 'localhost:6379'
$env:REDIS_DB = '0'
$env:REDIS_PASSWORD = ''

go install github.com/pressly/goose/v3/cmd/goose@v3.24.1

$goose = Join-Path (go env GOPATH) 'bin\goose.exe'

& $goose -dir ./ledger/migrations postgres "$env:DATABASE_URL" up
if ($LASTEXITCODE -ne 0) {
    throw 'Миграции не применились'
}
```

Compose автоматически создаёт БД cashapp и постоянный том postgres_data.

Откат последней миграции:

```powershell
& $goose -dir ./ledger/migrations postgres "$env:DATABASE_URL" down
```

Полный откат:

```powershell
& $goose -dir ./ledger/migrations postgres "$env:DATABASE_URL" down-to 0
```

Откат удаляет таблицы и данные.

## Конфигурация

DATABASE_URL имеет приоритет над отдельными переменными.

Без DATABASE_URL используются:

- DB_HOST=localhost;
- DB_PORT=5432;
- DB_USER=postgres;
- DB_PASS=postgres;
- DB_NAME=cashapp;
- sslmode=disable.

Redis:

- REDIS_ADDR=localhost:6379;
- REDIS_DB=0;
- REDIS_PASSWORD пустой.

Пул PostgreSQL:

- MaxOpenConns=10;
- MaxIdleConns=5;
- ConnMaxLifetime=30 минут.

После sql.Open выполняется PingContext.
Redis проверяется через Ping.
Ошибка подключения завершает приложение.
Успешные подключения логируются.

## Запуск

Из корня проекта после задания переменных:

```powershell
cd ledger
go mod tidy
go run ./cmd/ledger
```

Процесс ожидает Ctrl+C.
Консольная команда не является HTTP-сервером.

Gateway импортирует Ledger и самостоятельно подключается
к тем же PostgreSQL и Redis.
Переменные окружения повторяйте в каждом новом терминале.

## Кеш отчёта

GetReportSummary агрегирует SUM(amount) GROUP BY category
за включительный интервал дат.

Ключ Redis: report:summary:<from>:<to>.
TTL: 30 секунд.
Формат значения: JSON.

Ответ содержит from, to, total и items.
Каждый элемент items содержит category и amount.
Суммы ответа — копейки.

При cache hit SQL-запрос не выполняется.
При cache miss отчёт рассчитывается из PostgreSQL и сохраняется в Redis.
Если Redis недоступен после запуска, используется PostgreSQL.

Новая запись может появиться в отчёте после истечения TTL.
Списки всегда читаются из PostgreSQL.

После запуска Gateway:

```powershell
curl.exe -i 'http://localhost:8080/api/budgets'
curl.exe -i 'http://localhost:8080/api/transactions'
curl.exe -i 'http://localhost:8080/api/reports/summary?from=2026-10-01&to=2026-10-31'
curl.exe -i 'http://localhost:8080/api/reports/summary?from=2026-10-01&to=2026-10-31'
```

В логах Gateway должны появиться cache miss и cache hit.

## Тесты

В каталоге ledger:

```powershell
go test -count=1 -v -cover ./...
go vet ./...
```

Unit-тесты не требуют внешних сервисов.

Интеграционный тест с работающими PostgreSQL и Redis:

```powershell
$env:TEST_DATABASE_URL = 'postgres://postgres:postgres@127.0.0.1:15432/cashapp?sslmode=disable'
$env:REDIS_ADDR = 'localhost:6379'
$env:TEST_REDIS_DB = '15'
$env:REDIS_PASSWORD = ''

go test -tags=integration -run TestPostgresRedisIntegration -count=1 -v ./...
```

Тест создаёт отдельную PostgreSQL-схему и удаляет только её.
Redis DB 15 выделяется для тестов.

Полный ручной сценарий, проверка перезапуска и команды Git
приведены в корневом README.md.