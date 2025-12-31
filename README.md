# Task Manager Microservice

این مخزن یک میکروسرویس ساده برای مدیریت تسک‌ها (to-do tasks) است که:
- API RESTful با فریم‌ورک `gin` ارائه می‌دهد
- ذخیره‌سازی در PostgreSQL انجام می‌شود
- متریک‌های پایه برای Prometheus دارد
- مستندات OpenAPI (Swagger) در `docs/openapi.yaml` قرار دارد
- شامل Dockerfile (multi-stage) و `docker-compose.yml` برای اجرای محلی است

در ادامه راهنمای اجرا، ساختار پروژه، نمونهٔ درخواست‌ها، و تصمیمات طراحی را می‌بینید.

---

## نکات سریع (Quickstart)

1) با Docker Compose اجرا کن:
docker-compose up --build

- سرویس اپ روی `http://localhost:8080` و دیتابیس PostgreSQL روی پورت `5432` (local) در دسترس است.
- OpenAPI spec به صورت استاتیک در `http://localhost:8080/docs/openapi.yaml` سرو می‌شود.
- متریک‌های Prometheus در `http://localhost:8080/metrics` در دسترس‌اند.

2) نمونهٔ ایجاد یک تسک (curl):
curl -X POST http://localhost:8080/api/v1/tasks \
  -H "Content-Type: application/json" \
  -d '{"title":"Buy groceries","description":"Milk and eggs"}'

3) لیست تسک‌ها:
curl http://localhost:8080/api/v1/tasks


4) گرفتن یک تسک:
curl http://localhost:8080/api/v1/tasks/<TASK_ID>


---


## تست‌ها

- Unit tests و integration tests در توابع `go test` قابل اجرا هستند.
- برای unit tests از mocking (مثلاً `sqlmock`) استفاده شده و برای integration tests می‌توان از دیتابیس واقعی (مثلاً با `docker-compose up db`) استفاده کرد.

اجرای تمام تست‌ها:
go test ./...

اجرای تنها تست‌های integration (اگر جدا کنید با build tags):
# مثال اگر از tag استفاده شده باشد:
go test -tags=integration ./...

- هدف پوشش تستی: حداقل 70% (unit + integration). برای گزارش پوشش:
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out

---

## مستندات API

فایل OpenAPI در `docs/openapi.yaml` قرار دارد. می‌توانی آن را در Swagger UI بارگذاری کنی یا مستقیم از آدرس زیر درخواست بدهی:
http://localhost:8080/docs/openapi.yaml

مسیرهای اصلی API:
- `POST /api/v1/tasks` — ایجاد تسک
- `GET /api/v1/tasks` — لیست تسک‌ها (پارامترها: `limit`, `offset`, `completed`)
- `GET /api/v1/tasks/{id}` — دریافت یک تسک
- `PUT /api/v1/tasks/{id}` — بروزرسانی (partial)
- `DELETE /api/v1/tasks/{id}` — حذف

---

## Observability

- متریک‌ها با `client_golang` Prometheus در اپ ثبت شده‌اند:
  - `requests_total{method,path,status}` — تعداد درخواست‌ها
  - `request_latency_seconds{method,path}` — هیستوگرام تأخیر
  - `tasks_count` — تعداد فعلی تسک‌ها (بعد از ایجاد/حذف به‌روز می‌شود)
- متریک‌ها در `/metrics` قابل دستیابی‌اند.

---

## ساختار پروژه (بسته‌ها / مسیرها)

- `cmd/taskmanager` — ورودی اصلی برنامه و کانفیگ سرور
- `internal/handler` — http handlers (Gin)
- `internal/service` — منطق بیزینس (validation و قوانین)
- `internal/repositories` — repository (دسترس به PostgreSQL با `sqlx`)
- `internal/model` — مدل دامنه (`Task`)
- `internal/metric` — متریک
- `docs/openapi.yaml` — spec OpenAPI
- `Dockerfile` — multi-stage build
- `docker-compose.yml` — برای اجرای محلی (db + app)
- `migrations/` — (در این پروژه simple migration داخل main است؛ می‌توان SQLها را در این فولدر قرار داد تا توسط Postgres container اجرا شود)

---

## طراحی و تصمیمات کلیدی

- زبان: Go؛ فریمورک HTTP: `gin` 
- سادگی در طراحی: لایه‌بندی `handler -> service -> repository` برای تست‌پذیری و جدایی مسئولیت‌ها
- دسترسی به DB با `sqlx` (نه ORM کامل) برای کنترل دقیق SQL و ساده‌سازی اسکن ساختارها
- UUID برای شناسه‌ها (`github.com/google/uuid`)
- تست‌ها:
  - Unit: mock کردن repository/DB با `sqlmock` یا mock interface
  - Integration: اجرای تست‌ها علیه یک PostgreSQL واقعی (docker)
- OpenAPI: فایل yaml دستی تا کنترل کامل داشته باشیم؛ در آینده می‌شود از ابزارهای خودکار مانند `swag` استفاده کرد
- Observability: Prometheus metrics با نام‌های مشخص؛ tracing پایه از لاگ و request timing

Trade-offs و نکات:
- استفاده از `sqlx` به جای ORM باعث کنترل بیشتر روی کوئری‌ها و performance بهتر می‌شود اما مقدار بیشتری از boilerplate را می‌طلبد.
- برای سادگی اولیه migrationها به صورت inline در برنامه اجرا می‌شوند؛ در محیط تولید بهتر است از ابزار migration (مثل `golang-migrate`) استفاده شود.
- authentication/authorization حذف شده تا MVP سبک و سریع آماده شود. می‌توان آن را با JWT یا OAuth اضافه کرد.

---