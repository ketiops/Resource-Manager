package mysql

import (
	"fmt"
	"log"

	_ "github.com/go-sql-driver/mysql"
	"k8s.io/client-go/kubernetes"
)

func GetAvailableResource(clientset *kubernetes.Clientset) ([]map[string]interface{}, error) {
	// Extract available resources
	// Firstly, Get DB Connector
	db, err := GetDBConnector(clientset)
	if err != nil {
		return nil, fmt.Errorf("[ERROR] Failed to get db connector for getting gpu resource: %w", err)
	}

	// Secondly, get gpu resource from db
	selectSQL := fmt.Sprintf(`SELECT node_name, gpu_index, total_vram, vram_usage, vram_remain, is_available FROM gpuResource`)

	rows, err := db.Query(selectSQL)
	if err != nil {
		return nil, fmt.Errorf("[ERROR] Failed to get rows from table: %w", err)
	}
	defer rows.Close()

	// Lastly, Scan rows and extract values
	var results []map[string]interface{}

	for rows.Next() {
		var nodeName, gpuIndex string
		var totalVram, vramUsage, vramRemain, available int
		if err = rows.Scan(&nodeName, &gpuIndex, &totalVram, &vramUsage, &vramRemain, &available); err != nil {
			return nil, fmt.Errorf("[ERROR] Failed to scan gpu resource: %w", err)
		}

		row := map[string]interface{}{
			"node_name":    nodeName,
			"gpu_index":    gpuIndex,
			"total_vram":   totalVram,
			"vram_usage":   vramUsage,
			"vram_remain":  vramRemain,
			"is_available": available,
		}

		results = append(results, row)
	}

	log.Println("[INFO] Get node's resource, successfully")

	return results, nil
}

func AllocateResource(clientset *kubernetes.Clientset, nodeName string, gpuIndex string, totalVram int, vramUsage int, vramRemain int, available int, vramReq int) error {
	// Get DB Connector and Update DB
	db, err := GetDBConnector(clientset)
	if err != nil {
		return fmt.Errorf("[ERROR] Failed to get db connector for allocating resource: %w", err)
	}

	// Update DB
	if vramUsage+vramReq == totalVram && vramRemain-vramReq == 0 {
		updateSQL := "UPDATE gpuResource SET vram_usage = ?, vram_remain = ?, is_available = ? WHERE gpu_index = ? AND node_name = ?"
		available = 0

		_, err = db.Exec(updateSQL, vramUsage+vramReq, vramRemain-vramReq, available, gpuIndex, nodeName)
		if err != nil {
			return fmt.Errorf("[ERROR] Failed to exec query(update gpuResource): %w", err)
		}
	} else {
		updateSQL := "UPDATE gpuResource SET vram_usage = ?, vram_remain = ? WHERE gpu_index = ? AND node_name = ?"
		_, err = db.Exec(updateSQL, vramUsage+vramReq, vramRemain-vramReq, gpuIndex, nodeName)
		if err != nil {
			return fmt.Errorf("[ERROR] Failed to exec query(update gpuResource): %w", err)
		}
	}

	log.Println("[INFO] Allocate Resource, successfully")

	return nil
}

func ReturnResource(clientset *kubernetes.Clientset, nodeName string, gpuIndex string, vramUsage int, vramRemain int, available int, vramReq int) error {
	// Get DB Connector and Update DB
	db, err := GetDBConnector(clientset)
	if err != nil {
		return fmt.Errorf("[ERROR] Failed to get db connector for returning resource: %w", err)
	}

	// Update DB
	if vramRemain == 0 {
		updateSQL := "UPDATE gpuResource SET vram_usage = ?, vram_remain = ?, is_available = ? WHERE gpu_index = ? AND node_name = ?"
		available = 1

		_, err = db.Exec(updateSQL, vramUsage-vramReq, vramRemain+vramReq, available, gpuIndex, nodeName)
		if err != nil {
			return fmt.Errorf("[ERROR] Failed to exec query(update gpuResource): %w", err)
		}
	} else {
		updateSQL := "UPDATE gpuResource SET vram_usage = ?, vram_remain = ? WHERE gpu_index = ? AND node_name = ?"
		available = 1

		_, err = db.Exec(updateSQL, vramUsage-vramReq, vramRemain+vramReq, gpuIndex, nodeName)
		if err != nil {
			return fmt.Errorf("[ERROR] Failed to exec query(update gpuResource): %w", err)
		}
	}

	log.Println("[INFO] Return Resource, successfully")

	return nil
}

func InsertNewResource(clientset *kubernetes.Clientset, hostName string, gpuIndex []string, vRAM []int) error {
	// Firstly, Get DB Connector
	db, err := GetDBConnector(clientset)
	if err != nil {
		return fmt.Errorf("[ERROR] Failed to get db connector for initializing db: %w", err)
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
		checkSQL := `SELECT COUNT(*) FROM gpuResource
                        WHERE node_name = ? AND gpu_index = ?
                `

		var count int
		err = db.QueryRow(checkSQL, hostName, gpuIndex[i]).Scan(&count)
		if err != nil {
			return fmt.Errorf("[ERROR] Failed to exec query(check values): %w", err)
		}

		if count > 0 {
			continue
		}

		insertNewDataSQL := fmt.Sprintf(`
			INSERT INTO gpuResource (%s) VALUES ("%s", "%s", %d, %d, %d, %d)
                `, columnPlaceholders, hostName, gpuIndex[i], vRAM[i], 0, vRAM[i], 1)

		_, err = db.Exec(insertNewDataSQL)
		if err != nil {
			return fmt.Errorf("[ERROR] Failed to exec query(insert values): %w", err)
		}
	}

	return nil
}
