# 使い方

```console
$ ./spec.sh
spec.sh check qualifier server.
Usage:
  spec.sh [<options>]
Options:
  --target, -t   single target
  --debug, -d    debug mode.
  --help, -h     print this help.
Example:
  spec.sh -t app0022
```

## 違う goss.yaml を読み込ませる時

```console
GOSS_FILE=./reboot.yaml ./spec.sh -i ./isu7q-dummy.tsv
```

