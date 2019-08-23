import subprocess
import sys
import json
import numpy as np

'''
usage
$ python3 bench.py 10
-> 10回ベンチを回し、結果を表示する
'''


args = sys.argv
loop = int(args[1])

scores = []
for i in range(loop):
    print("ベンチマーク中　", i+1, " / ", loop)
    res = subprocess.call("bin/bench -remotes=127.0.0.1 -output result.json", shell=True)
    f = open("result.json", "r")
    score = json.load(f)["score"]
    scores.append(score)

for i, score in enumerate(scores):
    print(i+1, "回目のベンチマークスコア: ", score)

print("平均: ", np.mean(scores))
print("中央: ", np.median(scores))
print("最小: ", np.amin(scores))
print("最大: ", np.amax(scores))
print("------------------------------------------------------------")
