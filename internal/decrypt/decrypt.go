package decrypt

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"ibex/internal/manifest"
	"ibex/internal/types"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/pbkdf2"
)

// derivePasscodeKey replicates iOS backup passcode derivation from the Python script:
// https://stackoverflow.com/questions/1498342/how-to-decrypt-an-encrypted-apple-itunes-iphone-backup
//
//	passcode1 = PBKDF2-HMAC-SHA256(passcode, DPSL, DPIC, 32 bytes)
//	passcodeKey = PBKDF2-HMAC-SHA1(passcode1, SALT, ITER, 32 bytes)
func DerivePasscodeKey(kb *types.Keybag, passcode []byte) ([]byte, error) {
	dpslRaw, ok := kb.Attributes["DPSL"].([]byte)
	if !ok {
		return nil, fmt.Errorf("missing DPSL in keybag")
	}
	dpicRaw, ok := kb.Attributes["DPIC"].([]byte)
	if !ok {
		return nil, fmt.Errorf("missing DPIC in keybag")
	}
	saltRaw, ok := kb.Attributes["SALT"].([]byte)
	if !ok {
		return nil, fmt.Errorf("missing SALT in keybag")
	}
	iterRaw, ok := kb.Attributes["ITER"].([]byte)
	if !ok {
		return nil, fmt.Errorf("missing ITER in keybag")
	}

	if len(dpslRaw) == 0 || len(dpicRaw) < 4 || len(saltRaw) == 0 || len(iterRaw) < 4 {
		return nil, fmt.Errorf("incomplete passcode attributes in keybag")
	}

	dpicInt := binary.BigEndian.Uint32(dpicRaw)
	iterInt := binary.BigEndian.Uint32(iterRaw)

	// passcode1 = PBKDF2-HMAC-SHA256(passcode, DPSL, DPIC, 32)
	passcode1 := pbkdf2.Key(passcode, dpslRaw, int(dpicInt), 32, sha256.New)

	// passcodeKey = PBKDF2-HMAC-SHA1(passcode1, SALT, ITER, 32)
	passcodeKey := pbkdf2.Key(passcode1, saltRaw, int(iterInt), 32, sha1.New)

	return passcodeKey, nil
}

// unlockKeybagWithPasscode attempts to unwrap each class key that is wrapped with passcode bits.
func UnlockKeybagWithPasscode(kb *types.Keybag, passcodeKey []byte) error {
	// WRAP_PASSCODE = 2 (bitwise)
	for _, ckey := range kb.ClassKeys {
		if (ckey.WRAP & 2) != 0 {
			unwrapped, err := AesUnwrap(passcodeKey, ckey.WPKY)
			if err != nil {
				return fmt.Errorf("failed to unwrap WPKY for class %d: %w", ckey.CLAS, err)
			}
			ckey.KEY = unwrapped
		}
	}
	return nil
}

// unwrapKeyForClass uses the unwrapped class key to unwrap a persistent key (e.g., the ManifestKey).
func UnwrapKeyForClass(kb *types.Keybag, classID uint32, wrappedKey []byte) ([]byte, error) {
	ck, found := kb.ClassKeys[classID]
	if !found {
		return nil, fmt.Errorf("unknown class id %d", classID)
	}
	if ck.KEY == nil {
		return nil, fmt.Errorf("class id %d has no unwrapped KEY", classID)
	}
	if len(wrappedKey) != 0x28 {
		return nil, fmt.Errorf("unexpected wrapped key length (got %d, want 0x28)", len(wrappedKey))
	}
	return AesUnwrap(ck.KEY, wrappedKey)
}

