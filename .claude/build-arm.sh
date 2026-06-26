#!/bin/bash
# ARM Linux 交叉编译脚本 (IAM300 工控机, ARMv5/ARM926EJ-S)
# 使用 Docker + golang:1.21 镜像 + arm-linux-gnueabi-gcc 工具链
# 依赖: Docker Desktop for Windows

docker run --rm \
  -v "E:/project/com-manager:/workspace" \
  -v "C:\\Users\\Administrator\\go:/root/go" \
  -w "//workspace" \
  -e GOPROXY=https://goproxy.cn,direct \
  -e GOPATH=//root//go \
  golang:1.21 bash -c "
    apt-get update -qq &&
    apt-get install -y -qq gcc-arm-linux-gnueabi > /dev/null 2>&1 &&
    CC=arm-linux-gnueabi-gcc \
    GOOS=linux \
    GOARCH=arm \
    GOARM=5 \
    CGO_ENABLED=1 \
    go build -o com-manager-arm main.go &&
    echo BUILD_SUCCESS
  "

# 上传到工控机 (FTP)
# curl -T com-manager-arm ftp://192.168.1.233/com-manager-arm --user root:yess

# 工控机上部署命令:
# pkill -9 com-manager-arm
# mv /com-manager-arm /home/com/com-manager-arm
# chmod +x /home/com/com-manager-arm
# cd /home/com && nohup ./com-manager-arm > /dev/null 2>&1 &
