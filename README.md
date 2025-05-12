### Commands

#### 编译
```
go build -race example/main/main.go
```

#### 启动envoy
```
envoy -c sample/bootstrap-delta-ads.yaml --drain-time-s 1 -l debug
```

#### 启动control plane
```
./main -debug
```

#### 检查envoy config
```
curl 127.0.0.1:19000/clusters
curl 127.0.0.1:19000/listeners
curl 127.0.0.1:19000/config_dump
```