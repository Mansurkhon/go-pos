# go-pos

Мини-бэкенд кассы на Go: товары, продажа с атомарным списанием остатков и дневной отчёт.
Учебная Go-версия ядра моей POS-системы Vendor (FastAPI + PostgreSQL).

**Стек:** Go 1.23, Fiber v2, pgx v5, PostgreSQL.

## Что внутри

```
cmd/api/main.go            точка входа: конфиг, пул БД, миграции, graceful shutdown
internal/domain            модели и ошибки (ErrNotFound, ValidationError, ...)
internal/service           бизнес-правила, работает с БД через интерфейс Repository
internal/repository        SQL через pgx: CRUD, транзакция продажи, отчёт
internal/handler           HTTP на Fiber, единый ErrorHandler
internal/migrate           раннер миграций (schema_migrations)
migrations/*.up.sql        схема, встроена в бинарник через go:embed
```

Что стоит посмотреть:

- **Продажа** (`repository.CreateSale`) — одна транзакция: `SELECT ... FOR UPDATE`,
  проверка остатка, `UPDATE`, вставка чека и позиций. Позиции отсортированы по ID,
  чтобы параллельные продажи не ловили deadlock.
- **Деньги** — `int64` в дирамах, без float.
- **Ошибки** — оборачиваются через `%w`, в HTTP-коды их превращает одно место: `handler.ErrorHandler`.
- **Тесты** сервиса используют фейковый репозиторий вместо базы.

## Запуск

Нужны Go 1.23+ и PostgreSQL.

```powershell
createdb -U postgres go_pos
$env:DATABASE_URL = "postgres://postgres:ПАРОЛЬ@localhost:5432/go_pos"
go mod tidy
go run ./cmd/api
```

Тесты:

```powershell
go test ./...
```

## API

Цены — в дирамах: `550` = 5.50 сомони.

| Метод | Путь | Описание |
|---|---|---|
| GET | `/health` | проверка сервера |
| GET | `/api/v1/products` | список товаров |
| POST | `/api/v1/products` | создать товар |
| GET | `/api/v1/products/{id}` | товар по ID |
| PUT | `/api/v1/products/{id}` | изменить товар |
| DELETE | `/api/v1/products/{id}` | удалить (409, если товар уже продавался) |
| POST | `/api/v1/sales` | продажа (409, если не хватает остатка) |
| GET | `/api/v1/reports/daily?date=2026-10-31` | отчёт за день |

Пример:

```powershell
curl.exe -X POST localhost:8080/api/v1/products -H "Content-Type: application/json" -d '{\"name\":\"Хлеб\",\"price\":550,\"stock\":20}'
curl.exe -X POST localhost:8080/api/v1/sales -H "Content-Type: application/json" -d '{\"items\":[{\"product_id\":1,\"qty\":3}]}'
curl.exe "localhost:8080/api/v1/reports/daily"
```

## Что можно добавить дальше

- [ ] Пагинация и поиск в `GET /products` (`?q=&limit=&offset=`)
- [ ] Возврат продажи (`POST /sales/{id}/refund`) — обратная транзакция
- [ ] JWT-авторизация и роли (кассир / владелец) через middleware
- [ ] Интеграционные тесты репозитория на отдельной тестовой БД
- [ ] Dockerfile и docker-compose
- [ ] Страница отчёта на Templ + HTMX
