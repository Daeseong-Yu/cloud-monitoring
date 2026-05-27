FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN mkdir -p /out \
  && go build -o /out/collector ./cmd/collector \
  && go build -o /out/metricdefs-sync ./cmd/metricdefs-sync

FROM alpine:3.22

RUN apk add --no-cache ca-certificates postgresql-client

WORKDIR /app

COPY --from=build /out/collector /app/collector
COPY --from=build /out/metricdefs-sync /app/metricdefs-sync
COPY configs /app/configs
COPY db /app/db
COPY scripts/apply-schema.sh scripts/run-retention.sh /app/scripts/

CMD ["/app/collector"]
