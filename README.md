## CloudAI Overview

CloudAI is a distributed experimentation platform composed of a masternode that assigns work, a fleet of workers, and a MongoDB-backed metadata store. The repository also contains protobuf contracts (`proto/`) and documentation for the database schema (`docs/`).

```txt
.
├── .env
├── .git
├── .gitignore
├── database
├── docs
├── master
├── proto
└── README.md
```

## Prerequisites

- Go 1.22 or newer
- Docker Engine 20.10+ with Docker Compose v2
- `mongosh` (optional, for inspecting the local database)

## Set Up the MongoDB Instance

1. Change into the database directory: `cd database`
2. Start MongoDB: `docker compose up -d`
3. Verify the container: `docker compose ps`
4. Tail logs (optional): `docker compose logs -f mongodb`

## Bootstrap the Masternode

1. Change into the masternode directory: `cd master`
2. Download Go dependencies: `go mod tidy`
3. Run the initialization entry point to seed collections: `go run .`

> The masternode reads root credentials from `.env` (`MONGODB_USERNAME`, `MONGODB_PASSWORD`). Ensure the file is populated before starting services.

## Next Steps

- Implement the remaining masternode services so they call `internal/db.EnsureCollections` at startup.
- Build worker nodes that consume tasks issued by the masternode.
- Extend the Mongo schema in `docs/schema.md` as the project evolves.
