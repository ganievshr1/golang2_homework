# Finance Services: Gateway HTTP API и Ledger

Требуются Go 1.22 или выше и Git.
Используется стандартная библиотека Go.

Gateway принимает HTTP-запросы и импортирует локальный модуль Ledger.
Ledger содержит модели, валидацию, бюджетные ограничения и хранилища в памяти.

В gateway/go.mod настроены require и replace на ../ledger.
HTTP-слой расположен в gateway/internal/api.

## Запуск API

Из корня finance-services:

```bash
cd gateway
go run .
```

Сервер слушает :8080. Остановка — Ctrl+C.
Отдельно запускать Ledger для работы API не требуется.

При запуске Gateway оба хранилища пусты.
budgets.json автоматически не загружается.
Демонстрационные транзакции не добавляются.
После перезапуска данные исчезают.

Gateway и импортированный Ledger работают в одном процессе.
Отдельный консольный запуск Ledger использует свою память.

## Эндпоинты

| Метод | Путь | Результат |
| --- | --- | --- |
| POST | /api/transactions | 201 и созданная транзакция с ID |
| GET | /api/transactions | 200 и массив транзакций; сначала [] |
| POST | /api/budgets | Создание или обновление бюджета: 201 |
| GET | /api/budgets | 200 и массив бюджетов; сначала [] |
| GET | /ping | 200 и {"message":"pong"} |

Входные поля транзакции:

- amount;
- category;
- description;
- date.

Ответ дополнительно содержит id.

Поля запроса и ответа бюджета:

- category;
- limit.

DTO отделены от доменных моделей Ledger.
Для преобразования используются отдельные функции и методы.

## Формат ответов

Все ответы обработчиков имеют заголовок:

```text
Content-Type: application/json; charset=utf-8
```

Ошибки возвращаются в формате:

```json
{"error":"описание ошибки"}
```

Статусы:

- 400 — некорректный JSON или ошибка валидации;
- 409 — превышение бюджета: {"error":"budget exceeded"};
- 500 — внутренняя ошибка: {"error":"internal server error"};
- 404 — неизвестный путь;
- 405 — неподдерживаемый метод, также возвращается Allow.

Подробности внутренних ошибок записываются в лог.
Middleware логирует метод, путь и длительность каждого запроса.

## Правила данных

- amount и limit — положительные целые числа в копейках.
- Например, 1000 означает 10 рублей.
- Ноль и отрицательные значения запрещены.
- Категория не может быть пустой или состоять только из пробелов.
- Категории сравниваются точно, с учётом регистра и пробелов.
- date — корректная календарная дата YYYY-MM-DD.
- description может быть пустой.
- Сумма транзакций категории может равняться бюджету.
- Превышение бюджета отклоняется.
- Для категории без бюджета проверяются только поля транзакции.
- Отклонённая транзакция не сохраняется и не расходует ID.
- Обновление бюджета сохраняет историю транзакций.
- Неизвестные поля JSON и дополнительные JSON-значения отклоняются.
- Максимальный размер тела — 1 МиБ; превышение возвращает 400.

Validate возвращает ошибки без логирования и изменения данных.
AddTransaction и SetBudget повторно проверяют объекты перед сохранением.

Все обращения Gateway к Ledger проходят через адаптер с общей блокировкой.
Добавление транзакции и получение её ID выполняются под одной блокировкой.

При прямом использовании функций Ledger вне Gateway
вызовы должны быть последовательными.

## Проверка cURL в Bash / WSL

Выполняйте во втором терминале при запущенном Gateway.
Начинайте с нового процесса сервера.

