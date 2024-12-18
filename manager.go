package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"path/filepath"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	//"resourceManager/conf"
	"resourceManager/components/deployManager"
	"resourceManager/utils/nvidia"
	//corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
	"resourceManager/components/informer"
	"resourceManager/utils/mysql"
)

var (
	clientset   *kubernetes.Clientset
	namespace   = "xrcloud"
	secretNames []string
	ids         []string
	vram        []int
)

func init() {
	kubeconfig := filepath.Join(homedir.HomeDir(), ".kube", "config")

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		log.Fatalf("Failed to build config: %v", err)
	}

	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to create clientset: %v", err)
	}

	log.Println("[INFO] Create k8s client, successfully")

	// Get secret name defined root password
	secrets, err := clientset.CoreV1().Secrets(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: "info=server",
	})
	if err != nil {
		log.Fatalf("Fail: %v", err)
	}

	for _, secret := range secrets.Items {
		secretNames = append(secretNames, secret.Name)
	}

	// Extracting GPU resources from each server through secret
	// Then, Initialize database with GPU resources
	for _, secretName := range secretNames {
		servers, err := nvidia.GetServerInfo(clientset, namespace, secretName)
		if err != nil {
			log.Fatalf("Fail: %v", err)
		}

		for _, server := range servers {
			ids, vram, err = nvidia.GetGPUMemoryPerIndex(server.IPAddr, server.Password)
			if err != nil {
				log.Fatalf("Fail: %v", err)
			}
			err = mysql.InitDB(clientset, server.NodeName, ids, vram)
			if err != nil {
				log.Fatalf("Fail: %v", err)
			}
		}
	}

	log.Println("[INFO] Initialize Database, successfully")
}

func main() {
	http.HandleFunc("/create", deployManager.DeployPodHandler(clientset))
	go func() {
		port := 31000
		if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
			log.Fatalf("Error starting server: %s", err.Error())
		}
	}()

	podInformer := informer.CreatePodInformer(clientset)
	stopCh := make(chan struct{})
	defer close(stopCh)

	go podInformer.Run(stopCh)

	if !cache.WaitForCacheSync(stopCh, podInformer.HasSynced) {
		log.Fatalf("[ERROR] Failed to sync informer cache")
	}

	log.Println("[INFO] Started monitoring for pods...")

	//informer.StartMonitoringNode(clientset)
	select {}
	/*

			err = mysql.AllocateVRAMResource(clientset, "ketiops-gpu-node-1", "0", 2)
			if err != nil {
				log.Fatalf("Fail: %v", err)
			}

		for {
			results, err := mysql.GetAvailableResource(clientset)
			if err != nil {
				log.Fatalf("Fail: %v", err)
			}
			/*
				for _, result := range results {
					if result["is_available"] != 0 {
						err = mysql.AllocateResource(clientset, result["node_name"].(string), result["gpu_index"].(string), result["total_vram"].(int), result["vram_usage"].(int), result["vram_remain"].(int), result["is_available"].(int), 2)
						if err != nil {
							log.Println(err.Error())
							break
						}
						break
					}

					//log.Println(result["node_name"], result["gpu_index"], result["vram_usage"], result["vram_remain"])
				}
				log.Println("[INFO] There are no available resources")
	*/
	/*
			for _, result := range results {
				if result["gpu_index"] == "0" && result["node_name"] == "ketiops-gpu-node-1" {
					if result["vram_remain"] != result["total_vram"] {
						err = mysql.ReturnResource(clientset, result["node_name"].(string), result["gpu_index"].(string), result["vram_usage"].(int), result["vram_remain"].(int), result["is_available"].(int), 2)
						if err != nil {
							log.Println(err.Error())
							break
						}
						break
					}
				}
			}

			log.Println("[INFO] All node's resources are available")
			time.Sleep(5 * time.Second)
		}
	*/
	/*
		// deploy manager
		results, err := mysql.GetAvailableResource(clientset)
		if err != nil {
			log.Fatalf("Fail: %v", err)
		}

		name := "deploymanger-test5"
		image := "chromatices/mnist-torch:1.0"
		namespace := "xrcloud"
		reqVRAM := 5

		for _, result := range results {
			if result["is_available"] != 0 {
				if result["vram_remain"].(int) != 0 && reqVRAM <= result["vram_remain"].(int) {
					err = deployManager.DeployPod(clientset, result["node_name"].(string), name, namespace, image, result["gpu_index"].(string), reqVRAM)
					if err != nil {
						log.Println(err.Error())
						break
					}

					err = mysql.AllocateResource(clientset, result["node_name"].(string), result["gpu_index"].(string), result["total_vram"].(int), result["vram_usage"].(int), result["vram_remain"].(int), result["is_available"].(int), reqVRAM)
					if err != nil {
						log.Println(err.Error())
						break
					}

					break
				}
			}
			log.Printf("[INFO] Allocated/Total GPU %s's vram : %d/%d", result["gpu_index"].(string), result["vram_usage"].(int), result["vram_remain"].(int))
			log.Println("[INFO] There are no available vram resource in [" + result["node_name"].(string) + "]'s GPU " + result["gpu_index"].(string))
		}
	*/
}
