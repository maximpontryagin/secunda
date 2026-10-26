# Secunda

REST API управления задачами команд на Go, chi, MySQL и Redis.

Реализованы инфраструктура, регистрация, JWT-аутентификация, создание и список команд, приглашения участников. Задачи, комментарии, история, кеш списка и SQL-отчет будут добавлены на следующих этапах. Полный план находится в [PLAN.md](PLAN.md).

## Запуск

Требуются Go 1.25 и Docker Compose v2.

```sh
cp .env.example .env
make compose-up
```

Compose запускает MySQL и Redis, применяет миграции goose отдельным контейнером и запускает приложение на `localhost:8080`. Учетные данные в примере предназначены для локального запуска; замените их перед использованием вне локальной среды. `make compose-down` останавливает контейнеры, сохраняя данные MySQL.

Для локального запуска приложения с инфраструктурой в Docker:

```sh
cp .env.example .env
make infra-up
make migrate-up
make run
```

## Конфигурация

Переменные перечислены в [.env.example](.env.example). Приложение и команда миграций читают `.env`; переменные окружения имеют приоритет. Секреты не выводятся в логах.

- `HTTP_ADDR`, `HTTP_*_TIMEOUT`, `SHUTDOWN_TIMEOUT`: адрес HTTP и таймауты в формате Go duration.
- `MYSQL_HOST`, `MYSQL_PORT`, `MYSQL_DATABASE`, `MYSQL_USER`, `MYSQL_PASSWORD`: подключение к MySQL.
- `MYSQL_MAX_OPEN_CONNS`, `MYSQL_MAX_IDLE_CONNS`, `MYSQL_CONN_MAX_LIFETIME`: пул подключений.
- `MYSQL_ROOT_PASSWORD`: пароль для создания локального контейнера MySQL, приложение его не использует.
- `REDIS_HOST`, `REDIS_PORT`, `REDIS_USERNAME`, `REDIS_PASSWORD`, `REDIS_DB`: подключение к Redis. В Compose используется пользователь default, поэтому `REDIS_USERNAME` оставьте пустым.
- `JWT_SECRET`: минимум 32 байта; `JWT_TTL`: срок действия токена, минимум 1 секунда.

Compose подменяет хосты на имена контейнеров, внутренние порты на 3306/6379 и HTTP-адрес на `:8080`. Для локального Go-процесса используются значения из `.env`. Даты в MySQL обрабатываются в UTC.

## Миграции

Используется goose v3; версия библиотеки закреплена в `go.mod`. Команда `cmd/migrate` строит DSN из тех же переменных MySQL, что и приложение. Отдельный DSN с дублированием пароля не требуется.

```sh
make migrate-up
make migrate-status
make migrate-down
make migrate-create NAME=add_task_index
```

`migrate-down` откатывает последнюю миграцию. Откат начальной миграции удаляет таблицы и данные. MySQL выполняет DDL с implicit commit, поэтому миграция помечена `NO TRANSACTION`; при сбое посреди DDL следует проверить состояние схемы перед повторным запуском.

## Разработка

```sh
make tools
make generate
make fmt
make build
make vet
make test
```

Версии swag и mockery закреплены в Makefile; бинарные файлы устанавливаются в игнорируемый `bin/`. Swagger генерируется из аннотаций хендлеров и request/response-структур, моки из интерфейсов в `.mockery.yaml`. Спецификация текущих эндпоинтов: [docs/swagger.yaml](docs/swagger.yaml), [docs/swagger.json](docs/swagger.json). Их можно открыть в Swagger Editor; HTTP-маршрут для Swagger UI пока не подключен.

Слои: handler, service, repository. Каждый слой использует свои структуры и конвертеры. Сервис задает границы транзакции, менеджер помещает ее в контекст, репозиторий получает SQL executor через `transaction.Connection`. Вложенные транзакции явно запрещены, ошибка или паника приводят к откату.


## API пользователей и команд

Все тела запросов передаются с `Content-Type: application/json`. Неизвестные поля отклоняются. Максимальный размер тела: 1 MiB. Email приводится к нижнему регистру, пробелы по краям email и имени удаляются. Пароль содержит от 8 до 72 байт и хранится как bcrypt-хеш.

```sh
curl -sS http://localhost:8080/api/v1/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","password":"local-password","name":"Alice"}'

curl -sS http://localhost:8080/api/v1/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"bob@example.com","password":"local-password","name":"Bob"}'

curl -sS http://localhost:8080/api/v1/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","password":"local-password"}'
```

Из ответа login скопируйте `access_token`. Токен проверяется по подписи HS256, issuer, сроку действия и идентификатору пользователя. Роли команд берутся из БД при проверке доступа.

```sh
TOKEN='вставьте access_token'

curl -sS http://localhost:8080/api/v1/teams \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Backend"}'

curl -sS http://localhost:8080/api/v1/teams \
  -H "Authorization: Bearer $TOKEN"

TEAM_ID=1 # замените на id созданной команды
curl -sS "http://localhost:8080/api/v1/teams/$TEAM_ID/invite" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"email":"bob@example.com","role":"member"}'
```

Создание команды атомарно добавляет текущего пользователя с ролью owner. Список содержит только команды текущего пользователя и его роль в каждой. Пустой список: `{"items":[]}`.

Owner приглашает admin или member; admin приглашает только member. Если роль не указана, используется member. Пользователь должен быть зарегистрирован. Повторное приглашение возвращает 409 и не изменяет роль, в том числе роль owner. Отдельных методов смены роли и удаления участников нет.

Ошибки имеют поля `code` и `message`: 400 для некорректных данных, 401 для отсутствующих/неверных учетных данных, 403 для недостаточной роли, 404 для отсутствующего ресурса или недоступной команды, 409 для повторного email или участия, 413 для большого тела, 415 для неверного Content-Type. Внутренние ошибки возвращаются как 500 без деталей БД.

## Проверки

`make test` запускает unit-тесты с race detector: права приглашений, передача транзакционного контекста, регистрация, login, JWT, bcrypt и HTTP-границы. Моки генерируются mockery, вручную не редактируются.

Для интеграционных тестов нужна локальная MySQL с примененными миграциями:

```sh
make infra-up
make migrate-up
make test-integration
```

Проверяется реальный откат записей пользователя, команды и участника при ошибке callback, нарушении внешнего ключа и панике. Тестовые записи остаются внутри откатываемых транзакций. Обязательный интеграционный тест SQL-отчета будет добавлен на этапе 4.
