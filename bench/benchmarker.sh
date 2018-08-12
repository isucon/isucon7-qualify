#!/bin/bash
BENCH_DIR=/home/isucon/isubata/bench
LOG_DIR=/var/log
SLOW_LOG_FILE=${LOG_DIR}/mysql/mysql-slow.log
NGINX_FILE=${LOG_DIR}/nginx/access.log
BENCH_FILE=bench.json
GO_DIR=/home/isucon/isubata/webapp/go

#build go
cd ${GO_DIR}
make

cd ${BENCH_DIR}
sudo rm bench.json
sudo rm -f ${SLOW_LOG_FILE}
sudo rm -f ${NGINX_FILE}
sudo systemctl restart mysql.service
sudo systemctl restart nginx.service
sudo systemctl restart isubata.golang.service

./bin/bench -remotes=127.0.0.1 -output $BENCH_FILE
cat bench.json | jq .score
sudo mysqldumpslow -s t $SLOW_LOG_FILE > slowlog_dump.txt
sudo alp -f ${NGINX_FILE} > alp_access.log
