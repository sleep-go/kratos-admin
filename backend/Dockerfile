FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY api ./api
COPY backend ./backend
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./backend/cmd/api
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/worker ./backend/cmd/worker

FROM golang:1.26-alpine AS tools
RUN go install github.com/pressly/goose/v3/cmd/goose@v3.26.0
WORKDIR /app
COPY backend/migrations ./backend/migrations

FROM alpine:3.22 AS api
RUN addgroup -S kratos && adduser -S kratos -G kratos
COPY --from=build /out/api /usr/local/bin/kratos-admin-api
USER kratos
EXPOSE 8000 9000
ENTRYPOINT ["kratos-admin-api"]

FROM alpine:3.22 AS worker
RUN addgroup -S kratos && adduser -S kratos -G kratos
COPY --from=build /out/worker /usr/local/bin/kratos-admin-worker
USER kratos
ENTRYPOINT ["kratos-admin-worker"]
