.PHONY: api wire gorm-gen init-admin backend-test backend-vet backend-build frontend-install frontend-test frontend-build frontend-e2e compose-config compose-up compose-down

api:
	buf lint
	buf generate

wire:
	GOCACHE=/tmp/go-build go tool wire ./app/admin/cmd/server ./app/worker/cmd/worker

gorm-gen:
	GOCACHE=/tmp/go-build go run ./app/admin/cmd/gormgen

init-admin:
	GOCACHE=/tmp/go-build go run ./app/admin/cmd/initadmin

backend-test:
	GOCACHE=/tmp/go-build go test -race ./app/... ./internal/...

backend-vet:
	GOCACHE=/tmp/go-build go vet ./app/... ./internal/...

backend-build:
	GOCACHE=/tmp/go-build go build ./app/admin/cmd/server ./app/admin/cmd/initadmin ./app/admin/cmd/gormgen ./app/worker/cmd/worker

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
