-- Sign in / sign up dengan Google.

-- sub (id akun Google) dari ID token. Dipakai sebagai kunci login Google karena tetap sama
-- walau user mengganti email Google-nya. NULL = belum pernah login dengan Google.
ALTER TABLE users ADD COLUMN IF NOT EXISTS google_sub TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS users_google_sub_idx ON users(google_sub) WHERE google_sub IS NOT NULL;
-- Akun yang daftar lewat Google tidak punya password: password_hash diisi '' (login
-- email+password selalu ditolak sampai user memasang password).
