-- Migration 0158: Add Payment.order_id, linking a payment back to the cart
-- Order it belongs to (nil for a direct single-course checkout).
--
-- Needed so the Paymob webhook can resolve and update Order.status once
-- every OrderItem's Payment row has been settled — a multi-course cart
-- checkout creates one Payment per item, all sharing one Paymob order, but
-- Payment previously had no way to point back to its Order.

ALTER TABLE "Payment" ADD COLUMN IF NOT EXISTS "order_id" UUID REFERENCES "Order"("id") ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS "Payment_order_id_idx" ON "Payment" ("order_id");
