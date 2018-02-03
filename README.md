ISUCON7 予選問題
====

[予選マニュアル](https://gist.github.com/941/8c64842b71995a2d448315e2594f62c2)

## 感想戦用、1VMでの動かし方

### ディレクトリ構成

```sh
db      - データベーススキーマ等
bench   - ベンチマーカー、初期データ生成器
webapp  - 各種言語実装
files   - 各種設定ファイル
```

### 環境構築

Ubuntu 16.04 のものをなるべくデフォルトで使います。

まずは `isucon` ユーザーを作り、そのホームディレクトリ配下の `isubata` ディレクトリに
リポジトリをチェックアウトします。

```console
$ sudo apt install git
$ git clone https://github.com/isucon/isucon7-qualify.git isubata
```

nginx と MySQL は Ubuntu の標準のものを使います。

```
$ sudo apt install mysql-server nginx
```

各言語は xbuild で最新安定版をインストールします。まず xbuild が必要とするライブラリをインストールします。

```
$ sudo apt install -y git curl libreadline-dev pkg-config autoconf automake build-essential libmysqlclient-dev \
	libssl-dev python3 python3-dev python3-venv openjdk-8-jdk-headless libxml2-dev libcurl4-openssl-dev \
        libxslt1-dev re2c bison libbz2-dev libreadline-dev libssl-dev gettext libgettextpo-dev libicu-dev \
	libmhash-dev libmcrypt-dev libgd-dev libtidy-dev
```

xbuildで言語をインストールします。ベンチマーカーのために、Goは必ずインストールしてください。
他の言語は使わないのであればスキップしても問題ないと思います。

```
cd
git clone https://github.com/tagomoris/xbuild.git

mkdir local
xbuild/ruby-install   -f 2.4.2   /home/isucon/local/ruby
xbuild/perl-install   -f 5.26.1  /home/isucon/local/perl
xbuild/node-install   -f v6.11.4 /home/isucon/local/node
xbuild/go-install     -f 1.9     /home/isucon/local/go
xbuild/python-install -f 3.6.2   /home/isucon/local/python
xbuild/php-install    -f 7.1.9   /home/isucon/local/php -- --disable-phar --with-pcre-regex --with-zlib --enable-fpm --enable-pdo --with-mysqli=mysqlnd --with-pdo-mysql=mysqlnd --with-openssl --with-pcre-regex --with-pcre-dir --with-libxml-dir --enable-opcache --enable-bcmath --with-bz2 --enable-calendar --enable-cli --enable-shmop --enable-sysvsem --enable-sysvshm --enable-sysvmsg --enable-mbregex --enable-mbstring --with-mcrypt --enable-pcntl --enable-sockets --with-curl --enable-zip --with-pearAA
```

### ベンチマーカーの準備

Goを使うのでこれだけは最初に環境変数を設定しておく

```
export PATH=$HOME/local/go/bin:$HOME/go/bin:$PATH
```

ビルド

```sh
go get github.com/constabulary/gb/...   # 初回のみ
cd ~/isubata/bench
gb vendor restore
make
```

初期データ生成

```sh
cd ~/isubata/bench
./bin/gen-initial-dataset   #isucon7q-initial-dataset.sql.gz ができる
```

### データベース初期化

データベース初期化、アプリが動くのに最低限必要なデータ投入

```sh
$ sudo ./db/init.sh
$ sudo mysql
mysql> CREATE USER isucon@'%' IDENTIFIED BY 'isucon';
mysql> GRANT ALL on *.* TO isucon@'%';
mysql> CREATE USER isucon@'localhost' IDENTIFIED BY 'isucon';
mysql> GRANT ALL on *.* TO isucon@'localhost';
```

初期データ投入

```sh
zcat ~/isubata/bench/isucon7q-initial-dataset.sql.gz | sudo mysql isubata
```

デフォルトだとTCPが127.0.0.1しかbindしてないので、複数台構成に対応するには
`/etc/mysql/mysql.conf.d/mysqld.cnf` で `bind-address = 127.0.0.1` になっている
場所を `bind-address = 0.0.0.0` に書き換える。


### nginx

```sh
$ sudo cp ~/isubata/files/app/nginx.* /etc/nginx/sites-available
$ cd /etc/nginx/sites-enabled
$ sudo unlink default
$ sudo ln -s ../sites-available/nginx.conf  # php の場合は nginx.php.conf
$ sudo systemctl restart nginx
```


### 参考実装(python)を動かす

初回のみ

```console
$ cd ~/isubata/webapp/python
$ ./setup.sh
```

起動

```sh
export ISUBATA_DB_HOST=127.0.0.1
export ISUBATA_DB_USER=isucon
export ISUBATA_DB_PASSWORD=isucon
./venv/bin/gunicorn --workers=10 -b '127.0.0.1:5000' app:app
```

予選本番では、 `/etc/hosts` に各ホスト名を書いて、環境変数は systemd から `env.sh` ファイルを読み込んでいました。
この辺は適当に使いやすいように設定してください。


### ベンチマーク実行

```console
$ cd bench
$ ./bin/bench -h # ヘルプ確認
$ ./bin/bench -remotes=127.0.0.1 -output result.json
```

結果を見るには `sudo apt install jq` で jq をインストールしてから、

```
$ jq . < result.json
```

### 備考

systemd に置く設定ファイルなどは files/ ディレクトリから探してください。


### 使用データの取得元

- 青空文庫 http://www.aozora.gr.jp/
- なんちゃって個人情報 http://kazina.com/dummy/
- いらすとや http://www.irasutoya.com/
- pixabay https://pixabay.com/
