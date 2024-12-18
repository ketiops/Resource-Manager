package deployManager

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"resourceManager/conf"
	"resourceManager/utils/mysql"
)

func CreatePodSpec(nodeName string, podName string, namespace string, imgName string, gpuIndex string, vram int) *corev1.Pod {
	now := time.Now()

	annotations := map[string]string{
		"ALIYUN_COM_GPU_MEM_IDX":         gpuIndex,
		"ALIYUN_COM_GPU_MEM_ASSIGNED":    "false",
		"ALIYUN_COM_GPU_MEM_ASSUME_TIME": fmt.Sprintf("%d", now.UnixNano()),
	}

	labels := map[string]string{
		"app": "gpushare",
	}

	podSpec := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        podName,
			Namespace:   namespace,
			Annotations: annotations,
			Labels:      labels,
		},
		Spec: corev1.PodSpec{
			NodeName:      nodeName,
			RestartPolicy: corev1.RestartPolicyNever,
			ImagePullSecrets: []corev1.LocalObjectReference{
				{
					Name: "regcred",
				},
			},
			Containers: []corev1.Container{
				{
					Name:            podName,
					Image:           imgName,
					ImagePullPolicy: corev1.PullAlways,
					Env: []corev1.EnvVar{
						{
							Name:  "NVIDIA_VISIBLE_DEVICES",
							Value: gpuIndex,
						},
					},
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceName("aliyun.com/gpu-mem"): resource.MustParse(fmt.Sprintf("%d", vram)),
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "shmdir",
							MountPath: "dev/shm",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "shmdir",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{
							Medium:    corev1.StorageMediumMemory,
							SizeLimit: resource.NewQuantity(4*1024*1024*1024, resource.BinarySI),
						},
					},
				},
			},
		},
	}

	return podSpec
}

func DeployPodHandler(clientset *kubernetes.Clientset) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "[ERROR] Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		var req conf.PodCreationRequest

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "[ERROR] Invalid request body", http.StatusBadRequest)
			return
		}

		for {
			results, err := mysql.GetAvailableResource(clientset)
			if err != nil {
				http.Error(w, fmt.Sprintf("[ERROR] Failed to get available resources: %v", err), http.StatusInternalServerError)
				log.Printf("[ERROR] Fail to get resource: %v", err)
				return
			}

			for _, result := range results {
				if result["is_available"].(int) != 0 {
					podSpec := CreatePodSpec(result["node_name"].(string), req.PodName, "xrcloud", req.Image, result["gpu_index"].(string), req.VRAMReq)

					_, err := clientset.CoreV1().Pods("xrcloud").Create(context.TODO(), podSpec, metav1.CreateOptions{})
					if err != nil {
						if k8sErrors.IsAlreadyExists(err) {
							http.Error(w, fmt.Sprintf("[ERROR] Pod %s already exists in namespace %s", req.PodName, "xrcloud"), http.StatusConflict)
							return
						}

						http.Error(w, fmt.Sprintf("[ERROR] Error creating pod: %v", err), http.StatusInternalServerError)
						return
					}

					log.Println("[INFO] Created pod using gpu resource - " + req.PodName + " in namespace [xrcloud]")

					err = mysql.AllocateResource(clientset, result["node_name"].(string), result["gpu_index"].(string), result["total_vram"].(int), result["vram_usage"].(int), result["vram_remain"].(int), result["is_available"].(int), 2)
					if err != nil {
						http.Error(w, fmt.Sprintf("[ERROR] Failed to allocate resources: %v", err), http.StatusInternalServerError)
						log.Printf("[ERROR] %v", err)
						return
					}

					responseMessage := fmt.Sprintf("[INFO] Pod '%s' created successfully in namespace [%s]\n", req.PodName, "xrcloud")
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(responseMessage))

					return
				} else {
					log.Println("[INFO] There are no available resources")
				}
			}
		}
	}
}