```bash
# 200 и []
curl -i http://localhost:8080/api/transactions
curl -i http://localhost:8080/api/budgets

# 201: бюджет 1000 копеек
curl -i -X POST http://localhost:8080/api/budgets \
  -H 'Content-Type: application/json' \
  -d '{"category":"еда","limit":1000}'

# 201: транзакция с ID 1
curl -i -X POST http://localhost:8080/api/transactions \
  -H 'Content-Type: application/json' \
  -d '{"amount":450,"category":"еда","description":"ланч","date":"2026-10-18"}'

# 409: 450 + 600 > 1000
curl -i -X POST http://localhost:8080/api/transactions \
  -H 'Content-Type: application/json' \
  -d '{"amount":600,"category":"еда","description":"ужин","date":"2026-10-18"}'

# 400: сумма должна быть положительной
curl -i -X POST http://localhost:8080/api/transactions \
  -H 'Content-Type: application/json' \
  -d '{"amount":0,"category":"еда","date":"2026-10-18"}'

# В списке остаётся одна транзакция на 450 копеек
curl -i http://localhost:8080/api/transactions
curl -i http://localhost:8080/api/budgets

# 201: обновление существующего бюджета
curl -i -X POST http://localhost:8080/api/budgets \
  -H 'Content-Type: application/json' \
  -d '{"category":"еда","limit":1500}'
```

## Проверка cURL в Windows PowerShell

Используйте curl.exe.
JSON передаётся через stdin для корректной обработки кавычек.

Если ранее выполняли пример Bash, перезапустите Gateway,
чтобы начать с пустого хранилища.

```powershell
$OutputEncoding = [System.Text.UTF8Encoding]::new($false)

curl.exe -i http://localhost:8080/api/transactions
curl.exe -i http://localhost:8080/api/budgets

'{"category":"еда","limit":1000}' | curl.exe -i -X POST http://localhost:8080/api/budgets -H 'Content-Type: application/json' --data-binary '@-'

'{"amount":450,"category":"еда","description":"ланч","date":"2026-10-18"}' | curl.exe -i -X POST http://localhost:8080/api/transactions -H 'Content-Type: application/json' --data-binary '@-'

'{"amount":600,"category":"еда","description":"ужин","date":"2026-10-18"}' | curl.exe -i -X POST http://localhost:8080/api/transactions -H 'Content-Type: application/json' --data-binary '@-'

'{"amount":0,"category":"еда","date":"2026-10-18"}' | curl.exe -i -X POST http://localhost:8080/api/transactions -H 'Content-Type: application/json' --data-binary '@-'

curl.exe -i http://localhost:8080/api/transactions
curl.exe -i http://localhost:8080/api/budgets
```

Ожидаемые статусы по порядку:

200, 200, 201, 201, 409, 400, 200, 200.

В последнем списке транзакций должна остаться одна операция на 450 копеек.

## Демонстрация предыдущего задания Ledger

Из корня проекта:

```bash
cd ledger
go run ./cmd/ledger
```

Запускайте из ledger, где находится budgets.json.

RunDemo показывает CheckValid для Transaction и Budget,
загружает бюджеты через os.Open и bufio.NewReader,
сохраняет пять транзакций и отклоняет четыре.

LoadBudgets проверяет весь JSON-массив до записи.
При ошибке существующие бюджеты не изменяются.

В корне Ledger команда go run . больше не используется:
корневой пакет теперь импортируется Gateway.

## Тестирование и сборка

Из корня проекта:

```bash
gofmt -w gateway ledger

cd ledger
go test -count=1 -v -cover ./...
go vet ./...
go build -o ../bin/ledger.exe ./cmd/ledger

cd ../gateway
go test -count=1 -v -cover ./...
go vet ./...
go build -o ../bin/gateway.exe .

cd ..
```

Сборка выполняется для текущей ОС.
В Linux можно убрать суффикс .exe.

Общего go.mod в корне нет:
проверки выполняются в каждом модуле отдельно.

При доступном C-компиляторе можно дополнительно выполнить
go test -race ./... в gateway.

Тесты проверяют:

- преобразование DTO и назначенные ID;
- пустые списки;
- создание и обновление бюджетов;
- валидацию и некорректный JSON;
- ответы 400, 409 и 500;
- заголовки и формат ошибок;
- логирование;
- отсутствие сохранения при отказе;
- конкурентные запросы.

Ошибки 500 воспроизводятся тестовой заглушкой хранилища.