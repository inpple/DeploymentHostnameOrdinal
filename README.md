# 适配go-zero的volume模式，目前支持最大50个，避免性能影响

Deployment的pod发生变动的时候将pod分配hostname字段 格式Deployment-序号
主要通过pod.yaml去控制标签

#创建yaml

kubectl create -f apply  yaml/*
ClusterRole将创建一个名为 pod-reader 的 ClusterRole，具有获取、观察和列出 Pods 的权限。

ClusterRoleBinding把 pod-reader ClusterRole 绑定到 crd 命名空间中的 default 服务账户

pod.yaml是MutatingWebhook

#部署
#默认使用goproxy.cn
#export GOPROXY=https://goproxy.cn
# input your command here
//go mod init example.com/m/v2

go mod tidy 

GOOS=linux GOARCH=amd64 CGO_ENABLED=0  go build -v -o main .

效果图

![image](https://github.com/inpple/DeploymentHostnameOrdinal/assets/39829594/3fcb369c-72cb-46db-9327-5ac1a42800a3)
