# CloudAI

In the `go-master` directory run:

```bash
go mod tidy
```

In the `python-planner` directory run:

```bash
python -m venv venv
pip install -r requirements.txt
```

Turn on couchDB:

```bash
docker-compose up -d couchdb
```

Access couchdb on : `http://localhost:5984/_utils`