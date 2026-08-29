-- Migration 0157: Add Wishlist, CartItem, Order, OrderItem tables.
--
-- Wishlist and Cart are new student-facing concepts on the legacy
-- Subject/CourseReview model (the one the education frontend actually
-- reads/writes). Order/OrderItem back multi-course cart checkout and match
-- the shape the admin panel's existing Orders page already expects
-- (d:/admin .../admin/orders/page.tsx predates any backend route for it).

CREATE TABLE IF NOT EXISTS "Wishlist" (
    "id"         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    "user_id"    UUID NOT NULL REFERENCES "User"("id") ON DELETE CASCADE,
    "subject_id" UUID NOT NULL REFERENCES "Subject"("id") ON DELETE CASCADE,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT now(),
    "deleted_at" TIMESTAMPTZ,
    CONSTRAINT "Wishlist_user_subject_unique" UNIQUE ("user_id", "subject_id")
);

CREATE INDEX IF NOT EXISTS "Wishlist_user_id_idx" ON "Wishlist" ("user_id");
CREATE INDEX IF NOT EXISTS "Wishlist_created_at_idx" ON "Wishlist" ("created_at");
CREATE INDEX IF NOT EXISTS "Wishlist_deleted_at_idx" ON "Wishlist" ("deleted_at");

CREATE TABLE IF NOT EXISTS "CartItem" (
    "id"         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    "user_id"    UUID NOT NULL REFERENCES "User"("id") ON DELETE CASCADE,
    "subject_id" UUID NOT NULL REFERENCES "Subject"("id") ON DELETE CASCADE,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT now(),
    "deleted_at" TIMESTAMPTZ,
    CONSTRAINT "CartItem_user_subject_unique" UNIQUE ("user_id", "subject_id")
);

CREATE INDEX IF NOT EXISTS "CartItem_user_id_idx" ON "CartItem" ("user_id");
CREATE INDEX IF NOT EXISTS "CartItem_deleted_at_idx" ON "CartItem" ("deleted_at");

CREATE TABLE IF NOT EXISTS "Order" (
    "id"              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    "order_number"    TEXT NOT NULL UNIQUE,
    "user_id"         UUID NOT NULL REFERENCES "User"("id") ON DELETE CASCADE,
    "status"          TEXT NOT NULL DEFAULT 'PENDING',
    "total"           NUMERIC(19,4) NOT NULL,
    "currency"        TEXT NOT NULL DEFAULT 'EGP',
    "payment_method"  TEXT,
    "transaction_id"  TEXT,
    "coupon_code"     TEXT,
    "discount_amount" NUMERIC(19,4) NOT NULL DEFAULT 0,
    "created_at"      TIMESTAMPTZ NOT NULL DEFAULT now(),
    "updated_at"      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS "Order_user_id_idx" ON "Order" ("user_id");
CREATE INDEX IF NOT EXISTS "Order_status_idx" ON "Order" ("status");
CREATE INDEX IF NOT EXISTS "Order_created_at_idx" ON "Order" ("created_at");

CREATE TABLE IF NOT EXISTS "OrderItem" (
    "id"         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    "order_id"   UUID NOT NULL REFERENCES "Order"("id") ON DELETE CASCADE,
    "subject_id" UUID NOT NULL REFERENCES "Subject"("id"),
    "title"      TEXT NOT NULL,
    "type"       TEXT NOT NULL DEFAULT 'COURSE',
    "price"      NUMERIC(19,4) NOT NULL
);

CREATE INDEX IF NOT EXISTS "OrderItem_order_id_idx" ON "OrderItem" ("order_id");
