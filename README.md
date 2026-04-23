# Task Service

Сервис для управления задачами с HTTP API на Go.

## Требования

- Go `1.23+`
- Docker и Docker Compose

## Быстрый запуск через Docker Compose

```bash
docker compose up --build
```

После запуска сервис будет доступен по адресу `http://localhost:8080`.

Если `postgres` уже запускался ранее со старой схемой, пересоздай volume:

```bash
docker compose down -v
docker compose up --build
```

Причина в том, что SQL-файл из `migrations/0001_create_tasks.up.sql` монтируется в `docker-entrypoint-initdb.d` и применяется только при инициализации пустого data volume.

## Swagger

Swagger UI:

```text
http://localhost:8080/swagger/
```

OpenAPI JSON:

```text
http://localhost:8080/swagger/openapi.json
```

## API

Базовый префикс API:

```text
/api/v1
```

Основные маршруты:

- `POST /api/v1/tasks`
- `GET /api/v1/tasks`
- `GET /api/v1/tasks/{id}`
- `PUT /api/v1/tasks/{id}`
- `DELETE /api/v1/tasks/{id}`

## Периодические задачи

`POST /api/v1/tasks` теперь поддерживает блок `recurrence`.

Поддерживаемые типы:

- `daily` - задача на каждый `n`-й день
- `monthly` - задача на конкретное число месяца от `1` до `30`
- `specific_dates` - задача только на указанные даты
- `day_parity` - задача только на четные или только на нечетные дни месяца

Формат даты в API: `YYYY-MM-DD`.

Пример ежедневной задачи:

```json
{
  "title": "Обзвон пациентов",
  "description": "Утренний обзвон",
  "status": "new",
  "recurrence": {
    "type": "daily",
    "every_n_days": 2,
    "starts_on": "2026-04-22"
  }
}
```

Пример задачи на конкретные даты:

```json
{
  "title": "Инвентаризация",
  "description": "Проверка остатков",
  "status": "new",
  "recurrence": {
    "type": "specific_dates",
    "dates": ["2026-04-25", "2026-05-10", "2026-05-25"]
  }
}
```

Для `PUT /api/v1/tasks/{id}` и `DELETE /api/v1/tasks/{id}` можно передавать query-параметр `scope`.

Поддерживаемые значения:

- `this` - только текущая задача
- `all` - вся серия
- `this_and_following` - пока зарезервирован, возвращает ошибку `400`
