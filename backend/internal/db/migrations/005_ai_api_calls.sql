-- AI API Call Log (Panel admin → AI API Call Log, pola sama dengan tiara): satu baris per
-- request HTTP ke vendor AI (termasuk percobaan ulang & model cadangan). Ditulis asinkron
-- oleh applog.APICalls, dihapus otomatis setelah AI_CALL_LOG_RETENTION_DAYS hari.
CREATE TABLE IF NOT EXISTS ai_api_calls (
    id                BIGSERIAL PRIMARY KEY,
    provider          TEXT NOT NULL,               -- kenari | openrouter | anthropic | groq | ...
    endpoint          TEXT NOT NULL,               -- path URL, mis. /v1/chat/completions
    model             TEXT NOT NULL,               -- model yang benar-benar dikirim ke vendor
    operation         TEXT NOT NULL,               -- chat | image | edit
    feature           TEXT,                        -- fitur yang memanggil (ai.design, ai.refine, ...)
    user_id           UUID,                        -- user yang memicu (tanpa FK: log tetap ada walau user dihapus)
    request_id        TEXT,                        -- sama dengan request id di activity log / log server
    http_status       INTEGER,                     -- NULL = tidak sampai mendapat respons HTTP
    is_success        BOOLEAN NOT NULL DEFAULT false,
    latency_ms        INTEGER,
    prompt_tokens     INTEGER,
    completion_tokens INTEGER,
    total_tokens      INTEGER,
    cost              DOUBLE PRECISION,            -- USD, kalau dilaporkan vendor (OpenRouter)
    vendor_price      DOUBLE PRECISION,            -- Rupiah per hit dari Konfigurasi harga (hanya yang sukses)
    error_message     TEXT,
    request_body      JSONB,
    response_body     JSONB,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS ai_api_calls_created_at_idx ON ai_api_calls(created_at);
CREATE INDEX IF NOT EXISTS ai_api_calls_feature_idx ON ai_api_calls(feature);
CREATE INDEX IF NOT EXISTS ai_api_calls_model_idx ON ai_api_calls(model);
CREATE INDEX IF NOT EXISTS ai_api_calls_user_idx ON ai_api_calls(user_id);
