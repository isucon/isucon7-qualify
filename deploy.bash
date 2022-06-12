#!/bin/bash
docker compose down

if [ $1 = "--no-cache" ]; then
  docker-compose build --no-cache
else
  docker-compose build
fi

docker compose up -d db
docker compose up -d app
docker compose up -d web

for i in {1..20}; do
  sleep 10
  docker compose logs db | grep 'done'
  if [ $? != 0 ] ; then
    echo "until db is not initialized"
    continue
  else 
    echo "db is initialized"
    break
  fi
  
done

