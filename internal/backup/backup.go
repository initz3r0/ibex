package backup

import (
	"bufio"
	"fmt"
	"ibex/internal/decrypt"
	"ibex/internal/manifest"
	"ibex/internal/types"
	"ibex/internal/util"
	"io/ioutil"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tj/go-spin"
	"howett.net/plist"
)

func ListBackups(dir string) []string {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		slog.Error("Error reading directory.", "Error", err)
		os.Exit(1)
	}

	var backups []string
	for _, f := range files {
		if f.IsDir() && len(f.Name()) >= 24 && len(f.Name()) <= 40 {
			backups = append(backups, f.Name())
		}
	}
	return backups
}

// TODO: This is a user interactive function, need to move this out
func SelectBackup(backupDir string, backups []string) string {
	if len(backups) == 0 {
		fmt.Println("No backups found.")
		os.Exit(1)
	}

	for i, backup := range backups {

		backupPath := filepath.Join(backupDir, backup)
		backupDetails := GetBackupsDetails(backupPath)

		var deviceName string
		var deviceProductName string
		var deviceLastBackup time.Time
		var deviceUdid string

		for _, details := range backupDetails {

			deviceName = details.DeviceName
			deviceProductName = details.ProductName
			deviceLastBackup = details.LastBackupDate
			deviceUdid = details.UniqueIdentifier
		}

		fmt.Printf("%d. UDID: %s, Name: %s, Model: %s, Last Backup: %s\n", i+1, deviceUdid, deviceName, deviceProductName, deviceLastBackup.Format("2006-01-02 15:04:05"))
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("\nBackup selection: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		num, err := strconv.Atoi(input)
		if err != nil || num < 1 || num > len(backups) {
			fmt.Println("Invalid selection. Please try again.")
			continue
		}

		return filepath.Join(backupDir, backups[num-1])
	}
}

// Iterate through all backups within the backup directory and parse their Info.plist for details
func GetBackupsDetails(directoryPath string) types.BackupsInfoPlists {

	var backupInfo types.BackupsInfoPlists

	// Walk through the directory and its subdirectories
	err := filepath.Walk(directoryPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Check if the current file is Info.plist
		if info.Name() == "Info.plist" {
			// Open and decode the plist file
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			var data types.InfoPlist
			decoder := plist.NewDecoder(file)
			err = decoder.Decode(&data)
			if err != nil {
				return err
			}

			// Append the parsed plist data to the slice
			backupInfo = append(backupInfo, data)
		}

		return nil
	})

	if err != nil {
		fmt.Println("Error:", err)
		return backupInfo
	}

	return backupInfo
}

// getBackupPath returns the backup path from the flag or by prompting the user.
func GetDefaultBackupPath() (string, error) {

	// No backup flag: use OS-specific default.
	osName := runtime.GOOS
	defaultBackupDir, err := util.GetBackupDirectory(osName)
	if err != nil {
		slog.Error("Unable to get default backup directory path.")
	}
	return defaultBackupDir, nil
}

// prepareOutputDirectory builds a safe output directory based on backup details.
func PrepareOutputDirectory(cmd *cobra.Command, backupPath string) (string, error) {
	replacer := strings.NewReplacer(" ", "_", "’", "_", "'", "")
	layout := "2006-01-02"
	baseName := filepath.Base(backupPath)
	backupDetails := GetBackupsDetails(backupPath)
	var deviceLastBackup time.Time
	var deviceName string
	for _, details := range backupDetails {
		deviceLastBackup = details.LastBackupDate
		deviceName = replacer.Replace(details.DeviceName)
	}
	outputSuffixPath := filepath.Join(baseName, deviceName+"_"+deviceLastBackup.Format(layout))
	userOutput, err := cmd.Flags().GetString("output")
	if err != nil {
		return "", err
	}
	if userOutput != "" {
		return filepath.Join(userOutput, outputSuffixPath), nil
	}
	return filepath.Join(viper.GetString("output_path"), outputSuffixPath), nil
}

