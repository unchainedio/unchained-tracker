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
REQUIRED_GO_VERSION=$(grep "^go " go.mod | cut -d' ' -f2)
echo "Required Go version from go.mod: $REQUIRED_GO_VERSION"

# Install GVM if not already installed
if ! command -v gvm &> /dev/null; then
    echo "Installing GVM..."
    sudo apt-get install -y curl git mercurial make binutils bison gcc build-essential
    bash < <(curl -s -S -L https://raw.githubusercontent.com/moovweb/gvm/master/binscripts/gvm-installer)
    source ~/.gvm/scripts/gvm
    check_status "Installing GVM"
fi

# Source GVM in current shell
source ~/.gvm/scripts/gvm

# Install Go version if not already installed via GVM
if ! gvm list | grep -q "go$REQUIRED_GO_VERSION"; then
    echo "Installing Go $REQUIRED_GO_VERSION using GVM..."
    # Install Go 1.4 if needed (required to build newer versions)
    if ! gvm list | grep -q "go1.4"; then
        echo "Installing Go 1.4 (required for building newer versions)..."
        gvm install go1.4 -B
        gvm use go1.4
    fi
    # Install required version
    gvm install "go$REQUIRED_GO_VERSION"
    check_status "Installing Go $REQUIRED_GO_VERSION"
fi

# Use the required version
echo "Setting Go version to $REQUIRED_GO_VERSION..."
gvm use "go$REQUIRED_GO_VERSION"
check_status "Setting Go version"

# Verify correct version is being used
# Extract version from "go version go1.24.0 linux/amd64"
CURRENT_GO_VERSION=$(go version | cut -d' ' -f3 | sed 's/^go//')
if [ "$CURRENT_GO_VERSION" != "$REQUIRED_GO_VERSION" ]; then
    echo "❌ Error: Wrong Go version in use"
    echo "Expected: $REQUIRED_GO_VERSION"
    echo "Got: $CURRENT_GO_VERSION"
    echo "Version strings: '$CURRENT_GO_VERSION' vs '$REQUIRED_GO_VERSION'"
    exit 1
else
    echo "✅ Using Go $REQUIRED_GO_VERSION"
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
else
    echo "✅ Database $DB_NAME already exists"
fi

# Configure MySQL password policy
echo "Configuring MySQL password policy..."
mysql -u root -p"$MYSQL_ROOT_PASSWORD" -e "
    SET GLOBAL validate_password.policy=LOW;
    SET GLOBAL validate_password.length=6;
    SET GLOBAL validate_password.mixed_case_count=0;
    SET GLOBAL validate_password.number_count=0;
    SET GLOBAL validate_password.special_char_count=0;
"
check_status "Setting password policy"

# Always recreate the user and permissions to ensure they're correct
echo "Setting up database user..."
mysql -u root -p"$MYSQL_ROOT_PASSWORD" -e "
    DROP USER IF EXISTS '$DB_USER'@'$DB_HOST';
    CREATE USER '$DB_USER'@'$DB_HOST' IDENTIFIED BY '$DB_PASSWORD';
    GRANT ALL PRIVILEGES ON $DB_NAME.* TO '$DB_USER'@'$DB_HOST';
    FLUSH PRIVILEGES;
"
check_status "Setting up database user and permissions"

# Verify user creation
echo "Verifying user creation..."
if mysql -u root -p"$MYSQL_ROOT_PASSWORD" -e "SELECT User FROM mysql.user WHERE User='$DB_USER'" | grep -q $DB_USER; then
    echo "✅ Database user verified"
else
    echo "❌ Error: Database user not created properly"
    exit 1
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
    ProxyPass / http://127.0.0.1:8088/
    ProxyPassReverse / http://127.0.0.1:8088/
    
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