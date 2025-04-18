package util

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh/terminal"
)

// ANSI color utility function
var Pink = Color("\033[1;35m%s\033[0m")
var Cyan = Color("\033[1;36m%s\033[0m")

func Color(colorString string) func(...interface{}) string {
	sprint := func(args ...interface{}) string {
		return fmt.Sprintf(colorString,
			fmt.Sprint(args...))
	}
	return sprint
}

func GetBackupDirectory(osName string) (string, error) {
	switch osName {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			return "", fmt.Errorf("APPDATA environment variable not set")
		}
		return filepath.Join(appData, "Apple Computer", "MobileSync", "Backup"), nil

	case "darwin":
		home := os.Getenv("HOME")
		if home == "" {
			return "", fmt.Errorf("HOME environment variable not set")
		}
		return filepath.Join(home, "Library", "Application Support", "MobileSync", "Backup"), nil

	case "linux":
		home := os.Getenv("HOME")
		if home == "" {
			return "", fmt.Errorf("HOME environment variable not set")
		}
		return filepath.Join(home, ".config", "mobilesync", "backup"), nil

	default:
		return "", fmt.Errorf("Unsupported operating system detected: %s", osName)
	}
}

func GetPassword() string {
	fmt.Print("Decryption password: ")
	password, err := terminal.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading password: %v\n", err)
		os.Exit(1)
	}
	fmt.Println()
	return string(password)
}
