# ticketbench-contracts

[![CI](https://github.com/ryczek02/ticketbench-contracts/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/ryczek02/ticketbench-contracts/actions/workflows/ci.yml) [![Go Reference](https://pkg.go.dev/badge/github.com/ryczek02/ticketbench-contracts.svg)](https://pkg.go.dev/github.com/ryczek02/ticketbench-contracts) [![Release](https://img.shields.io/github/v/tag/ryczek02/ticketbench-contracts?sort=semver&label=tag)](https://github.com/ryczek02/ticketbench-contracts/tags) [![Go](https://img.shields.io/github/go-mod/go-version/ryczek02/ticketbench-contracts)](go.mod) [![License: MIT](https://img.shields.io/github/license/ryczek02/ticketbench-contracts)](LICENSE)

Shared Go module for the ticketbench project: gRPC contracts, the domain model,
the deterministic data generator, scenario definitions and the OpenTelemetry
setup used by every service.

ticketbench compares PostgreSQL, MariaDB, MongoDB and Valkey on a ticket
reservation workload. The project overview lives in
[ryczek02/ticketbench](https://github.com/ryczek02/ticketbench).

## Packages

| Path | Contents |
|---|---|
| `proto/ticketbench/v1` | `StoreService` (one per database) and `RunnerService` (orchestration) |
| `gen/ticketbench/v1` | Generated Go code, committed so consumers need no protoc |
| `pkg/model` | Domain types: events, venues, tickets, reservations, payments |
| `pkg/datagen` | Dataset generator. Same seed and size give the same rows on every store |
| `pkg/scenario` | The 24 CRUD scenarios plus 3 JSON scenarios and their parameters |
| `pkg/telemetry` | Tracer, meter and slog setup, W3C trace context over gRPC |
| `pkg/version` | Build metadata injected with `-ldflags` |

## Dataset

Size is the number of tickets. A ticket is one seat offered for one event, so
it can be free, held, reserved or sold. With seed 42:

| Tickets | Events | Venues | Seats | Customers | Reservations |
|---:|---:|---:|---:|---:|---:|
| 500 000 | 244 | 8 | 16 684 | 62 500 | 82 807 |
| 1 000 000 | 476 | 16 | 33 922 | 125 000 | 170 396 |
| 10 000 000 | 4 156 | 166 | 396 795 | 1 250 000 | 1 674 389 |

Every entity is derived from `(seed, kind, id)`. Stores generate the data
themselves instead of receiving it over the network, and `Checksum()` lets
the runner confirm that all four databases got identical rows.

Four sandbox events have no sales. Booking scenarios and the contention test
use them, so they never compete with each other or with the loaded data.
Fixture data created by scenarios lives in a separate id range starting at
10^12.

## Tracing

`telemetry.Setup` reads the standard variables:

```
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4317
OTEL_SDK_DISABLED=true     # keep trace ids, export nothing
TB_TRACE_RATIO=0.1         # sampling of root spans
```

`GRPCServerOptions` and `GRPCDialOptions` attach the otelgrpc stats handlers,
so a `traceparent` sent by the CLI becomes the parent of the runner span, the
store span and the database driver spans. `telemetry.Quiet(ctx)` keeps the
trace id but stops child spans from being recorded. Bulk loads use it to avoid
one span per insert batch.

## Development

```
make gen     # buf generate (needs buf, protoc-gen-go, protoc-gen-go-grpc)
make test    # go test -race ./...
make bench   # generator and scenario micro-benchmarks
```

CI runs buf lint, a breaking change check against `main` on pull requests,
a check that `gen/` matches the protos, golangci-lint, tests with the race
detector and the benchmarks.

## Authors

Łukasz Ryczko, Łukasz Pacholec. Politechnika Krakowska, course
"Zaawansowane technologie baz danych", 2026.
