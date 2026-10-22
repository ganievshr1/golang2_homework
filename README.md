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

## Тестирование и покрытие

В проекте два отдельных модуля: `ledger` и `gateway`.
В корне `finance-services` нет `go.mod`, поэтому `go test ./...`
нужно запускать внутри каждого модуля. Устанавливать testify не требуется:
все проверки используют стандартную библиотеку Go.

Из корня `finance-services` в Bash / WSL:

```bash
set -e

cd ledger
go test ./... -count=1 -v -cover -coverprofile=cover.out
go tool cover -func=cover.out
go tool cover -html=cover.out -o cover.html

cd ../gateway
go test ./... -count=1 -v -cover -coverprofile=cover.out
go tool cover -func=cover.out
go tool cover -html=cover.out -o cover.html

cd ..
```

После запуска созданы четыре файла:

- `ledger/cover.out` — профиль покрытия Ledger;
- `ledger/cover.html` — HTML-отчёт Ledger;
- `gateway/cover.out` — профиль покрытия Gateway;
- `gateway/cover.html` — HTML-отчёт Gateway.

Откройте `cover.html` в браузере. В WSL можно открыть текущую папку
командой `explorer.exe .` и дважды щёлкнуть по HTML-файлу.
`cover.out` и `cover.html` добавлены в `.gitignore`: они создаются заново
указанными командами и не являются исходным кодом.

Фактически измеренное покрытие на Go 1.22.12, Linux amd64:

| Область | Покрытие операторов |
| --- | ---: |
| Пакет `ledger` (домен и консольная демонстрация) | 98,0% |
| Пакет `gateway/internal/api` | 89,1% |
| Пакет `gateway` (`main.go`) | 25,0% |
| Пакет `ledger/cmd/ledger` (`main.go`) | 0,0% |
| Модуль Ledger целиком (`total` в `ledger/cover.out`) | 96,2% |
| Модуль Gateway целиком (`total` в `gateway/cover.out`) | 86,7% |

Итого: покрытие тестами модуля Ledger — около 96%, Gateway — около 87%.
Проценты модулей учитывают количество операторов, а не среднее арифметическое
процентов пакетов. Точки входа `main()` не вызываются этими тестами.
При стандартном запуске покрытие пакета API считается по его собственным
тестам; проверки маршрутов из `gateway/main_test.go` не повышают этот показатель.

### Что проверяют тесты

- `ledger/ledger_test.go`: табличные тесты `Transaction.Validate()` и
  `Budget.Validate()`, а также очистка обоих хранилищ через `Reset()`.
- `ledger/transactions_test.go`: сохранение, назначение ID, копирование списка,
  отказ при невалидных данных, бюджет 5000, точное достижение лимита,
  одиночное и суммарное превышение, категория без бюджета, переполнение `int`.
- `ledger/budgets_test.go`: создание и обновление бюджета без потери истории,
  загрузка JSON и отсутствие частичной записи при ошибке.
- `ledger/validation_test.go`: делегирование через `CheckValid`, проверка `nil`
  и отсутствие сохранения объектов при валидации.
- `gateway/internal/api/handler_test.go`: POST/GET бюджетов и транзакций через
  настоящие обработчики и Ledger, ответы 201/200/400/409, точный JSON ошибки
  превышения бюджета, отсутствие записи при отказе, поля JSON и Content-Type.
  Сохранены проверки 500 на заглушке, логирования и конкурентных запросов.
- `gateway/main_test.go`: подключение роутера, пустые списки, `/ping`, 404, 405.

Проверки организованы через `t.Run`, в том числе `ok`, `exceeded`, `bad_json`.
Только независимые unit-тесты `Validate()` используют `t.Parallel()`.

`ledger.Reset()` очищает транзакции и бюджеты. Unit-тесты состояния вызывают
его через `resetTransactions(t)`, HTTP-тесты — через `resetLedger(t)`;
оба помощника выполняют сброс до сценария и регистрируют его в `t.Cleanup()`.
В тестах пакета `gateway` сброс вызывается напрямую.

`Reset()` не синхронизирован с запросами Gateway: вызывайте его только при
отсутствии активных обращений к Ledger. Интеграционные сценарии не используют
`t.Parallel()`. В тесте конкурентных запросов все горутины завершаются до сброса.

### Проверка изоляции и гонок

При наличии C-компилятора (например, GCC) из корня проекта:

```bash
set -e
(cd ledger && go test ./... -race -shuffle=on -count=3)
(cd gateway && go test ./... -race -shuffle=on -count=3)
```

Эта проверка пройдена: повторные запуски и случайный порядок не влияют на результат,
гонки данных не обнаружены.

### Проверка и сборка

Из корня проекта:

```bash
set -e
gofmt -w ledger gateway
(cd ledger && go vet ./... && go build -o ../bin/ledger ./cmd/ledger)
(cd gateway && go vet ./... && go build -o ../bin/gateway .)
```

Сборка выполняется для текущей ОС. В Windows можно указать имена
`ledger.exe` и `gateway.exe`. Сборка и `go vet` обоих модулей проверены.