# Roadmap Projects API Tests Runner

REST-сервис, который принимает URL деплоя и название проекта, запускает заранее
скомпилированный набор Go-тестов и хранит прогресс и отчёт в PostgreSQL

## Локальный запуск

Для локального запуска нужно создать `.dev.env` файл в корне проекта

Пример - `.example.env`

Если установлен [just](https://just.systems/):

```sh
just
```

Без just:

```sh
docker compose up -d --build
```

Порт для REST `8080` захардкожен в `main.go`

Для тестов:

```sh
just post ${deploy_base_url} ${project}
just get ${id из POST респонса}
```

`project` по умолчанию `CURRENCY_EXCHANGE`

`just post` и `just get` берут `REST_API_KEY` из окружения

Примеры запросов в `justfile`

## REST API

Авторизация `Authorization: Bearer <REST_API_KEY>`

### POST /api/tests

Минимальный запрос:

```json
{
  "deploy_base_url": "http://example.com",
  "project_name": "CURRENCY_EXCHANGE"
}
```

Полный запрос:

```json
{
  "deploy_base_url": "http://example.com",
  "project_name": "CURRENCY_EXCHANGE",
  "telegram_username": "some_user",
  "telegram_user_id": 123456789,
  "github_username": "some_user",
  "github_repository": "some-repo",
  "project_language": "Java"
}
```

Ответ = `TestRun`

### GET /api/tests/{id}

`id` = UUID

Ответ = `TestRun`

### GET /api/tests{?filter=value}

Доступные фильтры; комбинируются через логическое `AND`:

- `deployBaseUrl`
- `telegramUsername`
- `telegramUserId`
- `githubUsername`
- `githubRepository`
- `projectLanguage`
- `projectName`
- `status`

Ответ = массив `TestRun`

### `TestRun`

```json
{
  "id": "6f1c0a52-3d0e-4a53-9a0e-5c1d5f0b9a11",
  "deploy_base_url": "https://example.com",
  "telegram_username": null,
  "telegram_user_id": null,
  "github_username": null,
  "github_repository": null,
  "project_language": null,
  "project_name": "CURRENCY_EXCHANGE",
  "created_at": 1794000000,
  "completed_at": null,
  "status": "PENDING",
  "error": null,
  "report": [
    {
      "name": "currencies/get/status_code_is_200",
      "status": "PASSED",
      "output": "",
      "description": "GET /currencies => HTTP статус код 200",
      "time": 1794000042,
      "elapsed": 12,
      "request": "GET /currencies HTTP/1.1\nHost: example.com\n...",
      "response": "HTTP/1.1 200 OK\n..."
    },
    {
      "name": "currencies/get/status_code_is_200",
      "status": "PASSED",
      "output": "",
      "description": "GET /currencies => HTTP статус код 200",
      "time": 1794000042,
      "elapsed": 12,
      "request": "GET /currencies HTTP/1.1\nHost: example.com\n...",
      "response": "HTTP/1.1 200 OK\n..."
    }
  ],
  "total": 496,
  "passed": 0,
  "failed": 0,
  "skipped": 0
}
```

## Флоу прогона тестов

1. POST /api/tests
2. Запрос валидируется, создаётся запись в БД со статусом `PENDING`
3. В фоне стартует прогон
    1. Статус `RUNNING`, прогресс обновляется в рантайме в памяти
    2. По завершении в БД сохраняются итоговый статус, счётчики и `report`
4. Ответ `201` + `TestRun`

## Тесты

### Сборка

Тестовые бинарники компилируются вместе с REST API в `Dockerfile` и лежат рядом
с `test-runner`

Перед сборкой `go generate` считает `total` по AST

Финальный образ фулл `golang` потому что в рантайме нужен `go tool test2json`

### Запуск

Runner запускает два процесса через пайп:

```sh
./currencyexchange.test -base-url=<deploy_base_url> -dry-run=<bool> -test.v=test2json | go tool test2json
```

### Как писать

```go
func getSomething(t *testing.T) {
        // Сохраняем ошибки
        resp, dumps, err := doRequest(t, http.MethodGet, "/something", nil)
        if err != nil {
            err = fmt.Errorf("не удалось отправить запрос: %w", err)
        }

        // Последняя ошибка и дампы уходят в checker
        c := &checker{t, dumps, err}

        // Все проверки и фейлы только внутри assert
        c.assert("status code is 200", "GET /something => HTTP статус код 200", func(t *testing.T) {
            if resp.StatusCode != http.StatusOK {
                t.Errorf("Ожидался статус код %d, получен %d", http.StatusOK, resp.StatusCode)
            }
        })
}
```

Правила:

1. Все проверки только внутри `c.assert(name, description, fn)`
2. Любые ошибки до assert'ов передаем в `checker`
3. Скипаем проверки только внутри assert'ов
4. Никогда ничего не фейлим вне assert'a ~`t.Fatal`,`t.Error`,`panic`, ранний`return`~
5. Все хелперы уважают `*dryRun == true` и возвращают заглушки

### Счётчик total

`total_counter.go` запускается через `go generate` (директива в `cmd/main.go`):

1. Загружает исходники `tests/...` и считает вызовы `.assert(...)`
2. Вызовы внутри `range` по слайс-литералу умножаются на число элементов
3. Результат по пакету записывается в `internal/test_run/constants.go`

### Проверка при старте

При старте сервиса для каждого проекта выполняется dry-run, и число полученных
результатов сравнивается с `total` сгенерированным через `go generate`

### Добавление нового проекта

1. Пакет `tests/<name>/` с `package <name>_test`
2. Имя пакета и константа в `constantOfPkg` в `total_counter.go`
3. `go generate ./...`
4. Ветка в `newTestSuite` и список проектов в `init()` в `suite.go`
5. Сборка и копирование нового тестового бинарника в `Dockerfile`
