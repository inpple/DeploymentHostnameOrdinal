#创建yaml

kubectl create -f apply  yaml/*
ClusterRole将创建一个名为 pod-reader 的 ClusterRole，具有获取、观察和列出 Pods 的权限。
ClusterRoleBinding把 pod-reader ClusterRole 绑定到 crd 命名空间中的 default 服务账户
pod.yaml是MutatingWebhook

#部署
# 默认使用goproxy.cn
export GOPROXY=https://goproxy.cn
# input your command here
#go mod init example.com/m/v2

go mod tidy 

GOOS=linux GOARCH=amd64 CGO_ENABLED=0  go build -v -o main .


Deployment更换hostname  格式Deployment-序号
主要通过pod.yaml去控制标签
