#!/bin/bash

ps auxw | grep "mmorms --addr=:8000" | grep -v grep > /dev/null

if [ $? != 0 ]
then
	cd ~/mmorms_go/src/mmorms
        ../../bin/mmorms --addr=:8000 > ~/mmorms8000.log 2>&1
        
	echo "Respawning mmorms:8000 process..." | /usr/sbin/sendmail lennytmp@gmail.com borodin.vadim@gmail.com
fi

