-- Akses admin, saldo (top up) & tarif AI.

-- role: user | admin. Admin mengatur model AI, harga, dan saldo user.
ALTER TABLE users ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT 'user';
-- Saldo dalam Rupiah (bilangan bulat). Dipotong setiap kali memakai fitur AI.
ALTER TABLE users ADD COLUMN IF NOT EXISTS balance BIGINT NOT NULL DEFAULT 0;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_balance_nonneg;
ALTER TABLE users ADD CONSTRAINT users_balance_nonneg CHECK (balance >= 0);

-- Buku besar saldo: setiap perubahan saldo tercatat di sini (tidak pernah diubah/dihapus).
CREATE TABLE IF NOT EXISTS wallet_transactions (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount        BIGINT NOT NULL,          -- + masuk (top up/refund), - keluar (pemakaian AI)
    balance_after BIGINT NOT NULL,
    kind          TEXT NOT NULL,            -- topup | charge | refund | adjust
    feature       TEXT NOT NULL DEFAULT '', -- design | refine | image | edit | copy (untuk charge/refund)
    description   TEXT NOT NULL DEFAULT '',
    request_id    TEXT NOT NULL DEFAULT '', -- korelasi dengan system log
    actor_id      UUID REFERENCES users(id) ON DELETE SET NULL, -- admin yang melakukan top up/adjust
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS wallet_tx_user_idx ON wallet_transactions(user_id, created_at DESC);

-- Pengaturan aplikasi yang diatur admin dari UI, mis.
--   ai      → {"textModel": "...", "imageModel": "...", "editModel": "..."}
--   pricing → {"markupPercent": 5, "freeHitPrice": 100}
CREATE TABLE IF NOT EXISTS app_settings (
    key        TEXT PRIMARY KEY,
    value      JSONB NOT NULL,
    updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Harga vendor per model (id "<provider>:<model>"), per hit (1 panggilan teks / 1 gambar).
-- Harga ke user = vendor_price + markup %; vendor_price 0 (model gratis) = tarif flat freeHitPrice.
CREATE TABLE IF NOT EXISTS model_prices (
    model          TEXT PRIMARY KEY,
    vendor_price   NUMERIC(14,2) NOT NULL DEFAULT 0,
    markup_percent NUMERIC(6,2),            -- NULL = pakai markup global
    updated_by     UUID REFERENCES users(id) ON DELETE SET NULL,
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