// aesUnwrap implements AES key unwrapping (RFC 3394), matching python's AESUnwrap
func AesUnwrap(kek, wrapped []byte) ([]byte, error) {
	if len(wrapped) < 16 || len(wrapped)%8 != 0 {
		return nil, fmt.Errorf("invalid wrapped key length")
	}
	n := (len(wrapped) / 8) - 1
	A := make([]byte, 8)
	copy(A, wrapped[:8])
	R := make([][]byte, n+1)
	idx := 8
	for i := 1; i <= n; i++ {
		R[i] = make([]byte, 8)
		copy(R[i], wrapped[idx:idx+8])
		idx += 8
	}

	block, err := aes.NewCipher(kek)
	if err != nil {
		return nil, err
	}

	// 6 rounds
	for j := 5; j >= 0; j-- {
		for i := n; i >= 1; i-- {
			t := uint64(n)*uint64(j) + uint64(i)
			temp := Xor64BE(A, t)
			buf := append(temp, R[i]...)
			decrypted := make([]byte, 16)
			block.Decrypt(decrypted, buf)

			copy(A, decrypted[:8])
			copy(R[i], decrypted[8:16])
		}
	}
	// check final A == 0xa6a6a6a6a6a6a6a6
	if !bytes.Equal(A, []byte{0xa6, 0xa6, 0xa6, 0xa6, 0xa6, 0xa6, 0xa6, 0xa6}) {
		return nil, fmt.Errorf("unwrap check failed (A != 0xa6...)")
	}
	// Flatten R[1]..R[n]
	var result []byte
	for i := 1; i <= n; i++ {
		result = append(result, R[i]...)
	}
	return result, nil
}

// xor64BE treats the first 8 bytes of A as big-endian and XORs with t, returning a new 8-byte slice.
func Xor64BE(A []byte, t uint64) []byte {
	if len(A) != 8 {
		return nil
	}
	aVal := binary.BigEndian.Uint64(A)
	x := aVal ^ t
	out := make([]byte, 8)
	binary.BigEndian.PutUint64(out, x)
	return out
}

// aesDecryptCBC does AES-CBC with a zero IV, returning raw (unpadded).
func AesDecryptCBC(data, key []byte) ([]byte, error) {
	if len(data) < aes.BlockSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	remainder := len(data) % aes.BlockSize
	if remainder != 0 {
		data = data[:len(data)-remainder]
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	iv := make([]byte, aes.BlockSize)
	mode := cipher.NewCBCDecrypter(block, iv)
	plaintext := make([]byte, len(data))
	mode.CryptBlocks(plaintext, data)
	return plaintext, nil
}

// decryptFileFromBackup uses the keybag to unwrap the per-file key, and decrypts the actual file bytes.
func GetUnencryptedFileFromBackup(kb *types.Keybag, backupRoot, fileID string, fileSize int) ([]byte, error) {

	fmt.Println("EXTRACTING UNENCRYPTED FILE")

	subdir := strings.ToLower(fileID[:2])

	fullPath := filepath.Join(backupRoot, subdir, fileID)
	unEncData, err := ioutil.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read backup file %s: %w", fileID, err)
	}

	// The file may have padding. Truncate to actual size:
	if len(unEncData) > fileSize {
		unEncData = unEncData[:fileSize]
	}
	return unEncData, nil
}

// decryptFileFromBackup uses the keybag to unwrap the per-file key, and decrypts the actual file bytes.
func DecryptFileFromBackup(kb *types.Keybag, backupRoot, fileID string, protClass int, wrappedKey []byte, fileSize int) ([]byte, error) {
	// We must first unwrap "wrappedKey" using the class's KEY. That means:
	//   1) locate the class key in kb.ClassKeys
	//   2) unwrap if not unwrapped, etc.
	cKey, found := kb.ClassKeys[uint32(protClass)]
	if !found || cKey.KEY == nil {
		return nil, fmt.Errorf("class key for protection class %d not available", protClass)
	}
	if len(wrappedKey) != 0x28 {
		return nil, fmt.Errorf("file has invalid wrapped encryption key length (expected 0x28, got %d)", len(wrappedKey))
	}
	realKey, err := AesUnwrap(cKey.KEY, wrappedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to unwrap file encryption key: %w", err)
	}

	// Next, read the actual file from the backup. Typically, iOS backup organizes by subfolders:
	// fileID => first 2 chars as subdir => file is subdir/fileID
	// e.g. if fileID = "ab12345...", path = "/backupRoot/ab/ab12345..."
	subdir := strings.ToLower(fileID[:2])
	fullPath := filepath.Join(backupRoot, subdir, fileID)
	encData, err := ioutil.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read backup file %s: %w", fileID, err)
	}

	// Decrypt AES-CBC with zero IV
	dec, err := AesDecryptCBC(encData, realKey)
	if err != nil {
		return nil, fmt.Errorf("AES decryption error: %w", err)
	}
	// The file may have padding. Truncate to actual size:
	if len(dec) > fileSize {
		dec = dec[:fileSize]
	}
	return dec, nil
}

