#!/bin/bash

# Dependency installation script for Agentic-Cloud-Cluster
# Optimized for macOS (Homebrew) and Linux (apt)

set -e

echo "🚀 Starting dependency installation..."

# 1. Check for Go
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed. Please install Go first: https://golang.org/doc/install"
    exit 1
fi
echo "✅ Go is installed: $(go version)"

# 2. Install protoc (Protobuf Compiler)
if ! command -v protoc &> /dev/null; then
    echo "📦 Installing protoc..."
    if [[ "$OSTYPE" == "darwin"* ]]; then
        if command -v brew &> /dev/null; then
            brew install protobuf
        else
            echo "❌ Homebrew not found. Please install Homebrew or manually install protoc."
            exit 1
        fi
    elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
        if command -v apt-get &> /dev/null; then
            sudo apt-get update && sudo apt-get install -y protobuf-compiler
        else
            echo "❌ apt-get not found. Please manually install protobuf-compiler."
            exit 1
        fi
    else
        echo "❌ Unsupported OS for automatic protoc installation. Please install it manually."
        exit 1
    fi
else
    echo "✅ protoc is already installed: $(protoc --version)"
fi

# 3. Create Python virtual environment
echo "🐍 Setting up Python virtual environment (venv)..."
python3 -m venv venv
if [ -d "venv/bin" ]; then
    source venv/bin/activate
elif [ -d "venv/Scripts" ]; then
    source venv/Scripts/activate
fi
echo "✅ Virtual environment created and activated."

# 4. Install Go gRPC plugins
echo "📦 Installing Go gRPC plugins..."
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Add GOPATH/bin to PATH for the current session
export PATH=$PATH:$(go env GOPATH)/bin
echo "✅ Go gRPC plugins installed/updated."

# 5. Install Python dependencies in venv
echo "📦 Installing Python dependencies from requirements.txt into venv..."
pip install --upgrade pip
pip install -r requirements.txt

echo "✨ All dependencies installed successfully!"
echo "💡 Note: To use the Python environment, run: source venv/bin/activate"
echo "💡 Note: Make sure $(go env GOPATH)/bin is in your PATH."
