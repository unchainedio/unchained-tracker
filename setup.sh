#!/bin/bash

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

print_status() {
    echo -e "${GREEN}[*] $1${NC}"
}

print_error() {
    echo -e "${RED}[!] $1${NC}"
}

# Check if MySQL is running
if ! pgrep mysqld > /dev/null; then
    print_error "MySQL is not running. Please start MySQL first."
    exit 1
fi

# Create database
print_status "Creating database..."
mysql -u root -proot << EOF
CREATE DATABASE IF NOT EXISTS unchained_tracker;
EOF

if [ $? -eq 0 ]; then
    print_status "Database setup complete!"
else
    print_error "Failed to create database. Please check MySQL credentials."
    exit 1
fi

# Build the application
print_status "Building application..."
go build -o bin/tracker cmd/tracker/main.go

print_status "Setup complete! You can now run:"
echo "go run cmd/tracker/main.go" 