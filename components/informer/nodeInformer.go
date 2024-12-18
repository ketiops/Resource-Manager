package informer

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"resourceManager/utils/mysql"
	"resourceManager/utils/nvidia"
)

var (
	existingNodes = map[string]struct{}{}
	mu            sync.Mutex
	namespace     = "xrcloud"
	ids           []string
	vram          []int
)

func IsGpuShareNode(node *corev1.Node) bool {
	labels := node.GetLabels()
	return labels["gpushare"] == "true"
}

func LoadExistingNodes(clientset *kubernetes.Clientset) error {
	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("[ERROR] Failed to list nodes: %w", err)
	}

	mu.Lock()
	defer mu.Unlock()

	for _, node := range nodes.Items {
		existingNodes[node.Name] = struct{}{}
	}

	return nil
}

func GetNodeIP(node *corev1.Node) string {
	for _, address := range node.Status.Addresses {
		if address.Type == corev1.NodeInternalIP {
			return address.Address
		}
	}
	return ""
}

func WaitForSecret(clientset *kubernetes.Clientset, nodeName string) string {
	timeout := time.After(5 * time.Minute) // Adjust the timeout as needed
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			log.Fatalf("[ERROR] Timed out waiting for secret")
		case <-ticker.C:
			secret, err := clientset.CoreV1().Secrets(namespace).Get(context.TODO(), fmt.Sprintf("%s-root-password", nodeName), metav1.GetOptions{})
			if err == nil {
				password, ok := secret.Data["password"]
				if !ok {
					log.Fatalf("[ERROR] Password key not found in secret")
				}

				return string(password)
			}

			log.Printf("[INFO] Secret not found for node %s, retrying...", nodeName)
		}
	}
}

func CreateNodeInformer(clientset *kubernetes.Clientset) cache.SharedInformer {
	// Get exist node list
	err := LoadExistingNodes(clientset)
	if err != nil {
		log.Fatalf("[ERROR] Failed to load existing nodes: %v", err)
	}

	// Create K8S Informer
	factory := informers.NewSharedInformerFactory(clientset, 30*time.Second)
	informer := factory.Core().V1().Nodes().Informer()

	// Define event handlers
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			node := obj.(*corev1.Node)
			mu.Lock()
			defer mu.Unlock()

			if _, exists := existingNodes[node.Name]; !exists {
				existingNodes[node.Name] = struct{}{}

				if IsGpuShareNode(node) {
					log.Printf("[INFO] GPU nodes detected and insert gpu resources in database...")

					// Insert gpu resources in database
					// Get New gpu node's ip & password
					ip := GetNodeIP(node)
					foundSecret := WaitForSecret(clientset, node.Name)

					ids, vram, err := nvidia.GetGPUMemoryPerIndex(ip, foundSecret)
					if err != nil {
						log.Printf("[ERROR] Fail: %v", err)
					}

					err = mysql.InsertNewResource(clientset, node.Name, ids, vram)
					if err != nil {
						log.Printf("[ERROR] Failed to insert gpu resource: %v", err)
					}
				}
			}
		},
		// Optionally handle UpdateFunc and DeleteFunc
	})

	return informer
}
