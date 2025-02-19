#!/bin/bash

set -e

# Variables
APP_NAME="hookrelay"
APP_DIR="/etc/$APP_NAME"
BINARY_PATH="/usr/local/bin/$APP_NAME"
SERVICE_FILE="/etc/systemd/system/$APP_NAME.service"
SYSLOG_FILE="/etc/rsyslog.d/50-$APP_NAME.conf"
LOG_FILE="/var/log/$APP_NAME.log"

# Create application directory
mkdir -p $APP_DIR

# Compile app
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o $APP_NAME cmd/cmd.go
mv ./$APP_NAME $BINARY_PATH
chmod +x $BINARY_PATH

# Create systemd service
cat >$SERVICE_FILE <<EOF
[Unit]
Description=$APP_NAME
After=network.target

[Service]
Environment='CFGFILE=$APP_DIR/config.toml'
ExecStartPre=/bin/rm -rf /tmp/$APP_NAME
ExecStart=$BINARY_PATH server -c \${CFGFILE} -l
Restart=always
StandardOutput=journal+console
StandardError=journal+console
SyslogIdentifier=$APP_NAME
SyslogFacility=local3
LimitNOFILE=100000

[Install]
WantedBy=multi-user.target
EOF

# Create rsyslog entry
echo "local3.*        $LOG_FILE" >$SYSLOG_FILE
touch $SYSLOG_FILE
chown syslog:adm $SYSLOG_FILE
chmod 644 $SYSLOG_FILE

# Reload systemd and start service
sudo systemctl daemon-reload
systemctl restart rsyslog
systemctl start $APP_NAME
systemctl enable $APP_NAME
