# Gateway

Gateway предоставляет HTTP API на :8080.
Он импортирует локальный модуль Ledger через require и replace.

Ledger хранит данные в PostgreSQL.
Отчёты кешируются в Redis на 30 секунд.
Контракты бюджетов и транзакций сохранены.

## Подготовка и запуск

Нужны Go 1.23+, Docker Desktop и Goose.

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
    throw 'Не применились миграции'
}

cd gateway
go mod tidy
go run .
```

Compose создаёт БД cashapp и запускает PostgreSQL/Redis.

Gateway вызывает ledger.Connect в собственном процессе.
Консольная команда Ledger не является отдельным HTTP-сервером.

Для запуска консольного Ledger используйте go run ./cmd/ledger
из каталога ledger с теми же переменными окружения.

Переменные нужно задавать в каждом новом терминале.

Откат последней миграции из корня:

```powershell
& $goose -dir ./ledger/migrations postgres "$env:DATABASE_URL" down
```

Откат удаляет таблицу expenses и её данные.
Полный down-to 0 удаляет также budgets.

## Эндпоинты

| Метод | Путь | Успех |
|---|---|---|
| POST | /api/budgets | 201 и бюджет |
| GET | /api/budgets | 200 и массив |
| POST | /api/transactions | 201 и транзакция с ID |
| GET | /api/transactions | 200 и массив |
| GET | /api/reports/summary?from=YYYY-MM-DD&to=YYYY-MM-DD | 200 и отчёт |
| GET | /ping | 200 и {"message":"pong"} |

Пустые списки возвращаются как [].

Суммы API — целые копейки.
Нулевые и отрицательные суммы/лимиты запрещены.

Ответы: application/json; charset=utf-8.

Ошибки:

- 400 — JSON, валидация или период;
- 409 — {"error":"budget exceeded"};
- 500 — {"error":"internal server error"};
- 404 — неизвестный путь;
- 405 — неподдерживаемый метод.

Подробности внутренних ошибок записываются в лог.

## Проверка через cURL

Во втором терминале PowerShell:

```powershell
$OutputEncoding = [System.Text.UTF8Encoding]::new($false)
$category = 'еда-' + (Get-Date -Format 'yyyyMMdd-HHmmss')
$api = 'http://localhost:8080/api'

@{ category=$category; limit=1000 } | ConvertTo-Json -Compress | curl.exe -i -X POST "$api/budgets" -H 'Content-Type: application/json' --data-binary '@-'

@{ amount=450; category=$category; description='ланч'; date='2026-10-30' } | ConvertTo-Json -Compress | curl.exe -i -X POST "$api/transactions" -H 'Content-Type: application/json' --data-binary '@-'

@{ amount=600; category=$category; date='2026-10-30' } | ConvertTo-Json -Compress | curl.exe -i -X POST "$api/transactions" -H 'Content-Type: application/json' --data-binary '@-'

curl.exe -i "$api/transactions"
curl.exe -i "$api/budgets"

curl.exe -i "$api/reports/summary?from=2026-10-01&to=2026-10-31"
curl.exe -i "$api/reports/summary?from=2026-10-01&to=2026-10-31"
```

Ожидаемые статусы:
201, 201, 409, 200, 200, 200, 200.

В новой категории сохраняется только транзакция на 450 копеек.
На новой БД total отчёта равен 450.

Для незакешированного периода первый запрос пишет cache miss,
следующий — cache hit.
Если ключ уже существовал, дождитесь истечения 30 секунд.

При новой записи отчёт обновляется после TTL.
Списки читаются из PostgreSQL сразу.

## Проверка сохранности

Остановите Gateway и консольный Ledger через Ctrl+C.
Запустите их повторно с тем же DATABASE_URL.

GET /api/transactions и GET /api/budgets должны вернуть
прежние записи и ID.

## Тесты

В каталоге gateway:

```powershell
go test -count=1 -v -cover ./...
go vet ./...
```

Полная проверка PostgreSQL/Redis и команды Git
приведены в корневом README.md.