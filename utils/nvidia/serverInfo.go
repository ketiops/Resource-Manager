package nvidia

import (
	"context"
	"log"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"resourceManager/conf"
)

func GetServerInfo(clientset *kubernetes.Clientset, namespace string, secretName string) ([]conf.GPUNodeAddr, error) {
	labelSelector := "gpushare=true"

	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}

	var ip, nodeName, rootpass string
	var nodeInfos []conf.GPUNodeAddr

	for _, node := range nodes.Items {
		for _, address := range node.Status.Addresses {
			nodeName = node.Name

			if address.Type == corev1.NodeInternalIP {
				ip = address.Address
			}

			secret, err := clientset.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
			if err == nil {
				password, ok := secret.Data["password"]
				if !ok {
					log.Fatalf("[ERROR] Password key not found in secret")
				}
				rootpass = string(password)
			}

		}

		if ip != "" && nodeName != "" && rootpass != "" {
			nodeInfos = append(nodeInfos, conf.GPUNodeAddr{
				IPAddr:   ip,
				Password: rootpass,
				NodeName: nodeName,
			})
		}

	}

	return nodeInfos, nil
}
