-- Katalog model AI (Panel admin → Katalog model): model yang tersedia di sistem per provider.
-- Konfigurasi AI hanya menawarkan model katalog yang aktif. Model dari env (KENARI_*_MODELS,
-- OPENROUTER_*_MODELS, ...) dimasukkan otomatis saat start sebagai model bawaan (is_builtin).
--   kind   → text | image (model gambar juga dipakai edit gambar kalau providernya bisa)
--   status → active (ditawarkan di Konfigurasi AI) | disabled (disembunyikan admin)
CREATE TABLE IF NOT EXISTS ai_models (
    id               BIGSERIAL PRIMARY KEY,
    provider         TEXT NOT NULL,               -- kenari | openrouter | anthropic | groq | ...
    model_id         TEXT NOT NULL,               -- id model di provider (tanpa awalan "<provider>:")
    kind             TEXT NOT NULL,
    name             TEXT NOT NULL DEFAULT '',
    maker            TEXT NOT NULL DEFAULT '',
    status           TEXT NOT NULL DEFAULT 'active',
    is_builtin       BOOLEAN NOT NULL DEFAULT false,
    is_free          BOOLEAN NOT NULL DEFAULT false,
    context_length   INTEGER,
    prompt_price     DOUBLE PRECISION,            -- USD / 1 juta token (dari daftar OpenRouter)
    completion_price DOUBLE PRECISION,
    image_price      DOUBLE PRECISION,            -- USD / gambar (dari daftar OpenRouter)
    notes            TEXT NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS ai_models_provider_kind_model_uniq ON ai_models(provider, kind, model_id);

-- Model OpenRouter tambahan dari versi sebelumnya (app_settings key "openrouter_models")
-- dipindah ke katalog.
INSERT INTO ai_models (provider, kind, model_id, name)
SELECT 'openrouter', 'text', m, m FROM app_settings,
    jsonb_array_elements_text(CASE WHEN jsonb_typeof(value->'text') = 'array' THEN value->'text' ELSE '[]'::jsonb END) AS m
WHERE key = 'openrouter_models'
ON CONFLICT DO NOTHING;
INSERT INTO ai_models (provider, kind, model_id, name)
SELECT 'openrouter', 'image', m, m FROM app_settings,
    jsonb_array_elements_text(CASE WHEN jsonb_typeof(value->'image') = 'array' THEN value->'image' ELSE '[]'::jsonb END) AS m
WHERE key = 'openrouter_models'
ON CONFLICT DO NOTHING;
DELETE FROM app_settings WHERE key = 'openrouter_models';
