-- Migration 0178: Add DirectMessage table (one-to-one community chat).
--
-- Backs the /api/v1/chat endpoints (GetChatConversations, GetChatMessages,
-- SendChatMessage). Identity is session-scoped: SenderID is always stamped
-- from the JWT on the server, ReceiverID is validated to exist — no
-- client-supplied sender identity is ever trusted.

CREATE TABLE IF NOT EXISTS "DirectMessage" (
    "id"          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    "sender_id"   UUID NOT NULL REFERENCES "User"("id") ON DELETE CASCADE,
    "receiver_id" UUID NOT NULL REFERENCES "User"("id") ON DELETE CASCADE,
    "content"     TEXT NOT NULL,
    "is_read"     BOOLEAN NOT NULL DEFAULT false,
    "read_at"     TIMESTAMPTZ,
    "created_at"  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT "DirectMessage_no_self_dm_check" CHECK ("sender_id" <> "receiver_id")
);

-- Conversation thread lookup (both directions) ordered by recency.
CREATE INDEX IF NOT EXISTS "idx_dm_sender_receiver_created" ON "DirectMessage" ("sender_id", "receiver_id", "created_at");
CREATE INDEX IF NOT EXISTS "idx_dm_receiver_sender_created" ON "DirectMessage" ("receiver_id", "sender_id", "created_at");

-- Unread badge count per receiver.
CREATE INDEX IF NOT EXISTS "idx_dm_receiver_read" ON "DirectMessage" ("receiver_id", "is_read") WHERE "is_read" = false;
