package storage

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	"github.com/sirupsen/logrus"
)

// pathSegmentSanitizer matches any character that is NOT alphanumeric, underscore, or hyphen
// Used to sanitize path segments for consistent URL construction
var pathSegmentSanitizer = regexp.MustCompile(`[^A-Za-z0-9_\-]`)

var (
	globalStorage MediaStorage
	storageMu     sync.RWMutex
)

// InitStorage initializes the global storage instance based on configuration
func InitStorage(storageType StorageType, localBasePath string, s3Config *S3Config) error {
	var storage MediaStorage
	var err error

	switch storageType {
	case StorageTypeLocal:
		storage, err = NewLocalStorage(localBasePath)
		if err != nil {
			return fmt.Errorf("failed to initialize local storage: %w", err)
		}
		logrus.Debug("Initialized local file storage")

	case StorageTypeS3:
		if s3Config == nil {
			return fmt.Errorf("S3 configuration is required for S3 storage type")
		}

		// Use MinIO native SDK for better compatibility
		storage, err = NewMinIOStorage(*s3Config)
		if err != nil {
			return fmt.Errorf("failed to initialize MinIO storage: %w", err)
		}

		logrus.Debugf("Initialized S3-compatible storage (endpoint: %s, bucket: %s)", s3Config.Endpoint, s3Config.Bucket)

	default:
		return fmt.Errorf("unsupported storage type: %s", storageType)
	}

	storageMu.Lock()
	globalStorage = storage
	storageMu.Unlock()
	return nil
}

// GetStorage returns the global storage instance
func GetStorage() MediaStorage {
	storageMu.RLock()
	s := globalStorage
	storageMu.RUnlock()
	if s != nil {
		return s
	}
	logrus.Warn("Storage not initialized, using default local storage")
	fallback, err := NewLocalStorage("statics/media")
	if err != nil {
		logrus.Errorf("Failed to create fallback local storage: %v", err)
		return nil
	}
	storageMu.Lock()
	if globalStorage == nil {
		globalStorage = fallback
	}
	s = globalStorage
	storageMu.Unlock()
	return s
}

// ParseStorageType converts a string to StorageType
func ParseStorageType(s string) (StorageType, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "local", "":
		return StorageTypeLocal, nil
	case "s3", "minio":
		return StorageTypeS3, nil
	default:
		return "", fmt.Errorf("invalid storage type: %s (valid options: local, s3)", s)
	}
}

// ConstructMediaURL constructs a direct media URL for S3 storage
// This allows clients to access media directly without calling the download endpoint
// Returns empty string if storage type is not S3 or if required parameters are missing
//
// Parameters:
//   - deviceID: The WhatsApp device ID (phone number)
//   - chatJID: The chat JID (e.g., "6281911770011@s.whatsapp.net")
//   - messageID: The message ID
//   - mediaType: The media type (unused, kept for API compatibility)
//
// URL Pattern: {S3PublicURL or S3Endpoint}/{S3Bucket}/{deviceID}/{chatJID_sanitized}/{messageID}
// Note: No file extension - S3 handles content-type via metadata
func ConstructMediaURL(deviceID, chatJID, messageID, mediaType string) string {
	// Only construct URL for S3 storage
	if config.MediaStorageType != "s3" {
		return ""
	}

	// Validate required parameters
	if deviceID == "" || chatJID == "" || messageID == "" {
		return ""
	}

	// Sanitize path segments (same logic as used when saving media)
	dev := pathSegmentSanitizer.ReplaceAllString(deviceID, "_")
	jid := pathSegmentSanitizer.ReplaceAllString(chatJID, "_")
	msg := pathSegmentSanitizer.ReplaceAllString(messageID, "_")

	if dev == "" || jid == "" || msg == "" {
		return ""
	}

	// Construct path: deviceID/chatJID/messageID (no extension)
	path := fmt.Sprintf("%s/%s/%s", dev, jid, msg)

	// Determine base URL (prefer PublicURL, fallback to Endpoint)
	baseURL := config.S3PublicURL
	if baseURL == "" {
		baseURL = config.S3Endpoint
	}

	// Remove trailing slash from base URL
	baseURL = strings.TrimRight(baseURL, "/")

	// Construct full URL: {baseURL}/{bucket}/{path}
	return fmt.Sprintf("%s/%s/%s", baseURL, config.S3Bucket, path)
}

// IsS3StorageEnabled returns true if S3 storage is configured and enabled
func IsS3StorageEnabled() bool {
	return config.MediaStorageType == "s3" && config.S3Bucket != "" && config.S3Endpoint != ""
}
