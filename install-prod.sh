#!/bin/bash

# Function to check last command status
check_status() {
    if [ $? -eq 0 ]; then
        echo "✅ $1"
    else
        echo "❌ Error: $1 failed"
        exit 1
    fi
}

# Load environment variables
if [ ! -f .env ]; then
    echo "Error: .env file not found"
    exit 1
fi
source .env
check_status "Loading environment variables"
echo "Using database: $DB_NAME"
echo "Using user: $DB_USER"

# Check if DOMAIN is set in .env
if [ -z "$DOMAIN" ]; then
    echo "❌ Error: DOMAIN not set in .env file"
    echo "Please set DOMAIN=your-domain.com in .env"
    exit 1
fi

echo "Installing Unchained Tracker on domain: $DOMAIN"
echo "Using domain: $DOMAIN"

# Install required packages
echo "📦 Installing dependencies..."
sudo apt-get update
check_status "Updating package list"

# Check and install Go if needed
if ! command -v go &> /dev/null; then
    echo "Installing Go..."
    sudo apt-get install -y golang-go
    check_status "Installing Go"
else
    echo "✅ Go already installed ($(go version))"
fi

# Only install MySQL if not already installed
if ! command -v mysql &> /dev/null; then
    echo "Installing MySQL server..."
    sudo apt-get install -y mysql-server
    check_status "Installing MySQL"
else
    echo "✅ MySQL already installed"
fi

# Check and install Apache if needed
if ! command -v apache2 &> /dev/null; then
    echo "Installing Apache..."
    sudo apt-get install -y apache2
    check_status "Installing Apache"
else
    echo "✅ Apache already installed ($(apache2 -v | head -n1))"
fi

# Check if database exists
echo "🔧 Setting up MySQL..."
if ! mysql -u root -p"$MYSQL_ROOT_PASSWORD" -e "USE $DB_NAME" 2>/dev/null; then
    echo "Creating database $DB_NAME..."
    mysql -u root -p"$MYSQL_ROOT_PASSWORD" -e "CREATE DATABASE IF NOT EXISTS $DB_NAME;"
    check_status "Creating database"
    
    # Lower MySQL password policy requirements
    echo "Configuring MySQL password policy..."
    mysql -u root -p"$MYSQL_ROOT_PASSWORD" -e "
        SET GLOBAL validate_password.policy=LOW;
        SET GLOBAL validate_password.length=6;
    "
    check_status "Setting password policy"
    
    # Create user only if it doesn't exist
    if ! mysql -u root -p"$MYSQL_ROOT_PASSWORD" -e "SELECT User FROM mysql.user WHERE User='$DB_USER'" 2>/dev/null | grep -q $DB_USER; then
        echo "Creating database user $DB_USER..."
        mysql -u root -p"$MYSQL_ROOT_PASSWORD" -e "
            CREATE USER '$DB_USER'@'$DB_HOST' IDENTIFIED BY '$DB_PASSWORD';
            GRANT ALL PRIVILEGES ON $DB_NAME.* TO '$DB_USER'@'$DB_HOST';
            FLUSH PRIVILEGES;
        "
        check_status "Creating database user"
    fi
else
    echo "✅ Database $DB_NAME already exists"
fi

# Test database connection
echo "Testing database connection..."
echo "Attempting to connect with:"
echo "  User: $DB_USER"
echo "  Host: $DB_HOST:$DB_PORT"
echo "  Database: $DB_NAME"

# First test MySQL service status
if ! systemctl is-active --quiet mysql; then
    echo "❌ Error: MySQL service is not running"
    echo "Starting MySQL service..."
    sudo systemctl start mysql
    check_status "Starting MySQL service"
fi

# Test connection with verbose output
MYSQL_TEST=$(mysql -u "$DB_USER" -p"$DB_PASSWORD" -h "$DB_HOST" "$DB_NAME" -e "SELECT 1" 2>&1)
if mysql -u "$DB_USER" -p"$DB_PASSWORD" -h "$DB_HOST" "$DB_NAME" -e "SELECT 1" &>/dev/null; then
    echo "✅ Database connection successful"
else
    echo "❌ Error: Could not connect to database"
    echo "Error details: $MYSQL_TEST"
    echo ""
    echo "Debugging steps:"
    echo "1. Check if MySQL is running: systemctl status mysql"
    echo "2. Verify credentials in .env file"
    echo "3. Try connecting manually: mysql -u $DB_USER -p"
    echo "4. Check MySQL logs: sudo tail -f /var/log/mysql/error.log"
    exit 1
fi

# Build the application
echo "🔨 Building application..."
go build -o tracker cmd/tracker/main.go
check_status "Building application"

# Create systemd service
echo "⚙️ Creating systemd service..."
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
check_status "Creating systemd service file"

# Enable Apache modules
echo "🌐 Configuring Apache..."
sudo a2enmod proxy
sudo a2enmod proxy_http
sudo a2enmod headers
check_status "Enabling Apache modules"

# Create Apache virtual host
echo "Creating virtual host for $DOMAIN..."
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
check_status "Creating Apache virtual host"

# Enable site and test config
echo "Enabling site..."
sudo a2ensite $DOMAIN
check_status "Enabling site"

echo "Testing Apache configuration..."
sudo apache2ctl configtest
check_status "Apache configuration test"

echo "Reloading Apache..."
sudo systemctl reload apache2
check_status "Reloading Apache"

# Start tracker service
echo "🚀 Starting tracker service..."
sudo systemctl daemon-reload
check_status "Reloading systemd configuration"

sudo systemctl enable unchained-tracker
check_status "Enabling tracker service"

sudo systemctl start unchained-tracker
check_status "Starting tracker service"

# Verify service is running
echo "Verifying service status..."
if systemctl is-active --quiet unchained-tracker; then
    echo "✅ Tracker service is running"
else
    echo "❌ Error: Tracker service failed to start"
    echo "Check logs with: sudo journalctl -u unchained-tracker"
    exit 1
fi

echo "✨ Installation complete!"
echo "🌐 Tracker is now available at: http://$DOMAIN"
echo "📊 Check status with: sudo systemctl status unchained-tracker"
echo "📝 View logs with: sudo journalctl -u unchained-tracker" 