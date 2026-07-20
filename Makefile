.PHONY: api gorm-gen init-admin backend-test backend-build frontend-install frontend-test frontend-build frontend-e2e compose-config compose-up compose-down

api:
	buf lint
	buf generate

gorm-gen:
	GOCACHE=/tmp/go-build go run ./backend/cmd/gormgen

init-admin:
	GOCACHE=/tmp/go-build go run ./backend/cmd/initadmin

backend-test:
	GOCACHE=/tmp/go-build go test -race ./backend/...

backend-build:
	GOCACHE=/tmp/go-build go build ./backend/cmd/api ./backend/cmd/worker

frontend-install:
	cd frontend && pnpm install

frontend-test:
	cd frontend && pnpm test:run

frontend-build:
	cd frontend && pnpm build

frontend-e2e:
	cd frontend && pnpm e2e

compose-config:
	docker compose --env-file .env -f deploy/docker-compose.yml config --quiet

compose-up:
	docker compose --env-file .env -f deploy/docker-compose.yml up --build

compose-down:
	docker compose --env-file .env -f deploy/docker-compose.yml down
