# Database migrations

Migrations are applied by the repository's executable runner; no external
migration service is required. Configure the database with the same variables
as the API, then run from the repository root:

```bash
go run ./cmd/migrate -command up
go run ./cmd/migrate -command status
go run ./cmd/migrate -command down -steps 1
```

`up` applies every pending migration. `down` reverts the latest applied
migration (or the number supplied with `-steps`). Each migration runs in its
own PostgreSQL transaction and is recorded in `schema_migrations`. The
`-dir` flag can be used when migrations are stored elsewhere.
