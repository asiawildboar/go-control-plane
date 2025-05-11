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
bin/example -debug
```
