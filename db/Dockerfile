FROM golang:1.9 AS build-dev

ARG GIT_URL=https://github.com/isucon/isucon7-qualify.git

RUN \
    git clone --depth=1 $GIT_URL /home/isucon/isubata && \
    cd /home/isucon/isubata/bench && \
    go get github.com/constabulary/gb/... && \
    gb vendor restore && \
    make && \
    bin/gen-initial-dataset

FROM mysql:5.7

ENV MYSQL_ALLOW_EMPTY_PASSWORD=yes MYSQL_DATABASE=isubata MYSQL_USER=isucon MYSQL_PASSWORD=isucon

COPY --from=build-dev /home/isucon/isubata/db/isubata.sql /docker-entrypoint-initdb.d/01_isubata.sql
COPY --from=build-dev /home/isucon/isubata/bench/isucon7q-initial-dataset.sql.gz /docker-entrypoint-initdb.d/isucon7q-initial-dataset.sql.gz 

CMD ["--character-set-server=utf8mb4"]