// processFiles opens the decrypted manifest DB and decrypts files.
// If targetFile is non-empty, it extracts only the first file whose relative path
// contains the target substring.
func ProcessFiles(manifestDBPath, backupPath string, kb *types.Keybag, outDir string, s *spin.Spinner, targetFile string, relative bool) error {
	db, err := manifest.OpenManifestDB(manifestDBPath)
	if err != nil {
		slog.Error("Error opening Decrypted_Manifest.db", "Error", err)
		return err
	}
	defer db.Close()

	query := `
		SELECT fileID, domain, relativePath, file
		FROM Files
		ORDER BY domain, relativePath
	`
	rows, err := db.Query(query)
	if err != nil {
		slog.Error("Query error on Manifest.db")
		return err
	}
	defer rows.Close()

	var found bool
	for rows.Next() {
		var fileID, domain, relativePath string
		var fileBlob []byte
		if err = rows.Scan(&fileID, &domain, &relativePath, &fileBlob); err != nil {
			slog.Error("Unable to scan row", "Error", err)
			return err
		}

		// If a target file filter was supplied, skip files that do not match.
		if targetFile != "" && !strings.Contains(relativePath, targetFile) {
			continue
		}

		// TODO: Return timestamp from the blob
		protClass, wrappedKey, fileSize, birth, lastModified, err := manifest.ParseFileBlob(fileBlob)
		if err != nil {
			slog.Warn("parseFileBlob failed", "FileID", fileID, "Error", err)
			continue
		}

		// If not encrypted, record the file in skipped.txt.
		// TODO: Let's try to copy the unencrypted file
		if wrappedKey == nil || len(wrappedKey) == 0 {
			skippedPath := filepath.Join(outDir, "skipped.txt")

			// Testing start
			safePath := filepath.Join(outDir, fmt.Sprintf("%s-%s", fileID, filepath.Base(relativePath)))

			if fileSize == 0 {
				continue
			}

			fileData, err := decrypt.GetUnencryptedFileFromBackup(kb, backupPath, fileID, fileSize)
			if err != nil {
				slog.Warn("Unable to decrypt.", "FileID", fileID, "RelativePath", relativePath, "Error", err)
				// If a target file was specified, do not exit – try next. Hence the continue.
				continue
			}

			if err := os.WriteFile(safePath, fileData, 0600); err != nil {
				slog.Error("Error when decrypting the file", "File", safePath, "Error", err)
				continue
			}
			// Testing end

			if err := appendSkippedFile(skippedPath, fileID, domain, relativePath); err != nil {
				slog.Error("Unable to record skipped file to skipped.txt", "Error", err)
			}
			// If we were looking for a specific file, continue to search.
			continue
		}

		fileData, err := decrypt.DecryptFileFromBackup(kb, backupPath, fileID, protClass, wrappedKey, fileSize)
		if err != nil {
			slog.Warn("Unable to decrypt.", "FileID", fileID, "RelativePath", relativePath, "Error", err)
			// If a target file was specified, do not exit – try next. Hence the continue.
			continue
		}

		if relative {

			safePath := filepath.Join(outDir, domain, relativePath)
			dirPath := filepath.Dir(safePath)
			// Create the directory if it doesn't exist

			if err := os.MkdirAll(dirPath, os.ModePerm); err != nil {
				fmt.Println("Error creating directories:", err)
			}

			if err := os.WriteFile(safePath, fileData, 0600); err != nil {
				slog.Error("Error when decrypting the file", "File", safePath, "Error", err)
				continue
			}
			err = os.Chtimes(safePath, time.Unix(int64(lastModified), 0), time.Unix(int64(birth), 0))
			if err != nil {
				log.Fatal(err)
			}

		} else {
			safePath := filepath.Join(outDir, fmt.Sprintf("%s-%s", fileID, filepath.Base(relativePath)))
			if err := os.WriteFile(safePath, fileData, 0600); err != nil {
				slog.Error("Error when decrypting the file", "File", safePath, "Error", err)
				continue
			}
		}

		fmt.Printf("\r \033[36mDecrypting\033[m %s ", s.Next())
		found = true

		// If we were only extracting a specific file, break after the first match.
		if targetFile != "" {
			break
		}
	}
	if err = rows.Err(); err != nil {
		slog.Error("Row iteration error", "Error", err)
		return err
	}

	if targetFile != "" && !found {
		slog.Error("unable to locate target file in backup",
			"targetFile", targetFile)
		return err
		//return fmt.Errorf("unable to locate target file in backup: %s", targetFile)
	}

	return nil
}

// appendSkippedFile writes details of a file that was not decrypted.
func appendSkippedFile(path, fileID, domain, relativePath string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "%s\t%s\t%s\n", fileID, domain, relativePath)
	return err
}
