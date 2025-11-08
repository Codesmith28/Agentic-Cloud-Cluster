#!/usr/bin/env bash
# setup.sh - bootstrap the project for local development
# usage: ./scripts/setup.sh [--yes] [--mongo-uri URI] [--jwt-secret SECRET]

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND_DIR="$ROOT_DIR/backend"
FRONTEND_DIR="$ROOT_DIR/frontend"

YES=false
MONGO_URI=""
JWT_SECRET=""
START=false

while [[ $# -gt 0 ]]; do
  case $1 in
    --yes|-y) YES=true; shift ;;
    --mongo-uri) MONGO_URI="$2"; shift 2 ;;
    --jwt-secret) JWT_SECRET="$2"; shift 2 ;;
    --start) START=true; shift ;;
    *) echo "Unknown arg: $1"; exit 1 ;;
  esac
done

echo "Bootstrapping project in $ROOT_DIR"

if ! command -v node >/dev/null 2>&1; then
  echo "Node is not installed. Install Node.js (>=16) and npm first." >&2
  exit 1
fi

if ! command -v npm >/dev/null 2>&1; then
  echo "npm is not installed. Install npm first." >&2
  exit 1
fi

echo "Installing backend dependencies..."
cd "$BACKEND_DIR"
npm install

# Set up backend .env
if [ -f "$BACKEND_DIR/.env" ]; then
  echo "backend/.env already exists; skipping creation"
else
  if [ -f "$BACKEND_DIR/.env.example" ]; then
    echo "Preparing backend/.env from .env.example"
    cp "$BACKEND_DIR/.env.example" "$BACKEND_DIR/.env"
    if [ -n "$MONGO_URI" ]; then
      sed -i"" -e "s~^MONGO_URI=.*~MONGO_URI=${MONGO_URI}~" "$BACKEND_DIR/.env" 2>/dev/null || sed -i -e "s~^MONGO_URI=.*~MONGO_URI=${MONGO_URI}~" "$BACKEND_DIR/.env"
    fi
    if [ -n "$JWT_SECRET" ]; then
      sed -i"" -e "s~^JWT_SECRET=.*~JWT_SECRET=${JWT_SECRET}~" "$BACKEND_DIR/.env" 2>/dev/null || sed -i -e "s~^JWT_SECRET=.*~JWT_SECRET=${JWT_SECRET}~" "$BACKEND_DIR/.env"
    else
      # generate a random secret if not provided
      if command -v openssl >/dev/null 2>&1; then
        RAND=$(openssl rand -hex 16)
      else
        RAND=$(date +%s | sha256sum | head -c 32)
      fi
      sed -i"" -e "s~^JWT_SECRET=.*~JWT_SECRET=${RAND}~" "$BACKEND_DIR/.env" 2>/dev/null || sed -i -e "s~^JWT_SECRET=.*~JWT_SECRET=${RAND}~" "$BACKEND_DIR/.env"
    fi
    echo "Created backend/.env (please review and edit values as needed)"
  else
    echo "No .env.example found in backend; creating minimal .env"
    cat > "$BACKEND_DIR/.env" <<EOF
PORT=5000
MONGO_URI=${MONGO_URI:-mongodb://localhost:27017/auth-demo}
JWT_SECRET=${JWT_SECRET:-$(date +%s | sha256sum | head -c 32)}
EOF
    echo "Created backend/.env"
  fi
fi

echo "Installing frontend dependencies..."
cd "$FRONTEND_DIR"
npm install

LOG_DIR="$ROOT_DIR/scripts/logs"
mkdir -p "$LOG_DIR"

echo "Setup complete. Next steps:"
echo "  1) Review and edit backend/.env if needed"
echo "  2) Start backend: cd backend && npm run dev"
echo "  3) Start frontend: cd frontend && npm run dev"

if [ "$START" = true ]; then
  echo "Starting backend and frontend in background; logs -> $LOG_DIR"
  # start backend
  cd "$BACKEND_DIR"
  nohup npm run dev > "$LOG_DIR/backend.log" 2>&1 &
  echo $! > "$LOG_DIR/backend.pid"
  echo "Backend started (pid $(cat $LOG_DIR/backend.pid)), log: $LOG_DIR/backend.log"

  # start frontend
  cd "$FRONTEND_DIR"
  nohup npm run dev -- --port 5173 > "$LOG_DIR/frontend.log" 2>&1 &
  echo $! > "$LOG_DIR/frontend.pid"
  echo "Frontend started (pid $(cat $LOG_DIR/frontend.pid)), log: $LOG_DIR/frontend.log"
else
  # interactive prompt to optionally start servers now
  if [ "$YES" = true ]; then
    RESP=y
  else
    read -p "Would you like to start backend and frontend now in background? [y/N] " RESP
  fi
  case "$RESP" in
    y|Y|yes|Yes)
      echo "Starting backend and frontend in background; logs -> $LOG_DIR"
      cd "$BACKEND_DIR"
      nohup npm run dev > "$LOG_DIR/backend.log" 2>&1 &
      echo $! > "$LOG_DIR/backend.pid"
      echo "Backend started (pid $(cat $LOG_DIR/backend.pid)), log: $LOG_DIR/backend.log"

      cd "$FRONTEND_DIR"
      nohup npm run dev -- --port 5173 > "$LOG_DIR/frontend.log" 2>&1 &
      echo $! > "$LOG_DIR/frontend.pid"
      echo "Frontend started (pid $(cat $LOG_DIR/frontend.pid)), log: $LOG_DIR/frontend.log"
      ;;
    *)
      echo "Skipping automatic start. Run the commands from the next steps when ready."
      ;;
  esac
fi

exit 0
