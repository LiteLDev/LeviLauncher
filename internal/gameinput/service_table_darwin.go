package gameinput

import (
	"errors"
	"os"
	"slices"
	"strings"
)

func disableGameInputService(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	// The first three lines of the Wine msidb export hold column names,
	// column types, and table and key information.
	lines := strings.Split(string(data), "\r\n")
	columns := strings.Split(lines[0], "\t")
	name, startType := slices.Index(columns, "Name"), slices.Index(columns, "StartType")
	if len(lines) < 4 || name == -1 || startType == -1 {
		return errors.New("unexpected GameInput ServiceInstall table")
	}
	for index, line := range lines[3:] {
		fields := strings.Split(line, "\t")
		if len(fields) == len(columns) && fields[name] == "GameInputRedistService" {
			fields[startType] = "4" // SERVICE_DISABLED
			lines[index+3] = strings.Join(fields, "\t")
			return os.WriteFile(path, []byte(strings.Join(lines, "\r\n")), 0600)
		}
	}
	return errors.New("GameInput installer service row not found")
}
