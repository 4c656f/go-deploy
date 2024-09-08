#!/bin/bash

set -e

# Load environment variables
source .env
# Clone or update the repository
if [ -d "$REPO_DIR" ]; then
    cd "$REPO_DIR"
    git pull
else
    git clone "$REPO_URL" "$REPO_DIR"
    cd "$REPO_DIR"
fi

# Build and start the new services
docker-compose up -d --build

echo "Update completed successfully"
