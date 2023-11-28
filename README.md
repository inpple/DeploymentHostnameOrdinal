#先创建
kubectl create -f apply pod.yaml 
#然后部署
# 默认使用goproxy.cn
export GOPROXY=https://goproxy.cn
# input your command here
#go mod init example.com/m/v2
go mod tidy 

GOOS=linux GOARCH=amd64 CGO_ENABLED=0  go build -v -o main .


Deployment更换hostname  格式Deployment-序号
主要通过pod.yaml去控制标签