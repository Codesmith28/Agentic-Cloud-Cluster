#!/bin/bash

# CloudAI System Launcher
# This script helps start the master and worker nodes

set -e

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${BLUE}"
echo "╔═══════════════════════════════════════════════════╗"
echo "║          CloudAI System Launcher                  ║"
echo "╚═══════════════════════════════════════════════════╝"
echo -e "${NC}"

# Check if MongoDB is running
echo -e "${YELLOW}Checking MongoDB...${NC}"
if ! docker ps | grep -q mongodb; then
    echo -e "${YELLOW}MongoDB not running. Starting...${NC}"
    cd database && docker-compose up -d && cd ..
    echo -e "${GREEN}✓ MongoDB started${NC}"
else
    echo -e "${GREEN}✓ MongoDB is already running${NC}"
fi

# Check if binaries exist
if [ ! -f "master/master-node" ]; then
    echo -e "${RED}✗ Master binary not found. Run: make master${NC}"
    exit 1
fi

if [ ! -f "worker/worker-node" ]; then
    echo -e "${RED}✗ Worker binary not found. Run: make worker${NC}"
    exit 1
fi

# Check for .env file
if [ ! -f ".env" ]; then
    echo -e "${YELLOW}Creating .env file...${NC}"
    cat > .env << EOF
MONGODB_USERNAME=admin
MONGODB_PASSWORD=password123
EOF
    echo -e "${GREEN}✓ .env file created${NC}"
fi

echo ""
echo -e "${BLUE}Choose an option:${NC}"
echo "  1) Start Master only"
echo "  2) Start Worker only"
echo "  3) Start Master + Worker (separate terminals required)"
echo "  4) Exit"
echo ""
read -p "Enter choice [1-4]: " choice

case $choice in
    1)
        echo -e "${GREEN}Starting Master...${NC}"
        cd master && ./master-node
        ;;
    2)
        echo -e "${GREEN}Starting Worker...${NC}"
        read -p "Enter worker ID [worker-1]: " worker_id
        worker_id=${worker_id:-worker-1}
        read -p "Enter master address [localhost:50051]: " master_addr
        master_addr=${master_addr:-localhost:50051}
        cd worker && ./worker-node -id "$worker_id" -master "$master_addr"
        ;;
    3)
        echo -e "${YELLOW}Opening Master in this terminal...${NC}"
        echo -e "${YELLOW}Open a new terminal and run: ./launch.sh${NC}"
        echo -e "${YELLOW}Then choose option 2 to start a worker${NC}"
        echo ""
        sleep 2
        cd master && ./master-node
        ;;
    4)
        echo "Exiting..."
        exit 0
        ;;
    *)
        echo -e "${RED}Invalid choice${NC}"
        exit 1
        ;;
esac
