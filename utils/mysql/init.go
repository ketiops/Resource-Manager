package mysql

import (
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	"k8s.io/client-go/kubernetes"
)

func InitDB(clientset *kubernetes.Clientset, hostName string, gpuIndex []string, vRAM []int) error {
	// Firstly, Get DB Connector
	db, err := GetDBConnector(clientset)
	if err != nil {
		return fmt.Errorf("[ERROR] Failed to get db connector for initializing db: %w", err)
	}

	// Secondly, Create Table, Table Name : gpuResource
	createTableSQL := `
                CREATE TABLE IF NOT EXISTS gpuResource(
                        id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
                        node_name VARCHAR(30) NOT NULL,
                        gpu_index TINYINT NOT NULL,
			total_vram SMALLINT NOT NULL,
			vram_usage SMALLINT NOT NULL,
			vram_remain SMALLINT NOT NULL,
			is_available TINYINT(1) NOT NULL
                );
        `

	_, err = db.Exec(createTableSQL)
	if err != nil {
		return fmt.Errorf("[ERROR] Failed to exec query(create table): %w", err)
	}

	// Thirdly, Extract column names and Insert initial data
	var columns []string

	selectColumnsSQL := `SELECT COLUMN_NAME
        	FROM INFORMATION_SCHEMA.COLUMNS
                WHERE TABLE_NAME = "gpuResource" AND COLUMN_NAME != "id";
        `

	rows, err := db.Query(selectColumnsSQL)
	if err != nil {
		return fmt.Errorf("[ERROR] Failed to get rows from table: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var columnName string
		if err := rows.Scan(&columnName); err != nil {
			return fmt.Errorf("[ERROR] Failed to scan(column_name): %w", err)
		}
		columns = append(columns, columnName)
	}

	columnPlaceholders := ""
	for i, column := range columns {
		if i > 0 {
			columnPlaceholders += ", "
		}
		columnPlaceholders += column
	}

	for i := 0; i < len(gpuIndex); i++ {
		count := 0

		checkDuplicateSQL := `SELECT COUNT(*) FROM gpuResource WHERE node_name = ? AND gpu_index = ?`

		err = db.QueryRow(checkDuplicateSQL, hostName, gpuIndex[i]).Scan(&count)
		if err != nil {
			return fmt.Errorf("[ERROR] Failed to exec query(check duplicate): %w", err)
		}

		if count > 0 {
			continue
		}

		insertInitialDataSQL := fmt.Sprintf(`
			INSERT INTO gpuResource (%s) VALUES ("%s", "%s", %d, %d, %d, %d)
                `, columnPlaceholders, hostName, gpuIndex[i], vRAM[i], 0, vRAM[i], 1)

		_, err = db.Exec(insertInitialDataSQL)
		if err != nil {
			return fmt.Errorf("[ERROR] Failed to exec query(insert values): %w", err)
		}
	}

	return nil
}
