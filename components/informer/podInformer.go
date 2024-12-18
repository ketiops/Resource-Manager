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
)

var (
	podStatusCache = make(map[string]corev1.PodPhase)
	cacheMutex     sync.Mutex
)

func GetVRAMFromPod(pod *corev1.Pod) (int, error) {
	for _, container := range pod.Spec.Containers {
		if gpuMem, ok := container.Resources.Limits[corev1.ResourceName("aliyun.com/gpu-mem")]; ok {
			quantityValue := gpuMem.Value()
			return int(quantityValue), nil
		}
	}

	return 0, fmt.Errorf("[ERROR] Resource gpu-mem not found in any container")
}

func CreatePodInformer(clientset *kubernetes.Clientset) cache.SharedInformer {
	// Create K8S Informer
	factory := informers.NewSharedInformerFactory(clientset, 30*time.Second)
	informer := factory.Core().V1().Pods().Informer()

	// Define event handlers
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod := obj.(*corev1.Pod)
			if pod.Namespace == namespace {
				log.Printf("[INFO] Pod added in namespace %s: %s\n", namespace, pod.Name)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			pod := newObj.(*corev1.Pod)

			cacheMutex.Lock()
			defer cacheMutex.Unlock()

			oldPhase, exists := podStatusCache[pod.Name]

			if !exists || oldPhase != pod.Status.Phase {
				if pod.Namespace == namespace && pod.Status.Phase == corev1.PodSucceeded {
					log.Printf("[INFO] Pod completed in namespace %s: %s\n", namespace, pod.Name)

					gpuMem, err := GetVRAMFromPod(pod)
					if err != nil {
						log.Printf("[ERROR] Error getting vram: %v", err)
					}

					err = clientset.CoreV1().Pods(namespace).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{})
					if err != nil {
						log.Printf("[ERROR] Error deleting pod %s: %v", pod.Name, err)
					} else {
						log.Printf("[INFO] Pod %s deleted successfully\n", pod.Name)
					}

					results, err := mysql.GetAvailableResource(clientset)
					if err != nil {
						log.Printf("[ERROR] Fail to get resource: %v", err)
					}

					for _, result := range results {
						if result["gpu_index"].(string) == "0" && result["node_name"].(string) == pod.Spec.NodeName {
							if result["vram_remain"].(int) != result["total_vram"].(int) {
								err = mysql.ReturnResource(clientset, result["node_name"].(string), result["gpu_index"].(string), result["vram_usage"].(int), result["vram_remain"].(int), result["is_available"].(int), gpuMem)
								if err != nil {
									log.Printf("[ERROR] %v", err)
									break
								}
								break
							}
						}
					}
				}

				podStatusCache[pod.Name] = pod.Status.Phase
			}
		},
	})

	return informer
}
