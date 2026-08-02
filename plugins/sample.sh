#!/bin/sh
# サンプルプラグイン: 2 秒ごとにダミー変更ログを出力する
i=0
while :; do
  i=$((i + 1))
  echo "plugin tick #$i $(date '+%H:%M:%S')"
  sleep 2
done
