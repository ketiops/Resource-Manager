package mysql

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	NAMESPACE = "xrcloud"
	DB_HOST   = "tcp(10.0.1.110:30306)"
	DB_NAME   = "resourceBoard"
	DB_USER   = "root"
)

var (
	DB_PASS       string  = ""
	DB_CONNECTION string  = ""
	DB_Conn       *sql.DB = nil
)

func GetDBConnector(clientset *kubernetes.Clientset) (*sql.DB, error) {
	secretName := "mysql-root"
	opts := metav1.GetOptions{}
	secret, err := clientset.CoreV1().Secrets(NAMESPACE).Get(context.TODO(), secretName, opts)
	if err != nil {
		return nil, fmt.Errorf("[ERROR] Failed to get k8s secret: %w", err)
	}

	for key, value := range secret.Data {
		if key == "password" {
			DB_PASS = string(value)
		}
	}

	DB_CONNECTION = DB_USER + ":" + DB_PASS + "@" + DB_HOST + "/" + DB_NAME

	if DB_Conn == nil {
		DB_Conn, err = sql.Open("mysql", DB_CONNECTION)
	}

	return DB_Conn, nil
}
