package main

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "sort"
    "strconv"
    "sync"

    "k8s.io/api/admission/v1beta1"
    v1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/rest"
)

type PodHostnameTracker struct {
    clientset *kubernetes.Clientset
    lock      sync.Mutex
}

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

func (tracker *PodHostnameTracker) GetNextHostname(namespace, deploymentName string) (string, error) {
    tracker.lock.Lock()
    defer tracker.lock.Unlock()

    pods, err := tracker.clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
        LabelSelector: "app=" + deploymentName,
    })
    if err != nil {
        return "", err
    }

    usedNumbers := make(map[int]bool)
    for _, pod := range pods.Items {
        if hostname := pod.Labels["hostname"]; hostname != "" {
            if number, err := strconv.Atoi(hostname); err == nil {
                usedNumbers[number] = true
            }
        }
    }

    for i := 1; i <= 50; i++ {
        if !usedNumbers[i] {
            return fmt.Sprintf("%s-%d", deploymentName, i), nil
        }
    }

    return "", fmt.Errorf("no available hostname number found")
}

var hostnameTracker *PodHostnameTracker

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

    deploymentName := pod.GetLabels()["app"]
    hostname, err := hostnameTracker.GetNextHostname(pod.Namespace, deploymentName)
    if err != nil {
        http.Error(w, fmt.Sprintf("error getting next hostname: %v", err), http.StatusInternalServerError)
        return
    }

    patch := []map[string]interface{}{
        {
            "op":    "add",
            "path":  "/metadata/labels/hostname",
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
    var err error
    hostnameTracker, err = NewPodHostnameTracker()
    if err != nil {
        fmt.Printf("Failed to initialize hostname tracker: %v\n", err)
        return
    }

    http.HandleFunc("/mutate", handleMutate)
    fmt.Println("Starting webhook server...")
    if err := http.ListenAndServeTLS(":8443", "/app/tls/tls.crt", "/app/tls/tls.key", nil); err != nil {
        fmt.Printf("Failed to start server: %v\n", err)
    }
}
