package cmd

import (
	"errors"
	"fmt"
	"ibex/internal/backup"
	"ibex/internal/decrypt"
	"ibex/internal/util"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/tj/go-spin"
)

var backupPath string
var password string
var outDir string

const bannerArt = `

			██╗██████╗ ███████╗██╗  ██╗
			██║██╔══██╗██╔════╝╚██╗██╔╝
			██║██████╔╝█████╗   ╚███╔╝
			██║██╔══██╗██╔══╝   ██╔██╗
			██║██████╔╝███████╗██╔╝ ██╗
			╚═╝╚═════╝ ╚══════╝╚═╝  ╚═╝

			iOS Backup & Extraction
`

var rootCmd = &cobra.Command{
	Use:   "ibex",
	Short: "iOS backup extraction tool",
	Long:  `ibex is a cross-platform iOS backup decryption and extraction tool.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runBackupExtraction(cmd)
	},
}

// Execute adds all child commands to the root command and executes it.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		slog.Error("Error encountered when running.", "Error", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringP("backup", "b", "", "(optional) iOS backup path")
	rootCmd.Flags().StringP("password", "p", "", "(optional) Backup decryption password")
	rootCmd.Flags().StringP("output", "o", "", "(optional) Output directory path")
	rootCmd.Flags().StringP("file", "f", "", "(optional) Name or substring of a single file to extract and decrypt")
	rootCmd.Flags().BoolP("relative", "r", false, "(optional) Output decrypted files using the original file structure")
}

// getDecryptionPassword returns the decryption password from flag or by prompting.
func getDecryptionPassword(cmd *cobra.Command) (string, error) {
	userPassword, err := cmd.Flags().GetString("password")
	if err != nil {
		return "", err
	}
	if userPassword == "" {
		return util.GetPassword(), nil
	}
	return userPassword, nil
}

// runBackupExtraction orchestrates the full extraction process.
func runBackupExtraction(cmd *cobra.Command) error {

	fmt.Println(util.Pink(bannerArt))

	// 1. Determine backup path.
	userBackupPath, err := cmd.Flags().GetString("backup")
	if err != nil {
		slog.Error("Unable to determine if backup path was passed")
		return err
	}

	if userBackupPath != "" {
		if _, err := os.Stat(userBackupPath); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				slog.Error("[ABORTING] Backup folder does not exist", "Folder", userBackupPath, "Error", err)
				return err
			}
			slog.Error("[ABORTING] Error accessing backup folder", "Folder", userBackupPath, "Error", err)
			return err
		}
		backupPath = userBackupPath

	} else {

		fmt.Println("\nAvailable backups:\n")
		defaultbackupPath, err := backup.GetDefaultBackupPath()
		availableBackups := backup.ListBackups(defaultbackupPath)
		backupPath = backup.SelectBackup(defaultbackupPath, availableBackups)
		if err != nil {
			slog.Error("[ABORTING] Error accessing backup folder", "Folder", backupPath, "Error", err)
			return err
		}
	}

	// 2. Prepare output directory.
	outDir, err := backup.PrepareOutputDirectory(cmd, backupPath)
	if err != nil {
		return err
	}

	// 3. Get decryption password.
	password, err := getDecryptionPassword(cmd)
	if err != nil {
		return err
	}

	// Create output directory.
	if err := os.MkdirAll(outDir, 0750); err != nil {
		slog.Error("Could not create output directory", "Error", err)
		return err
	}

	// Create a spinner for progress display.
	s := spin.New()
	s.Set(spin.Spin4)

	// 4. Load and parse Manifest.plist into a types.ManifestPlist.
	mp, err := decrypt.LoadAndParseManifest(backupPath)
	if err != nil {
		return err
	}

	// 5. Parse and unlock the keybag using the password.
	kb, err := decrypt.ParseAndUnlockKeybag(mp, []byte(password))
	if err != nil {
		return err
	}

	// 6. Unwrap the Manifest.db key and decrypt Manifest.db.
	dbKey, err := decrypt.UnwrapManifestDBKey(kb, mp)
	if err != nil {
		return err
	}
	manifestDBPath, err := decrypt.DecryptManifestDB(backupPath, dbKey, outDir)
	if err != nil {
		return err
	}

	// 7. Check if a target file is specified.
	targetFile, err := cmd.Flags().GetString("file")
	if err != nil {
		return err
	}

	// 7. Check if a target file is specified.
	relative, err := cmd.Flags().GetBool("relative")
	if err != nil {
		return err
	}

	// 8. Process files from the manifest: either extract all or a single specific file.
	if err := backup.ProcessFiles(manifestDBPath, backupPath, kb, outDir, s, targetFile, relative); err != nil {
		return err
	}

	fmt.Printf("\nDecryption completed successfully: %s.\n", outDir)
	return nil
}