// loadAndParseManifest reads Manifest.plist and returns a types.ManifestPlist.
func LoadAndParseManifest(backupPath string) (*types.ManifestPlist, error) {
	plistPath := filepath.Join(backupPath, "Manifest.plist")
	plistBytes, err := os.ReadFile(plistPath)
	if err != nil {
		return nil, fmt.Errorf("could not read Manifest.plist: %v", err)
	}
	mp, err := manifest.ParseManifestPlist(plistBytes)
	if err != nil {
		return nil, fmt.Errorf("parseManifestPlist error: %v", err)
	}
	if mp.BackupKeyBag == nil || mp.ManifestKey == nil {
		return nil, fmt.Errorf("Manifest.plist missing BackupKeyBag or ManifestKey")
	}
	return mp, nil
}

// parseAndUnlockKeybag creates and unlocks the keybag using the provided passcode.
func ParseAndUnlockKeybag(mp *types.ManifestPlist, passcode []byte) (*types.Keybag, error) {
	kb, err := manifest.ParseKeybag(mp.BackupKeyBag)
	if err != nil {
		return nil, fmt.Errorf("failed to parse keybag: %v", err)
	}
	passcodeKey, err := DerivePasscodeKey(kb, passcode)
	if err != nil {
		return nil, fmt.Errorf("derivePasscodeKey failed: %v", err)
	}
	if err = UnlockKeybagWithPasscode(kb, passcodeKey); err != nil {
		return nil, fmt.Errorf("unlockKeybag failed: %v", err)
	}
	return kb, nil
}

// unwrapManifestDBKey extracts the database key from the manifest key.
func UnwrapManifestDBKey(kb *types.Keybag, mp *types.ManifestPlist) ([]byte, error) {
	if len(mp.ManifestKey) < 0x28 {
		return nil, fmt.Errorf("ManifestKey is too short to contain a valid key")
	}
	manifestClass := binary.LittleEndian.Uint32(mp.ManifestKey[:4])
	wrapped := mp.ManifestKey[4:]
	dbKey, err := UnwrapKeyForClass(kb, manifestClass, wrapped)
	if err != nil {
		return nil, fmt.Errorf("failed to unwrap Manifest DB key: %v", err)
	}
	return dbKey, nil
}

// decryptManifestDB decrypts Manifest.db and writes it to disk.
func DecryptManifestDB(backupPath string, dbKey []byte, outDir string) (string, error) {
	encDB, err := ioutil.ReadFile(filepath.Join(backupPath, "Manifest.db"))
	if err != nil {
		return "", fmt.Errorf("could not read Manifest.db: %v", err)
	}
	decDB, err := AesDecryptCBC(encDB, dbKey)
	if err != nil {
		return "", fmt.Errorf("aesDecryptCBC error on Manifest.db: %v", err)
	}
	manifestDBPath := filepath.Join(outDir, "Decrypted_Manifest.db")
	if err := os.WriteFile(manifestDBPath, decDB, 0600); err != nil {
		return "", fmt.Errorf("could not write %s: %v", manifestDBPath, err)
	}
	return manifestDBPath, nil
}
