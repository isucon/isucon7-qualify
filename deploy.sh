#!/bin/bash
set -ex
IPADDR=$1
BRANCH=`git symbolic-ref --short HEAD`
USERNAME=$USER

echo $BRANCH

ssh isucon@$IPADDR "source ~/.profile && source ~/.bashrc && cd /home/isucon/isubata && git pull && cd webapp/go && make && sudo systemctl restart mysql && sudo service nginx restart && sudo sudo systemctl enable isubata.golang.service && sudo sysctl -p"

