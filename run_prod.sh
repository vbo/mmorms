#!/bin/bash
# Production respawn script. Set ALERT_EMAIL for restart notifications.

ps auxw | grep "mmorms --addr=:8000" | grep -v grep > /dev/null

if [ $? != 0 ]
then
	cd "$(dirname "$0")"
	./mmorms --addr=:8000 >> mmorms8000.log 2>&1

	if [ -n "$ALERT_EMAIL" ]; then
		echo "Respawning mmorms:8000 process..." | /usr/sbin/sendmail "$ALERT_EMAIL"
	fi
fi

