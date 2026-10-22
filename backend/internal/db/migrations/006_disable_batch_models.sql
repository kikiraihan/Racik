-- Varian OpenRouter ":batch" (mis. anthropic/claude-sonnet-5.5:batch) hanya bisa dipakai lewat
-- Batch API dan ditolak di /chat/completions (HTTP 404). Model seperti ini yang terlanjur masuk
-- Katalog model dinonaktifkan; daftar /models OpenRouter tidak menampilkannya lagi.
UPDATE ai_models
SET status = 'disabled',
    notes = CASE WHEN notes = '' THEN 'Dinonaktifkan otomatis: varian :batch tidak bisa dipakai lewat chat/completions'
                 ELSE notes END,
    updated_at = now()
WHERE model_id LIKE '%:batch' AND status <> 'disabled';
