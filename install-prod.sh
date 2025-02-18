#!/bin/bash

# Check if domain argument is provided
if [ -z "$1" ]; then
    echo "Usage: $0 <domain>"
    echo "Example: $0 tracker.example.com"
    exit 1
fi

DOMAIN=$1

# Create service user and directory
sudo useradd -r -s /bin/false tracker
sudo mkdir -p /opt/tracker
sudo chown tracker:tracker /opt/tracker

# Copy files to installation directory
cp -r * /opt/tracker/
cd /opt/tracker

# Create systemd service file
sudo cat > /etc/systemd/system/tracker.service << EOF
[Unit]
Description=Unchained Tracker
After=network.target mysql.service

[Service]
Type=simple
User=tracker
Group=tracker
WorkingDirectory=/opt/tracker
ExecStart=/opt/tracker/bin/tracker
Restart=always
Environment=SERVER_ADDR=127.0.0.1:8080
Environment=DATABASE_URL=user:pass@tcp(localhost:3306)/tracker

[Install]
WantedBy=multi-user.target
EOF

# Enable Apache modules
sudo a2enmod proxy
sudo a2enmod proxy_http
sudo a2enmod headers

# Create Apache virtual host
sudo cat > /etc/apache2/sites-available/$DOMAIN.conf << EOF
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

# Start tracker service
sudo systemctl daemon-reload
sudo systemctl enable tracker
sudo systemctl start tracker

echo "Installation complete!"
echo "Tracker is now available at: http://$DOMAIN" 