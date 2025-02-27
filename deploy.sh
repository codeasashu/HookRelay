#!/bin/bash

set -e

# Variables
APP_NAME="hookrelay"
BINARY_PATH="/usr/local/bin/$APP_NAME"
CFG_FILE="/etc/hookrelay/config.toml"
LOG_DIR="/var/log/$APP_NAME"

# Compile app
exec ./build.sh
mv ./bin/$APP_NAME $BINARY_PATH
chmod +x $BINARY_PATH

# Create systemd service
cat >/etc/supervisor/conf.d/$APP_NAME.conf <<EOF
[group:$APP_NAME]
programs=server,worker

[program:server]
command=$BINARY_PATH server -c $CFG_FILE
startsecs=3
autostart=true
autorestart=true
stopsignal=INT
killasgroup=true
environment=LANG=en_US.UTF-8,LC_ALL=en_US.UTF-8
user=root
stdout_logfile=$LOG_DIR/server.log
stderr_logfile=$LOG_DIR/server_err.log


[program:worker]
process_name=%(program_name)s_%(process_num)02d
command=$BINARY_PATH worker -c $CFG_FILE
startsecs=3
autostart=true
autorestart=true
stopsignal=INT
killasgroup=true
environment=LANG=en_US.UTF-8,LC_ALL=en_US.UTF-8
user=root
numprocs=1
stdout_logfile=$LOG_DIR/%(program_name)s_%(process_num)02d.log
stderr_logfile=$LOG_DIR/%(program_name)s_%(process_num)02d_err.log
EOF

# Reload supervisor
supervisorctl reread
supervisorctl update $APP_NAME
