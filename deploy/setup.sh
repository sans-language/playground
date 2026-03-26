#!/bin/bash
set -euo pipefail

echo "=== Sans Playground Server Setup ==="

# Create service user
sudo useradd -r -s /bin/false sans 2>/dev/null || true

# Create data directory
sudo mkdir -p /var/lib/sans-playground
sudo chown sans:sans /var/lib/sans-playground

# Build the sandbox Docker image
echo "Building Docker sandbox image..."
curl -fsSL "https://github.com/sans-language/sans/releases/latest/download/sans-linux-x86_64.tar.gz" | tar xz
cp sans sandbox/sans
# Clone runtime .sans source files (compiler needs them to build user programs)
git clone --depth 1 https://github.com/sans-language/sans /tmp/sans-src 2>/dev/null || true
cp -r /tmp/sans-src/runtime sandbox/runtime
rm -rf /tmp/sans-src
sudo docker build -t sans-playground sandbox/
rm -rf sandbox/sans sandbox/runtime sans

# Build the Go server
echo "Building playground server..."
CGO_ENABLED=1 go build -o playground-server .
sudo mv playground-server /usr/local/bin/sans-playground

# Install configs
sudo cp deploy/sans-playground.service /etc/systemd/system/
sudo cp deploy/nginx.conf /etc/nginx/sites-available/sans-playground
sudo ln -sf /etc/nginx/sites-available/sans-playground /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx

# Start service
sudo systemctl daemon-reload
sudo systemctl enable sans-playground
sudo systemctl start sans-playground

echo "=== Setup complete ==="
echo "Run: sudo certbot --nginx -d api.sans.dev"
echo "Then: curl https://api.sans.dev/api/health"
