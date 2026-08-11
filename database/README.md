# CloudAI MongoDB Service

This directory hosts a Docker Compose setup that provisions a MongoDB instance for local CloudAI development and testing.

## Prerequisites

- Docker Engine 20.10+
- Docker Compose v2 (bundled with modern Docker Desktop or Docker Engine installations)

## Start the database

```bash
docker compose up -d
```

This launches the container named `cloudai-mongo` and creates a persistent Docker volume called `mongo_data` so data survives container restarts.

## Verify the container

```bash
docker compose ps
```

Expect the `mongodb` service to show a `running` state.

## Connect to MongoDB

- Connection string: `mongodb://cloudai:cloudai_secret@localhost:27017`
- MongoDB shell (requires `mongosh`):
  ```bash
  mongosh "mongodb://cloudai:cloudai_secret@localhost:27017"
  ```
  The shell authenticates with the `cloudai` root user configured in Compose.

## Logs

```bash
docker compose logs -f mongodb
```

Press `Ctrl+C` to exit the log tail without stopping the container.

## Stop the database

```bash
docker compose down
```

Add `--volumes` if you want to remove the `mongo_data` volume and all stored data.

## Troubleshooting tips

- If `docker compose up` fails, ensure no other process occupies port `27017` and that Docker is running.
- Delete the volume for a clean start: `docker volume rm database_mongo_data` (replace with the actual volume name shown by `docker volume ls`).
