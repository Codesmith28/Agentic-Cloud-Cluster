# Web UI Setup & Login Guide

This document provides essential information for setting up and accessing the Agentic Cloud Cluster Web UI.

## Quick Start

### 1. Start Master Node (with UI)
```bash
cd /Users/codesmith28/personal/Projects/acc/BTEP
./runMaster.sh --ppo
```

The script will:
- Start MongoDB (if not running)
- Launch the Web UI on **http://localhost:3001**
- Start the master gRPC server on **:50051**
- Start the master HTTP API on **:8080**

### 2. Default Login Credentials

The Web UI admin user is **automatically created on startup** using environment variables from `.env`:

| Field | Default Value | .env Variable |
|-------|---|---|
| **Email** | `admin@localhost` | `WEBUI_ADMIN_EMAIL` |
| **Password** | `ChangeMeAdmin123!` | `WEBUI_ADMIN_PASSWORD` |
| **Admin Name** | `Web UI Admin` | `WEBUI_ADMIN_NAME` |

### 3. Check Your Configuration

View your current `.env` settings:
```bash
grep WEBUI_ADMIN /Users/codesmith28/personal/Projects/acc/BTEP/.env
```

If not found, defaults are used. To customize, edit `.env`:
```env
WEBUI_ADMIN_EMAIL=your-email@example.com
WEBUI_ADMIN_PASSWORD=YourSecurePassword123!
WEBUI_ADMIN_NAME=Your Display Name
```

### 4. Navigate to Web UI

Once master is running and logs show:
```
✓ Bootstrapped default Web UI admin user: admin@localhost
✓ Frontend started on port 3001
Web UI URL: http://localhost:3001
```

Open **http://localhost:3001** and login with the credentials above.

---

## Troubleshooting

### Port 3001 Already in Use

**Error:**
```
Error: Port 3001 is already in use
```

**Solution:**
Kill the existing process:
```bash
lsof -ti:3001 | xargs kill -9
```

Then start master again:
```bash
./runMaster.sh --ppo
```

**Alternative:** Use a different port:
```bash
WEBUI_PORT=3002 ./runMaster.sh --ppo
```

### Login Not Working

1. **Verify admin user was created** — check master logs for:
   ```
   ✓ Bootstrapped default Web UI admin user: admin@localhost
   ```

2. **Check backend API health:**
   ```bash
   curl http://localhost:8080/health | jq
   ```

3. **Verify auth endpoints:**
   ```bash
   # Test registration
   curl -X POST http://localhost:8080/api/auth/register \
     -H 'Content-Type: application/json' \
     -d '{
       "name": "Test User",
       "email": "test@example.com",
       "password": "TestPassword123!"
     }'

   # Test login
   curl -X POST http://localhost:8080/api/auth/login \
     -H 'Content-Type: application/json' \
     -c cookies.txt \
     -d '{
       "email": "admin@localhost",
       "password": "ChangeMeAdmin123!"
     }'

   # Verify session
   curl -b cookies.txt http://localhost:8080/api/auth/me | jq
   ```

### Cookie/Session Issues

The auth cookie is automatically configured for localhost HTTP development:

- **Secure flag:** Auto-set based on request context (localhost HTTP → Secure=false, others → Secure=true)
- **Override:** Set `AUTH_COOKIE_SECURE` in `.env` if needed:
  ```env
  AUTH_COOKIE_SECURE=false  # Force non-secure (dev only)
  AUTH_COOKIE_SECURE=true   # Force secure
  ```

---

## Running Full Campaign with Live UI

### 1. Start Infrastructure
```bash
make testbench-host-up
```

### 2. Start Master (new terminal)
```bash
./runMaster.sh --ppo
```
Watch for: `✓ Frontend started on port 3001`

### 3. Register Workers (new terminal)
```bash
make testbench-host-register
```

### 4. Run Campaign (new terminal)
```bash
make campaign-full
```

### 5. Watch Live in Web UI

- **Web UI:** http://localhost:3001 (login with admin credentials)
- **Grafana:** http://localhost:3300 (see dashboard setup below)

---

## Grafana Dashboard Access

Grafana is available when using the testbench stack.

### Default Credentials

| Field | Default | .env Variable |
|-------|---------|---|
| **Username** | `admin` | `GF_ADMIN_USER` |
| **Password** | `password` | `GF_ADMIN_PASSWORD` |

### URL

**Host-master mode:** http://localhost:3300  
**Full Docker mode:** http://localhost:3000

### Login & View Dashboards

1. Navigate to Grafana URL above
2. Login with credentials above
3. Click **Dashboards** → select:
   - **Agentic Cloud Cluster Overview** — key metrics, queue depth, task success rate
   - **Agentic Cloud Cluster Scheduler & Queue** — scheduling latency, throughput
   - **Agentic Cloud Cluster Worker Runtime** — execution performance, resource usage

---

## Environment Variables Reference

### Web UI Admin Bootstrap

```env
# Admin user created at startup if missing
WEBUI_ADMIN_NAME=Web UI Admin
WEBUI_ADMIN_EMAIL=admin@localhost
WEBUI_ADMIN_PASSWORD=ChangeMeAdmin123!

# Set true to reset existing admin password on startup
WEBUI_ADMIN_RESET_PASSWORD=false
```

### Auth & Cookies

```env
# Optional override for auth cookie Secure flag
# Default: auto (true for HTTPS/proxies, false for localhost HTTP)
AUTH_COOKIE_SECURE=

# Stable JWT secret (generated if unset)
JWT_SECRET=CHANGE_ME_GENERATE_WITH_openssl_rand_base64_32
```

### Grafana (Testbench)

```env
# Admin credentials for Grafana
GF_ADMIN_USER=admin
GF_ADMIN_PASSWORD=password
```

### Web UI Server

```env
# Port for Web UI dev server (default 3001)
WEBUI_PORT=3001
```

---

## Common Commands

| Task | Command |
|------|---------|
| Start master + UI | `./runMaster.sh --ppo` |
| Use custom UI port | `WEBUI_PORT=3002 ./runMaster.sh` |
| Reset admin password | `WEBUI_ADMIN_RESET_PASSWORD=true ./runMaster.sh` |
| Start testbench stack | `make testbench-host-up` |
| Run campaign | `make campaign-full` |
| Stop testbench | `make testbench-host-down` |
| Kill stuck UI process | `lsof -ti:3001 \| xargs kill -9` |

---

## Support

For detailed troubleshooting or additional setup info:

- **API Reference:** See `master/README.md`
- **Testbench Guide:** See `testbench/README.md`
- **Project Overview:** See root `README.md`
- **Architecture:** See `docs/DOCUMENTATION.md`

