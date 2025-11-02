#!/bin/bash
# Quick setup script for mc-server-agent-v2

set -e

echo "🚀 MC Server Agent v2 - Quick Setup"
echo "===================================="
echo ""

# Check if .env exists
if [ ! -f .env ]; then
    echo "📝 Creating .env from .env.example..."
    cp .env.example .env
    echo "✅ .env created"
    echo "⚠️  Please edit .env and add your Discord credentials"
else
    echo "✓ .env already exists"
fi

# Check if settings.json exists
if [ ! -f settings.json ]; then
    echo "📝 Creating settings.json from settings.example.json..."
    cp settings.example.json settings.json
    echo "✅ settings.json created"
    echo "⚠️  Please edit settings.json and configure your containers"
else
    echo "✓ settings.json already exists"
fi

# Check Docker socket permissions
echo ""
echo "🔍 Checking Docker socket permissions..."
if [ -w /var/run/docker.sock ]; then
    echo "✅ Docker socket is writable"
else
    echo "⚠️  Docker socket is not writable"
    echo "   Run: sudo usermod -aG docker $USER"
    echo "   Then log out and log back in"
fi

# Get current UID and Docker GID
echo ""
echo "📋 System Information:"
echo "   UID: $(id -u)"
echo "   Docker GID: $(getent group docker | cut -d: -f3)"

echo ""
echo "✨ Setup complete!"
echo ""
echo "Next steps:"
echo "  1. Edit .env with your Discord Bot credentials"
echo "  2. Edit settings.json with your container configuration"
echo "  3. Run: docker compose up --build"
echo ""
echo "For more information, see README.md"
