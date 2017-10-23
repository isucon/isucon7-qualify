# Go言語参照実装について

## GOPATH

このディレクトリを GOPATH にして、 src/isucon にアプリのソースを置いています。

make を実行すれば GOPATH を設定しつつ go build するようになっています。


## 依存ライブラリ

依存ライブラリは dep を使って管理していて、 src/isucon/vendor に含まれています。

ライブラリのバージョンを上げたい場合は、 ``GOPATH=`pwd` go get -u ...`` をしても
vendor 側が優先されるので、 vendor を消すか dep を使ってバージョンを上げてください。

ライブラリを追加する場合は問題ありません。
