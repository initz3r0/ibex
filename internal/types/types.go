package types

import (
	"time"

	"howett.net/plist"
)

type InfoPlist struct {
	BuildVersion     string    `plist:"Build Version"`
	DeviceName       string    `plist:"Device Name"`
	LastBackupDate   time.Time `plist:"Last Backup Date"`
	ProductName      string    `plist:"Product Name"`
	ProductVersion   string    `plist:"Product Version"`
	SerialNumber     string    `plist:"Serial Number"`
	UniqueIdentifier string    `plist:"Unique Identifier"`
}

type BackupsInfoPlists []InfoPlist

// Keybag is a container for keybag metadata, class keys, etc.
type Keybag struct {
	Type       uint32
	UUID       []byte
	Wrap       uint32
	ClassKeys  map[uint32]*ClassKey
	Attributes map[string]interface{} // catch-all for extra keybag tags
}

// ClassKey holds a single class key’s relevant fields (CLAS, WRAP, WPKY, KTYP, etc.).
type ClassKey struct {
	CLAS uint32
	WRAP uint32
	WPKY []byte
	KTYP uint32
	UUID []byte
	KEY  []byte // Unwrapped AES key, if we unwrap it using the passcode key or device key
}

// ManifestPlist models the subset of fields we care about from Manifest.plist
type ManifestPlist struct {
	BackupKeyBag []byte `plist:"BackupKeyBag"`
	ManifestKey  []byte `plist:"ManifestKey"`
}

type MBFileArchive struct {
	Archiver string        `plist:"$archiver,omitempty"`
	Objects  []interface{} `plist:"$objects,omitempty"`
	Top      plistTop      `plist:"$top,omitempty"`
	Version  int           `plist:"$version,omitempty"`
}

type plistTop struct {
	Root plist.UID `plist:"root"`
}

// MBFile represents the dictionary that describes a single file entry.
// Notice that Class, RelativePath, and EncryptionKey all use plist.UID
// instead of a custom struct with “CF$UID” as an int.
type MBFile struct {
	Class            plist.UID `plist:"$class,omitempty"`
	Birth            int       `plist:"Birth,omitempty"`
	Flags            int       `plist:"Flags,omitempty"`
	GroupID          int       `plist:"GroupID,omitempty"`
	InodeNumber      int       `plist:"InodeNumber,omitempty"`
	LastModified     int       `plist:"LastModified,omitempty"`
	LastStatusChange int       `plist:"LastStatusChange,omitempty"`
	Mode             int       `plist:"Mode,omitempty"`

	ProtectionClass int       `plist:"ProtectionClass,omitempty"`
	RelativePath    plist.UID `plist:"RelativePath,omitempty"`
	Size            int       `plist:"Size,omitempty"`
	UserID          int       `plist:"UserID,omitempty"`

	// EncryptionKey is another plist.UID pointer into the $objects array
	// that we’ll follow if present.
	EncryptionKey plist.UID `plist:"EncryptionKey,omitempty"`
}

// encryptionKeyObject is the typical dictionary that holds actual
// key bytes. For example: { "NS.data": <some bytes> }.
type encryptionKeyObject struct {
	NSData []byte `plist:"NS.data,omitempty"`
}

type EncryptionKeyObject struct {
	NSData []byte `plist:"NS.data,omitempty"`
}
