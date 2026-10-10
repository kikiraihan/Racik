.PHONY: help check-env build up down restart logs logs-backend logs-frontend ps psql backup backup-uploads restore dev-backend dev-frontend fmt fmt-check vet test build-backend build-frontend typecheck ci deploy deploy-remote rollback health version clean

COMPOSE ?= docker compose
BACKUP_DIR ?= backups
TIMESTAMP := $(shell date +%Y%m%d-%H%M%S)

# --- Konfigurasi deploy ------------------------------------------------------
# DEPLOY_BRANCH : branch yang di-deploy ke produksi
# SSH_TARGET    : user@host VPS, atau alias dari ~/.ssh/config (untuk deploy-remote)
# SSH_PORT      : port SSH VPS
# REMOTE_DIR    : lokasi repo ini di VPS
DEPLOY_BRANCH ?= main
SSH_TARGET    ?=
SSH_PORT      ?= 22
REMOTE_DIR    ?= /var/www/racik
APP_PORT_ENV   = $(shell [ -f .env ] && grep -E '^[[:space:]]*(export[[:space:]]+)?APP_PORT[[:space:]]*=' .env | tail -1 | sed -E 's/^[^=]*=//; s/\#.*$$//' | tr -d '"'\''[:space:]')
APP_PORT      ?= $(if $(APP_PORT_ENV),$(APP_PORT_ENV),8091)

help: ## Tampilkan daftar perintah
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-16s\033[0m %s\n", $$1, $$2}'

check-env: ## Pastikan file .env sudah ada
	@if [ ! -f .env ]; then \
		echo "File .env belum ada. Menyalin dari .env.example..."; \
		cp .env.example .env; \
		echo "Edit .env dulu (APP_PORT, DB_PASSWORD, JWT_SECRET, CORS_ORIGINS, ADMIN_EMAILS, KENARI_API_KEY, dst), lalu jalankan ulang."; \
		exit 1; \
	fi

build: check-env ## Build image backend + frontend
	$(COMPOSE) build

up: check-env ## Jalankan stack di background (build otomatis jika perlu)
	$(COMPOSE) up -d --build

down: ## Hentikan & hapus container (volume data tetap aman)
	$(COMPOSE) down

restart: down up ## Restart seluruh stack

logs: ## Ikuti log semua service
	$(COMPOSE) logs -f

logs-backend: ## Ikuti log backend saja
	$(COMPOSE) logs -f backend

logs-frontend: ## Ikuti log frontend (Nuxt) saja
	$(COMPOSE) logs -f frontend

ps: ## Status container
	$(COMPOSE) ps

psql: ## Buka psql ke database racik
	$(COMPOSE) exec db psql -U racik -d racik

backup: ## Backup PostgreSQL ke ./backups/racik-<timestamp>.sql.gz
	@mkdir -p $(BACKUP_DIR)
	$(COMPOSE) exec -T db pg_dump -U racik -d racik --clean --if-exists | gzip > $(BACKUP_DIR)/racik-$(TIMESTAMP).sql.gz
	@echo "Backup tersimpan di $(BACKUP_DIR)/racik-$(TIMESTAMP).sql.gz"

backup-uploads: ## Backup file upload/gambar AI ke ./backups/uploads-<timestamp>.tar.gz
	@mkdir -p $(BACKUP_DIR)
	$(COMPOSE) exec -T backend tar czf - -C /data uploads > $(BACKUP_DIR)/uploads-$(TIMESTAMP).tar.gz
	@echo "Backup tersimpan di $(BACKUP_DIR)/uploads-$(TIMESTAMP).tar.gz"

restore: ## Restore PostgreSQL dari backup: make restore FILE=backups/racik-xxxx.sql.gz
	@if [ -z "$(FILE)" ]; then echo "Gunakan: make restore FILE=backups/racik-xxxx.sql.gz"; exit 1; fi
	$(COMPOSE) stop backend
	gzip -cd $(FILE) | $(COMPOSE) exec -T db psql -U racik -d racik -v ON_ERROR_STOP=1 -q
	$(COMPOSE) start backend
	@echo "Restore selesai dari $(FILE), backend di-start ulang."

dev-backend: ## Jalankan backend Go secara lokal (tanpa Docker), membaca .env otomatis
	cd backend && go run ./cmd/server

dev-frontend: ## Jalankan Nuxt dev server secara lokal (tanpa Docker)
	cd frontend && npm run dev

build-backend: ## go build backend secara lokal
	cd backend && go build -o /dev/null ./...

build-frontend: ## npm run build frontend secara lokal
	cd frontend && npm run build

typecheck: ## nuxi typecheck frontend
	cd frontend && npx nuxi typecheck

fmt: ## gofmt seluruh kode backend
	cd backend && gofmt -w .

fmt-check: ## Gagal kalau ada file Go yang belum di-gofmt
	@cd backend && out=$$(gofmt -l .); \
	if [ -n "$$out" ]; then echo "File belum di-gofmt:"; echo "$$out"; exit 1; fi; \
	echo "gofmt OK"

vet: ## go vet backend
	cd backend && go vet ./...

test: ## go test backend
	cd backend && go test ./...

ci: fmt-check vet test build-backend typecheck build-frontend ## Jalankan semua cek CI secara lokal

# --- Deploy ------------------------------------------------------------------

deploy: check-env ## [DI VPS] Pull kode terbaru + backup DB + rebuild + health check + auto-rollback
	DEPLOY_BRANCH=$(DEPLOY_BRANCH) ./scripts/deploy.sh

deploy-remote: ## [DI LOKAL] Trigger deploy di VPS via SSH: make deploy-remote SSH_TARGET=user@ip
	@if [ -z "$(SSH_TARGET)" ]; then \
		echo "Gunakan: make deploy-remote SSH_TARGET=user@ip [SSH_PORT=22] [REMOTE_DIR=/var/www/racik]"; \
		exit 1; \
	fi
	ssh -p $(SSH_PORT) $(SSH_TARGET) "cd $(REMOTE_DIR) && DEPLOY_BRANCH=$(DEPLOY_BRANCH) ./scripts/deploy.sh"

rollback: ## [DI VPS] Kembali ke commit sebelum deploy terakhir (atau: make rollback SHA=abc1234)
	./scripts/rollback.sh $(SHA)

health: ## Cek endpoint health app di localhost VPS
	@curl -fsS --max-time 5 http://127.0.0.1:$(APP_PORT)/api/health && echo "" || \
		(echo "Health check GAGAL di http://127.0.0.1:$(APP_PORT)/api/health"; exit 1)

version: ## Tampilkan commit yang sedang ter-deploy
	@echo "commit  : $$(git rev-parse --short HEAD)"
	@echo "pesan   : $$(git log -1 --format='%s')"
	@echo "tanggal : $$(git log -1 --format='%ci')"
	@echo "branch  : $$(git rev-parse --abbrev-ref HEAD)"

clean: ## HATI-HATI: hapus container, network, DAN volume (database, upload, log ikut terhapus!)
	$(COMPOSE) down -v
