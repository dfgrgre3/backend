# Docker Compose - بيئة التطوير الكاملة

هذا الملف يوضح كيفية تشغيل وإيقاف جميع الخدمات (PostgreSQL, Redis, MinIO, Mailpit, Backend, Frontend) باستخدام Docker Compose.

## الخدمات المتوفرة

| الخدمة | الصورة | المنفذ | الوصف |
|--------|--------|--------|-------|
| **PostgreSQL** | `postgres:17.6-alpine` | `5432` | قاعدة البيانات الرئيسية |
| **Redis** | `redis:8-alpine` | `6379` | التخزين المؤقت، تحديد المعدل، العمال (Workers) |
| **MinIO** | `minio/minio:RELEASE.2025-09-07T16-13-09Z` | `9000` (API) / `9001` (Console) | تخزين S3 متوافق |
| **Mailpit** | `axllent/mailpit:v1.24` | `1025` (SMTP) / `8025` (UI) | اختبار البريد الإلكتروني (Dev Profile) |
| **Backend** | مبنى من `Dockerfile` | `8082` | خادم Go API |
| **Frontend** | مبنى من `Dockerfile.frontend` | `3000` | تطبيق Next.js |

## التشغيل السريع

### 1. تشغيل جميع الخدمات

```bash
# الطريقة الموصى بها (عبر Makefile)
make docker-up

# أو مباشرة عبر Docker Compose
docker compose up -d
```

### 2. تشغيل جميع الخدمات مع أدوات التطوير (Mailpit)

```bash
docker compose up -d --profile dev
```

### 3. التحقق من حالة الخدمات

```bash
make docker-ps
# أو
docker compose ps
```

### 4. إيقاف الخدمات (مع الحفاظ على البيانات)

```bash
make docker-down
# أو
docker compose down
```

### 5. إيقاف الخدمات وحذف البيانات (تحذير!)

```bash
make docker-down-v
# أو
docker compose down -v
```

### 6. متابعة السجلات

```bash
make logs
# أو
docker compose logs -f --tail=100
```

### 7. إعادة تشغيل الخدمات

```bash
make docker-restart
# أو
docker compose restart
```

## أهداف Makefile الإضافية

| الهدف | الوصف |
|-------|-------|
| `make backend` | تشغيل خدمة Backend فقط |
| `make frontend` | تشغيل خدمة Frontend فقط |
| `make logs` | متابعة سجلات جميع الخدمات |
| `make redis-cli` | فتح Redis CLI |
| `make postgres` | فتح PostgreSQL shell |

## الإعدادات المخصصة

### ملف `.env.docker`

يمكنك تخصيص الإعدادات (كلمات المرور، المنافذ) عبر ملف `.env.docker`:

```bash
# انسخ المثال
cp .env.docker.example .env.docker

# عدّل القيم حسب الحاجة
```

> **ملاحظة:** كلمات المرور لـ PostgreSQL و MinIO **إلزامية** ولا تحتوي على قيم افتراضية.

عند استخدام ملف `.env.docker`، استخدم:

```bash
docker compose --env-file .env.docker up -d
```

> **ملاحظة:** أهداف Makefile تكتشف تلقائيًا وجود `.env.docker` وتستخدمه.

### إعدادات التطبيق في `.env`

بعد تشغيل الخدمات، عدّل ملف `.env` الخاص بالتطبيق:

```env
# PostgreSQL
DATABASE_URL=postgresql://thanawy:your_password@localhost:5432/thanawy?sslmode=disable

# Redis
REDIS_URL=redis://localhost:6379

# MinIO (S3)
STORAGE_TYPE=s3
S3_ENDPOINT=localhost:9000
S3_ACCESS_KEY=minioadmin
S3_SECRET_KEY=your_minio_password
S3_BUCKET=thanawy
S3_USE_SSL=false
S3_PUBLIC_URL=http://localhost:9000/thanawy

# Mailpit (SMTP)
SMTP_ENABLED=true
SMTP_HOST=localhost
SMTP_PORT=1025
SMTP_USER=
SMTP_PASS=
SMTP_FROM="Thanawy <noreply@thanawy.local>"
```

## الواجهات المتاحة

| الخدمة | الرابط |
|--------|--------|
| **MinIO Console** | http://localhost:9001 |
| **Mailpit UI** | http://localhost:8025 |
| **Backend API** | http://localhost:8082 |
| **Frontend** | http://localhost:3000 |

## الأوامر الشائعة

| الأمر | الوصف |
|-------|-------|
| `docker compose up -d` | تشغيل جميع الخدمات في الخلفية |
| `docker compose up -d --profile dev` | تشغيل جميع الخدمات مع Mailpit |
| `docker compose down` | إيقاف الخدمات (الحفاظ على البيانات) |
| `docker compose down -v` | إيقاف الخدمات وحذف البيانات |
| `docker compose ps` | عرض حالة الخدمات |
| `docker compose logs -f` | متابعة السجلات |
| `docker compose restart` | إعادة تشغيل الخدمات |
| `docker compose pull` | تحديث الصور |
| `docker compose build` | إعادة بناء الصور |

## استكشاف الأخطاء

### تعارض المنفذ 5432 (PostgreSQL محلي)

إذا كان لديك PostgreSQL محلي يعمل على المنفذ 5432:

1. أوقف خدمة PostgreSQL المحلية، أو
2. غيّر المنفذ في `.env.docker`:
   ```env
   POSTGRES_PORT=5433
   ```
   ثم حدّث `DATABASE_URL` في `.env`:
   ```env
   DATABASE_URL=postgresql://thanawy:your_password@localhost:5433/thanawy?sslmode=disable
   ```

### تعارض المنفذ 6379 (Redis محلي)

نفس الحل: غيّر `REDIS_PORT` في `.env.docker` وحدّث `REDIS_URL` في `.env`.

### حذف البيانات

لحذف جميع البيانات (قواعد البيانات، التخزين، البريد):

```bash
docker compose down -v
```

> **تحذير:** هذا الأمر يحذف جميع البيانات نهائيًا. استخدمه بحذر.

## ملاحظات الأمان

- **كلمات المرور**: لا توجد كلمات مرور افتراضية لـ PostgreSQL و MinIO. يجب تعيينها في `.env.docker`.
- **السجلات**: جميع الخدمات محدودة بـ 10MB لكل ملف سجل و 3 ملفات كحد أقصى.
- **الموارد**: جميع الخدمات محدودة بـ 1 CPU و 1GB ذاكرة.
- **النسخ المتعددة**: تم إزالة `container_name` للسماح بتشغيل نسخ متعددة من المشروع.