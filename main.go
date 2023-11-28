package main

import (
    "encoding/json"
    "fmt"
    "net/http"
    "sync"
    "k8s.io/api/admission/v1beta1"
    "k8s.io/api/core/v1"
)

// PodHostnameTracker 跟踪每个 Deployment 的 Pod 序号
type PodHostnameTracker struct {
    usedNumbers map[string][]bool
    lock        sync.Mutex
}

func NewPodHostnameTracker() *PodHostnameTracker {
    return &PodHostnameTracker{
        usedNumbers: make(map[string][]bool),
    }
}

func (tracker *PodHostnameTracker) GetNextHostname(deploymentName string) string {
    tracker.lock.Lock()
    defer tracker.lock.Unlock()

    used, ok := tracker.usedNumbers[deploymentName]
    if !ok {
        // 第一次为这个 Deployment 分配序号
        used = make([]bool, 50) // 序号从 1 到 50
    }

    // 查找未使用的最小序号
    for i := 0; i < 50; i++ {
        if !used[i] {
            used[i] = true
            tracker.usedNumbers[deploymentName] = used
            return fmt.Sprintf("%s-%d", deploymentName, i+1)
        }
    }

    // 所有序号都已使用，重置并重新开始
    used = make([]bool, 50)
    used[0] = true
    tracker.usedNumbers[deploymentName] = used
    return fmt.Sprintf("%s-1", deploymentName)
}

var hostnameTracker = NewPodHostnameTracker()

func handleMutate(w http.ResponseWriter, r *http.Request) {
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

    deploymentName := pod.GetLabels()["app"] // 假设 'app' 标签包含了 Deployment 名称
    hostname := hostnameTracker.GetNextHostname(deploymentName)

    patch := []map[string]string{
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

    review.Response = &v1beta1.AdmissionResponse{
        UID:     review.Request.UID,
        Allowed: true,
        Patch:   patchBytes,
        PatchType: func() *v1beta1.PatchType {
            pt := v1beta1.PatchTypeJSONPatch
            return &pt
        }(),
    }

    if err := json.NewEncoder(w).Encode(review); err != nil {
        http.Error(w, fmt.Sprintf("encode error: %v", err), http.StatusInternalServerError)
    }
}
func main() {
    http.HandleFunc("/mutate", handleMutate)
    fmt.Println("Starting webhook server...")
    if err := http.ListenAndServeTLS(":8443", "./tls.crt", "./tls.key", nil); err != nil {
        fmt.Printf("Failed to start server: %v", err)
    }
}
