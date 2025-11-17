#!/bin/bash

# CloudAI Frontend Setup Script

echo "🚀 Setting up CloudAI Frontend..."
echo ""

# Navigate to ui directory
cd "$(dirname "$0")"

# Check if node_modules exists
if [ -d "node_modules" ]; then
    echo "✓ Dependencies already installed"
else
    echo "📦 Installing dependencies..."
    npm install --legacy-peer-deps
    
    if [ $? -eq 0 ]; then
        echo "✓ Dependencies installed successfully"
    else
        echo "❌ Failed to install dependencies"
        exit 1
    fi
fi

echo ""
echo "✅ Setup complete!"
echo ""
echo "To start the development server:"
echo "  npm run dev"
echo ""
echo "The app will be available at: http://localhost:3000"
echo "Make sure the CloudAI backend is running on: http://localhost:8080"
