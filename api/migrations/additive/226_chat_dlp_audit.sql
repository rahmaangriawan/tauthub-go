-- 226_chat_dlp_audit.sql
-- Anti-bypass obrolan klien ↔ mitra:
--   1. Dual-store pesan: original_body (asli, hanya super admin) + body (versi
--      tersensor yang tampil ke peserta), plus flag pelanggaran.
--   2. Flag tingkat ruang (chat_threads.is_flagged) untuk badge di Log Obrolan.
--   3. Audit trail khusus akses admin ke log obrolan, dengan debounce 15 menit
--      (access_count + last_accessed_at) supaya baris tidak menumpuk.

ALTER TABLE public.chat_messages ADD COLUMN IF NOT EXISTS original_body text;
UPDATE public.chat_messages SET original_body = body WHERE original_body IS NULL;
ALTER TABLE public.chat_messages ALTER COLUMN original_body SET NOT NULL;

ALTER TABLE public.chat_messages ADD COLUMN IF NOT EXISTS is_flagged boolean NOT NULL DEFAULT false;
ALTER TABLE public.chat_threads ADD COLUMN IF NOT EXISTS is_flagged boolean NOT NULL DEFAULT false;

CREATE INDEX IF NOT EXISTS chat_messages_flagged_idx
    ON public.chat_messages (thread_id) WHERE is_flagged;

CREATE TABLE IF NOT EXISTS public.admin_chat_audit_logs (
    id               character(36) PRIMARY KEY DEFAULT gen_random_uuid()::text,
    admin_id         character(36) NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    chat_room_id     character(36) NOT NULL REFERENCES public.chat_threads(id) ON DELETE CASCADE,
    ip_address       varchar(45),
    user_agent       varchar(500),
    action           varchar(40) NOT NULL DEFAULT 'VIEW_CHAT_LOG',
    accessed_at      timestamptz NOT NULL DEFAULT now(),
    last_accessed_at timestamptz NOT NULL DEFAULT now(),
    access_count     integer NOT NULL DEFAULT 1
);

-- Pencarian baris debounce: admin + room + action, terbaru dulu.
CREATE INDEX IF NOT EXISTS admin_chat_audit_debounce_idx
    ON public.admin_chat_audit_logs (admin_id, chat_room_id, action, last_accessed_at DESC);
