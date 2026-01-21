package mcservice

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/types"
	"github.com/liteldev/LeviLauncher/internal/utils"
)

func ListServers(versionName string, player string) ([]types.Server, error) {
	roots := GetContentRoots(versionName)
	if roots.UsersRoot == "" || player == "" {
		return []types.Server{}, nil
	}

	serverFile := filepath.Join(roots.UsersRoot, player, "games", "com.mojang", "minecraftpe", "external_servers.txt")
	if !utils.FileExists(serverFile) {
		return []types.Server{}, nil
	}

	file, err := os.Open(serverFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var servers []types.Server
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, ":")
		// Format: Index:ServerName:IP:Port:Timestamp
		// Example: 1:12312:127.0.0.1:19132:1768910786
		if len(parts) >= 5 {
			ts, _ := strconv.ParseInt(parts[4], 10, 64)
			servers = append(servers, types.Server{
				Index:     parts[0],
				Name:      parts[1],
				IP:        parts[2],
				Port:      parts[3],
				Timestamp: ts,
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return servers, nil
}
