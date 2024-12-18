package workspaceChecker

import (
	"context"
	"log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"resourceManager/utils/mysql"
)

func WorkspaceChecker(clientset *kubernetes.Clientset, namespace string) error {
	for {
		labelSelector := "app=gpushare"
		pods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			log.Println("Failed to get Pod: %v", err)
			//time.Sleep(5 * time.Second)
			continue
		}

		for _, pod := range pods.Items {
			if pod.Status.Phase == metav1.PodSucceeded {
				log.Println("[INFO] Pod %s is in Completed state", pod.Name)
				results, err := mysql.GetAvailableResource(clientset)
				if err != nil {
					log.Fatalf("Fail: %v", err)
				}

				err = mysql.ReturnResource()
			}
		}
	}
}
