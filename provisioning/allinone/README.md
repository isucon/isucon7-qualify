# allinone (ALL in One) 用 Playbook

ベンチマークとアプリをひとつのサーバにセットアップする Playbook です（予選の際とは構成が異なります）。

## 目次

  * [この Playbook の使い方](#この-playbook-の使い方)
    * [Vagrant を利用する場合](#vagrant-を利用する場合)
      * [Vagrant を利用する場合のコマンド実行例](#vagrant-を利用する場合のコマンド実行例)
    * [Ansible でプロビジョニングをする場合](#ansible-でプロビジョニングをする場合)
      * [Ansible を直接利用する場合のコマンド実行例](#ansible-を直接利用する場合のコマンド実行例)
  * [各ロール役割](#各ロール役割)


## この Playbook の使い方

### Vagrant を利用する場合

※ Ansible のセットアップなどは不要ですのでお手軽に始めることができると思います

- [Vagrant](https://www.vagrantup.com/) の実行環境を用意してください
- [isucon7-qualify](https://github.com/isucon/isucon7-qualify) を clone してください
- このディレクトリに移動してください
- 各言語の実行環境を用意するのに時間がかかるので、site.yml を編集して必要な言語実装のみプロビジョニングするようにすることをおすすめします
  - 例えば、golang の実装だけで良い場合は以下のよう使わない言語をコメントアウトします
```yaml
# -*- mode: Yaml; -*-
# vi: set ts=2 sw=2 sts=0 et:

---
- hosts: all
  user: ubuntu
  become: yes
  # for ansible_env
  gather_facts: yes

  roles:
    - common
    - bench
    - golang
#   - nodejs
#   - perl
#   - php
#   - python
#   - ruby
    - mysql
    - nginx
```
- `vagrant up` を行い VM の起動とプロビジョニングが行われます
- 起動が完了したら `vagrant ssh` したあとに `sudo -u isucon -i` で isucon ユーザに切り替えてご利用ください
- アプリケーションの動作切り替えなどは [ISUCON7予選 当日マニュアル](https://gist.github.com/941/8c64842b71995a2d448315e2594f62c2) を確認してください
- ベンチマークを実施する際は以下のように localhost に対して行います
```
$ cd /home/isucon/bench
$ # ベンチマークの実行
$ ./bin/bench -remotes=127.0.0.1 -output result.json
$ # 結果の確認
$ jq . < result.json
```


#### Vagrant を利用する場合のコマンド実行例

```console
$ git clone https://github.com/isucon/isucon7-qualify
$ cd isucon7/qualify/provisioning/allinone/
$ # プロビジョニングに時間がかかるので必要な言語実装だけにしたほうが良いです。
$ vi site.yml
$ vagrant up
$ vagrant ssh
[isucon7-qualify:~] $ sudo -u isucon -i
```

### Ansible でプロビジョニングをする場合

Vagrant を使わずにサーバなどへデプロイする場合は Ansible から Playbook を直接利用してください

- Ansible の環境が必要です（手元では Ansible 2.4 でのみ確認しています）
- [isucon7-qualify](https://github.com/isucon/isucon7-qualify) を clone してください
- このディレクトリに移動してください
- 各言語の実行環境を用意するのに時間がかかるので、site.yml を編集して必要な言語実装のみプロビジョニングするようにすることをおすすめします（詳しくは上記を確認してください）
- 対象ホストに対してプロビジョニングをじてください

#### Ansible を直接利用する場合のコマンド実行例

```console
$ git clone https://github.com/isucon/isucon7-qualify
$ cd isucon7/qualify/provisioning/allinone/
$ # プロビジョニングに時間がかかるので必要な言語実装だけにしたほうが良いです。
$ vi site.yml
$ ansible-playbook -i target-host, site.yml
```


## 各ロール役割

各ロールでやっていることは以下のようなものになります

- common
  - ユーザーの作成
  - xbuild のインストール
  - 依存パッケージのインストール
  - レポジトリのクローン
- bench
  - go の環境セットアップ
  - gb のインストール
  - benchmark プログラムの依存ライブラリをインストール
  - benchmark プログラムのコンパイル
  - dummy データの作成
- golang
  - go の環境セットアップは bench でしてるのでスキップ
  - dep のインストール
  - アプリケーションの build
  - systemd unit file の設置
- nodejs
  - nodejs の環境セットアップ
  - 依存パッケージのインストール
  - systemd unit file の設置
- perl
  - perl の環境セットアップ
  - 依存パッケージのインストール
  - systemd unit file の設置
- php
  - php の環境セットアップ
  - composer のインストール
  - 依存パッケージのインストール
  - systemd unit file の設置
  - php の設定ファイルの設置
- python
  - python の環境セットアップ
  - 依存パッケージのインストール
  - systemd unit file の設置
  - isubata.python を自動起動に設定
- ruby
  - ruby の環境セットアップ
  - 依存パッケージのインストール
  - systemd unit file の設置
- mysql
  - mysql のインストール
  - isucon ユーザの設定
  - 初期データの設置
- nginx
  - nginx のインストール
  - 設定ファイルの配置
