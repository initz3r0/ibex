package manifest

import (
	"bytes"
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"ibex/internal/types"
	"io"

	_ "github.com/mattn/go-sqlite3"
	"github.com/mitchellh/mapstructure"
	"howett.net/plist"
)

// parseKeybag parses a binary BackupKeyBag blob into a Keybag struct.
func ParseKeybag(blob []byte) (*types.Keybag, error) {
	kb := &types.Keybag{
		ClassKeys:  make(map[uint32]*types.ClassKey),
		Attributes: make(map[string]interface{}),
	}
	var currentCK *types.ClassKey
	r := bytes.NewReader(blob)

	for {
		header := make([]byte, 8)
		if _, err := io.ReadFull(r, header); err != nil {
			// normal EOF => end of T/L/V blocks
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("failed to read T/L header: %w", err)
		}

		tag := string(header[:4])
		length := binary.BigEndian.Uint32(header[4:8])
		val := make([]byte, length)
		if _, err := io.ReadFull(r, val); err != nil {
			return nil, fmt.Errorf("failed to read payload for tag=%q: %w", tag, err)
		}

		switch tag {
		// Keybag-level fields
		case "TYPE":
			kb.Type = binary.BigEndian.Uint32(val)
			if kb.Type > 3 {
				return nil, fmt.Errorf("unexpected keybag type: %d", kb.Type)
			}
		case "UUID":
			// If kb.UUID is still nil, this is the keybag's ID. Otherwise, a new class key block
			if kb.UUID == nil {
				kb.UUID = val
			} else {
				if currentCK != nil && currentCK.CLAS != 0 {
					kb.ClassKeys[currentCK.CLAS] = currentCK
				}
				currentCK = &types.ClassKey{UUID: val}
			}
		case "WRAP":
			if currentCK == nil {
				kb.Wrap = binary.BigEndian.Uint32(val)
			} else {
				currentCK.WRAP = binary.BigEndian.Uint32(val)
			}
		case "CLAS":
			if currentCK != nil {
				currentCK.CLAS = binary.BigEndian.Uint32(val)
			}
		case "WPKY":
			if currentCK != nil {
				currentCK.WPKY = val
			}
		case "KTYP":
			if currentCK != nil {
				currentCK.KTYP = binary.BigEndian.Uint32(val)
			}
		case "PBKY":
			// Public key, not essential in passcode-based unwrapping unless needed by iTunes
		default:
			// If not recognized, store in Keybag.Attributes
			kb.Attributes[tag] = val
		}
	}

	// Finalize whichever class key is in progress
	if currentCK != nil && currentCK.CLAS != 0 {
		kb.ClassKeys[currentCK.CLAS] = currentCK
	}

	return kb, nil
}

// parseManifestPlist using github.com/DHowett/go-plist
func ParseManifestPlist(data []byte) (*types.ManifestPlist, error) {
	var mp types.ManifestPlist
	decoder := plist.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&mp); err != nil {
		return nil, err
	}
	return &mp, nil
}

// openManifestDB uses "github.com/mattn/go-sqlite3" to open the decrypted DB from disk or memory.
func OpenManifestDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func DecodeMapToStruct(m map[string]interface{}, out interface{}) error {
	config := &mapstructure.DecoderConfig{
		TagName:          "plist",
		Result:           out,
		WeaklyTypedInput: true,
	}
	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return err
	}
	return decoder.Decode(m)
}

func MapstructureDecoderConfig(result interface{}) *mapstructure.DecoderConfig {
	return &mapstructure.DecoderConfig{
		Metadata: nil,
		Result:   result,
		TagName:  "plist",
		// WeaklyTypedInput ensures we can decode float64 -> int, etc.
		WeaklyTypedInput: true,
	}
}

// TODO: We need to extract the timestamp from the blob as well
func ParseFileBlob(blob []byte) (protectionClass int, encryptionKey []byte, fileSize int, birth int, lastModified int, err error) {
	var archive types.MBFileArchive

	decoder := plist.NewDecoder(bytes.NewReader(blob))
	if decodeErr := decoder.Decode(&archive); decodeErr != nil {
		return 0, nil, 0, 0, 0, fmt.Errorf("plist decode failed: %w", decodeErr)
	}

	// 1) Verify the root UID is in range
	rootIndex := int(archive.Top.Root) // plist.UID is uint64, cast to int
	if rootIndex < 0 || rootIndex >= len(archive.Objects) {
		return 0, nil, 0, 0, 0, fmt.Errorf("root UID out of range")
	}

	// The root object should be the dictionary describing our MBFile
	fileMap, ok := archive.Objects[rootIndex].(map[string]interface{})
	if !ok {
		return 0, nil, 0, 0, 0, fmt.Errorf("root object is not a dictionary")
	}

	// 2) Decode that dictionary into MBFile
	var file types.MBFile
	if err := DecodeMapToStruct(fileMap, &file); err != nil {
		return 0, nil, 0, 0, 0, fmt.Errorf("failed to decode MBFile: %w", err)
	}

	protectionClass = file.ProtectionClass
	fileSize = file.Size

	// 3) If we have EncryptionKey, follow its UID to get the actual bytes
	encIndex := int(file.EncryptionKey)
	if encIndex > 0 && encIndex < len(archive.Objects) {
		keyMap, ok := archive.Objects[encIndex].(map[string]interface{})
		if ok {
			var key types.EncryptionKeyObject
			if err := DecodeMapToStruct(keyMap, &key); err != nil {
				return 0, nil, 0, 0, 0, fmt.Errorf("failed decoding encryption key object: %w", err)
			}
			// Often the first 4 bytes are a “class” tag, then 0x28 for the wrapped key
			// This can vary by iOS version; adapt as necessary.
			if len(key.NSData) >= 4+0x28 {
				encryptionKey = key.NSData[4:]
			} else {
				encryptionKey = key.NSData
			}
		}
	}

	return protectionClass, encryptionKey, fileSize, file.Birth, file.LastModified, nil
}
