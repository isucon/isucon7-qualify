Docker環境で利用するためのツールです。
mysqlのimageテーブルから画像を引き抜きファイルとして保存します。

``` shell
cd ${Repository path}/tools/image_output_tool

go run main.go
```

上記コマンドで直下のimagesにファイルが出力される。