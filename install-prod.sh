#!/bin/bash

# Load environment variables
if [ ! -f .env ]; then
    echo "Error: .env file not found"
    exit 1
fi
source .env

# Check if domain argument is provided
if [ -z "$1" ]; then
    echo "Usage: $0 <domain>"
    echo "Example: $0 tracker.example.com"
    exit 1
fi

DOMAIN=$1
echo "Installing Unchained Tracker on domain: $DOMAIN"

# Install required packages
echo "Installing dependencies..."
sudo apt-get update
# Only install MySQL if not already installed
if ! command -v mysql &> /dev/null; then
    sudo apt-get install -y mysql-server
fi
sudo apt-get install -y golang-go apache2

# Check if database exists
echo "Setting up MySQL..."
if ! mysql -u root -e "USE unchained_tracker" 2>/dev/null; then
    echo "Creating database unchained_tracker..."
    sudo mysql -e "CREATE DATABASE IF NOT EXISTS unchained_tracker;"
    
    # Create user only if it doesn't exist
    if ! mysql -u root -e "SELECT User FROM mysql.user WHERE User='tracker'" 2>/dev/null | grep -q tracker; then
        echo "Creating database user..."
        sudo mysql -e "CREATE USER '$DB_USER'@'$DB_HOST' IDENTIFIED BY '$DB_PASSWORD';"
        sudo mysql -e "GRANT ALL PRIVILEGES ON $DB_NAME.* TO '$DB_USER'@'$DB_HOST';"
        sudo mysql -e "FLUSH PRIVILEGES;"
    fi
else
    echo "Database unchained_tracker already exists, skipping database creation"
fi

# Build the application
echo "Building application..."
go build -o tracker cmd/tracker/main.go

# Create systemd service
echo "Creating systemd service..."
# Update database URL to use tracker user instead of root
sudo tee /etc/systemd/system/unchained-tracker.service << EOF
[Unit]
Description=Unchained Tracker
After=network.target mysql.service

[Service]
Type=simple
User=$USER
WorkingDirectory=$(pwd)
Environment="DATABASE_URL=$DB_USER:$DB_PASSWORD@tcp($DB_HOST:$DB_PORT)/$DB_NAME"
Environment="SERVER_ADDR=$SERVER_ADDR"
ExecStart=$(pwd)/tracker
Restart=always

[Install]
WantedBy=multi-user.target
EOF

# Enable Apache modules
echo "Configuring Apache..."
sudo a2enmod proxy
sudo a2enmod proxy_http
sudo a2enmod headers

# Create Apache virtual host
sudo tee /etc/apache2/sites-available/$DOMAIN.conf << EOF
<VirtualHost *:80>
    ServerName $DOMAIN
    
    ProxyPreserveHost On
    ProxyPass / http://127.0.0.1:8080/
    ProxyPassReverse / http://127.0.0.1:8080/
    
    RequestHeader set X-Forwarded-Proto "http"
    RequestHeader set X-Real-IP %{REMOTE_ADDR}s
    
    ErrorLog \${APACHE_LOG_DIR}/$DOMAIN-error.log
    CustomLog \${APACHE_LOG_DIR}/$DOMAIN-access.log combined
</VirtualHost>
EOF

# Enable site
sudo a2ensite $DOMAIN
sudo apache2ctl configtest && sudo systemctl reload apache2

# Reload systemd and start service
sudo systemctl daemon-reload
sudo systemctl enable unchained-tracker
sudo systemctl start unchained-tracker

echo "Installation complete!"
echo "Tracker is now available at: http://$DOMAIN"
echo "Check status with: sudo systemctl status unchained-tracker" 