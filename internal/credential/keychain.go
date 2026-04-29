package credential

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
)

const keychainService = "app.paulie.agent-dd"

func keychainStore(name, apiKey, appKey string) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("keychain not available")
	}

	data, _ := json.Marshal(map[string]string{
		"api_key": apiKey,
		"app_key": appKey,
	})

	_ = exec.Command("security", "delete-generic-password", "-s", keychainService, "-a", name).Run()

	return exec.Command("security", "add-generic-password",
		"-s", keychainService, "-a", name, "-w", string(data),
		"-U",
	).Run()
}

func keychainGet(name string) (apiKey, appKey string, err error) {
	if runtime.GOOS != "darwin" {
		return "", "", fmt.Errorf("keychain not available")
	}

	out, err := exec.Command("security", "find-generic-password",
		"-s", keychainService, "-a", name, "-w",
	).Output()
	if err != nil {
		return "", "", err
	}

	var keys map[string]string
	if err := json.Unmarshal(out, &keys); err != nil {
		return "", "", err
	}
	return keys["api_key"], keys["app_key"], nil
}

func keychainDelete(name string) {
	if runtime.GOOS != "darwin" {
		return
	}
	_ = exec.Command("security", "delete-generic-password", "-s", keychainService, "-a", name).Run()
}
