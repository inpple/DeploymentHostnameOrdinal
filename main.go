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

    // Kubernetes 客户端库
    "k8s.io/api/admission/v1beta1"
    v1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/rest"
)

// PodHostnameTracker 是一个结构体，用于跟踪和获取下一个可用的主机名
type PodHostnameTracker struct {
    clientset *kubernetes.Clientset // Kubernetes 客户端集
    lock      sync.Mutex            // 用于同步的互斥锁
}

// NewPodHostnameTracker 创建并返回一个新的 PodHostnameTracker 实例
func NewPodHostnameTracker() (*PodHostnameTracker, error) {
    config, err := rest.InClusterConfig()
    if err != nil {
        return nil, err
    }
    clientset, err := kubernetes.NewForConfig(config)
    if err != nil {
        return nil, err
    }
    return &PodHostnameTracker{clientset: clientset}, nil
}

// GetNextHostname 为指定的命名空间和部署名获取下一个可用的主机名
func (tracker *PodHostnameTracker) GetNextHostname(namespace, deploymentName string) (string, error) {
    tracker.lock.Lock()
    defer tracker.lock.Unlock()

    // 获取指定部署的所有 Pods
    pods, err := tracker.clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
        LabelSelector: "app=" + deploymentName,
    })
    if err != nil {
        return "", err
    }

    usedNumbers := make(map[int]bool)
    for _, pod := range pods.Items {
        hostname := pod.Spec.Hostname
    
        // 如果 hostname 设置了，提取并标记使用过的编号
        if hostname != "" {
            trimmedHostname := strings.TrimPrefix(hostname, deploymentName+"-")
            num, err := strconv.Atoi(trimmedHostname)
            if err == nil {
                usedNumbers[num] = true
            }
        }
    }
    
    // 检查编号 1 是否已被使用
    if !usedNumbers[1] {
        // 如果编号 1 没有被使用，直接返回它
        return fmt.Sprintf("%s-%d", deploymentName, 1), nil
    }
    
    // 如果编号 1 已被使用，查找下一个可用编号
    for i := 2; i <= 50; i++ {
        if !usedNumbers[i] {
            return fmt.Sprintf("%s-%d", deploymentName, i), nil
        }
    }
    
    // 如果找不到可用编号，返回错误
    return "", fmt.Errorf("no available hostname number found")
    
}

// handleMutate 处理变更请求，为 Pod 设置唯一的主机名
func handleMutate(tracker *PodHostnameTracker, w http.ResponseWriter, r *http.Request) {
    var review v1beta1.AdmissionReview
    if err := json.NewDecoder(r.Body).Decode(&review); err != nil {
        http.Error(w, fmt.Sprintf("decode error: %v", err), http.StatusBadRequest)
        return
    }

    var pod v1.Pod
    if err := json.Unmarshal(review.Request.Object.Raw, &pod); err != nil {
        http.Error(w, fmt.Sprintf("unmarshal error: %v", err), http.StatusBadRequest)
        return
    }

    // 获取部署名，并生成新的主机名
    deploymentName := pod.GetLabels()["app"]
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

    // 设置返回的 AdmissionReview 响应
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

// main 函数设置 HTTP 服务器和处理路由
func main() {
    tracker, err := NewPodHostnameTracker()
    if err != nil {
        fmt.Printf("Failed to initialize hostname tracker: %v\n", err)
        return
    }

    // 设置 /mutate 路径的处理函数
    http.HandleFunc("/mutate", func(w http.ResponseWriter, r *http.Request) {
        handleMutate(tracker, w, r)
    })
    fmt.Println("Starting webhook server...")
    // 启动 HTTPS 服务器
    if err := http.ListenAndServeTLS(":8443", "/app/tls/tls.crt", "/app/tls/tls.key", nil); err != nil {
        fmt.Printf("Failed to start server: %v\n", err)
    }
}
