# minikube

```
minikube start
minikube dashboard
```

# docker

以下を実行し、DOCKER ENDPOINT を minikube に変更

```
eval $(minikube docker-env)
```

# DB(mysql)

```
docker build -t k8s/isucon-7-db:v1.0.0 . -f dbDockerfile
kubectl apply -f kube/db-deployment.yaml
kubectl apply -f kube/db-service.yaml
```

# APP(go-backend)

```
docker build -t k8s/isucon-7-app:v1.0.0 . -f appDockerfile
kubectl apply -f kube/app-deployment.yaml
kubectl apply -f kube/app-service.yaml
```

# WEB(nginx)

```
docker build -t k8s/isucon-7-web:v1.0.0 . -f webDockerfile
kubectl apply -f kube/web-deployment.yaml
kubectl apply -f kube/web-service.yaml
```

# 動作確認

```
minikube tunnel

(別のterminalで)
kubectl get svc
  出力例:
    isucon-7-web   LoadBalancer isucon-7-web   LoadBalancer   10.108.43.30    127.0.0.1     80:30507/TCP   4m12s
(出力例の場合、ブラウザで以下にアクセス)
  127.0.0.1:80
```

# bench

```
docker build -t k8s/isucon-7-bench:v1.0.0 . -f benchDockerfile
kubectl apply -f kube/bench-job.yaml
(結果確認)
kubectl get pod
  出力例:
    isucon-7-bench-9bnfp            0/1     Completed   0          5m6s
kubectl logs isucon-7-bench-9bnfp

(二回目以降)
kubectl replace --force -f kube/bench-job.yaml
```
