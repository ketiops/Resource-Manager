package nvidia

import (
	"os/exec"
	"regexp"
	"strconv"
)

func GetGPUMemoryPerIndex(server string, password string) ([]string, []int, error) {
	var ids []string
	var vram []int

	indexoutput, err := exec.Command("sshpass", "-p", password, "ssh", "-o StrictHostKeyChecking=no", "root@"+server, "nvidia-smi --query-gpu=index --format=csv,noheader").Output()
	if err != nil {
		return nil, nil, err
	}

	for i, id := range indexoutput {
		if i%2 == 0 {
			ids = append(ids, string(id))
			memoutput, err := exec.Command("sshpass", "-p", password, "ssh", "-o StrictHostKeyChecking=no", "root@"+server, "nvidia-smi -i "+string(id)+" --query-gpu=memory.total --format=csv").Output()
			if err != nil {
				return nil, nil, err
			}

			outputStr := string(memoutput)

			re := regexp.MustCompile(`(\d+)\s+MiB`)
			match := re.FindStringSubmatch(outputStr)
			if len(match) > 1 {
				memoryMIB, err := strconv.Atoi(match[1])
				if err != nil {
					return nil, nil, err
				}
				vram = append(vram, memoryMIB/1024)
			}
		}
	}

	return ids, vram, nil
}
