CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT NOT NULL UNIQUE,
    name          TEXT NOT NULL DEFAULT '',
    password_hash TEXT NOT NULL,
    is_guest      BOOLEAN NOT NULL DEFAULT false, -- akun tamu, agar user bisa langsung generate tanpa daftar
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Satu desain berisi banyak halaman. Kolom data menyimpan JSON:
-- { "pages": [ { "id": "...", "name": "...", "fabric": { ...canvas JSON fabric.js... } } ] }
CREATE TABLE IF NOT EXISTS designs (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title      TEXT NOT NULL DEFAULT 'Desain tanpa judul',
    width      INT  NOT NULL DEFAULT 1080,
    height     INT  NOT NULL DEFAULT 1080,
    data       JSONB NOT NULL DEFAULT '{"pages":[]}'::jsonb,
    thumbnail  TEXT NOT NULL DEFAULT '',
    starred    BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS designs_user_updated_idx ON designs(user_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS assets (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    filename   TEXT NOT NULL,
    mime       TEXT NOT NULL,
    size       BIGINT NOT NULL,
    path       TEXT NOT NULL,
    source     TEXT NOT NULL DEFAULT 'upload', -- upload | ai
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS assets_user_idx ON assets(user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS templates (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug       TEXT NOT NULL UNIQUE,
    title      TEXT NOT NULL,
    category   TEXT NOT NULL,
    width      INT  NOT NULL,
    height     INT  NOT NULL,
    spec       JSONB NOT NULL, -- disimpan dalam format "design spec" (sama dengan output AI)
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS brand_kits (
    user_id    UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    name       TEXT NOT NULL DEFAULT 'Brand saya',
    colors     JSONB NOT NULL DEFAULT '[]'::jsonb,
    fonts      JSONB NOT NULL DEFAULT '[]'::jsonb,
    logo_url   TEXT NOT NULL DEFAULT '',
    voice      TEXT NOT NULL DEFAULT '', -- gaya bahasa brand, dipakai AI
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Riwayat generate AI + rating user. Rating dipakai sebagai contoh (few-shot)
-- pada prompt berikutnya agar hasil desain makin sesuai selera user.
CREATE TABLE IF NOT EXISTS ai_generations (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind       TEXT NOT NULL, -- design | image | copy | refine | edit
    prompt     TEXT NOT NULL,
    model      TEXT NOT NULL,
    result     JSONB NOT NULL DEFAULT '{}'::jsonb,
    rating     SMALLINT, -- 1 (suka) / -1 (tidak suka)
    feedback   TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS ai_gen_user_idx ON ai_generations(user_id, kind, rating, created_at DESC);
