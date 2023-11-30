package main

import (
    // 导入必要的包
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "strconv"
    "strings"
    "sync"

    // Kubernetes 相关的包
    "k8s.io/api/admission/v1beta1"
    v1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/rest"
)

// PodHostnameTracker 结构体用于追踪和分配 Pod 主机名
type PodHostnameTracker struct {
    clientset *kubernetes.Clientset // Kubernetes 客户端
    lock      sync.Mutex            // 保证并发访问的互斥锁
}

// NewPodHostnameTracker 创建并返回一个新的 PodHostnameTracker 实例
func NewPodHostnameTracker() (*PodHostnameTracker, error) {
    // 获取 Kubernetes 集群配置
    config, err := rest.InClusterConfig()
    if err != nil {
        return nil, err
    }
    // 创建 Kubernetes 客户端实例
    clientset, err := kubernetes.NewForConfig(config)
    if err != nil {
        return nil, err
    }
    return &PodHostnameTracker{clientset: clientset}, nil
}

// GetNextHostname 根据当前在特定命名空间和部署下的 Pod，生成下一个可用的主机名
func (tracker *PodHostnameTracker) GetNextHostname(namespace, deploymentName string) (string, error) {
    tracker.lock.Lock() // 加锁以保证线程安全
    defer tracker.lock.Unlock() // 函数结束时解锁

    // 获取指定命名空间和标签的所有 Pod
    pods, err := tracker.clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
        LabelSelector: "app=" + deploymentName,
    })
    if err != nil {
        return "", err
    }

    // 分析现有 Pod 的主机名，记录已经使用的编号
    usedNumbers := make(map[int]bool)
    for _, pod := range pods.Items {
        if parts := strings.Split(pod.Name, "-"); len(parts) > 1 {
            if num, err := strconv.Atoi(parts[len(parts)-1]); err == nil {
                usedNumbers[num] = true
            }
        }
    }

    // 找到未使用的最小编号
    for i := 1; i <= 50; i++ {
        if !usedNumbers[i] {
            return fmt.Sprintf("%s-%d", deploymentName, i), nil
        }
    }

    return "", fmt.Errorf("no available hostname number found")
}

// handleMutate 处理 Kubernetes 准入控制器的 Webhook 请求
func handleMutate(tracker *PodHostnameTracker, w http.ResponseWriter, r *http.Request) {
    var review v1beta1.AdmissionReview
    if err := json.NewDecoder(r.Body).Decode(&review); err != nil {
        http.Error(w, fmt.Sprintf("decode error: %v", err), http.StatusBadRequest)
        return
    }

    // 解析请求中的 Pod 对象
    var pod v1.Pod
    if err := json.Unmarshal(review.Request.Object.Raw, &pod); err != nil {
        http.Error(w, fmt.Sprintf("unmarshal error: %v", err), http.StatusBadRequest)
        return
    }

    // 获取 Pod 的部署名称
    deploymentName := pod.GetLabels()["app"]
    // 生成新的主机名
    hostname, err := tracker.GetNextHostname(pod.Namespace, deploymentName)
    if err != nil {
        http.Error(w, fmt.Sprintf("error getting next hostname: %v", err), http.StatusInternalServerError)
        return
    }

    // 创建 JSON Patch 来更新 Pod 的主机名
    patch := []map[string]interface{}{
        {
            "op":    "add",
            "path":  "/spec/hostname",
            "value": hostname,
        },
    }
    patchBytes, err := json.Marshal(patch)
    if err != nil {
        http.Error(w, fmt.Sprintf("marshal error: %v", err), http.StatusInternalServerError)
        return
    }

    // 构造准入控制响应
    review.Response = &v1beta1.AdmissionResponse{
        UID:       review.Request.UID,
        Allowed:   true,
        Patch:     patchBytes,
        PatchType: func() *v1beta1.PatchType {
            pt := v1beta1.PatchTypeJSONPatch
            return &pt
        }(),
    }

    // 发送响应
    if err := json.NewEncoder(w).Encode(review); err != nil {
        http.Error(w, fmt.Sprintf("encode error: %v", err), http.StatusInternalServerError)
    }
}

// 主函数
func main() {
    // 初始化 PodHostnameTracker
    tracker, err := NewPodHostnameTracker()
    if err != nil {
        fmt.Printf("Failed to initialize hostname tracker: %v\n", err)
        return
    }

    // 设置 HTTP 路由和处理函数
    http.HandleFunc("/mutate", func(w http.ResponseWriter, r *http.Request) {
        handleMutate(tracker, w, r)
    })
    fmt.Println("Starting webhook server...")

    // 启动 HTTPS 服务器
    if err := http.ListenAndServeTLS(":8443", "/app/tls.crt", "/app/tls.key", nil); err != nil {
        fmt.Printf("Failed to start server: %v\n", err)
    }
}
