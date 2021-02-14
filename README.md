ISUCON7 予選問題

# ローカル
```
docker-compose -f docker-compose-local.yml build
docker-compose up -d db
docker-compose up -d app
docker-compose up -d web
docker-compose up -d bench # ベンチマークを動かす
```
あとは `webapp` 以下をチューニングする

# サーバに対してベンチマークを投げる
git, dockerの設定は出来てると仮定する
サーバに入ってbench以外のサービスを立ち上げる
```
docker-compose build
docker-compose up
```

`docker-compose-bench.yml` の `command` のIPを上のサーバのものに書き換えて実行する

