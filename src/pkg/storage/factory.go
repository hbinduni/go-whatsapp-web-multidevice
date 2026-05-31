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

// InitStorage initializes the global S3 storage instance
func InitStorage(s3Config *S3Config) error {
	if s3Config == nil {
		return fmt.Errorf("S3 configuration is required")
	}

	storage, err := NewMinIOStorage(*s3Config)
	if err != nil {
		return fmt.Errorf("failed to initialize S3 storage: %w", err)
	}

	logrus.Infof("Initialized S3 storage (endpoint: %s, bucket: %s)", s3Config.Endpoint, s3Config.Bucket)

	storageMu.Lock()
	globalStorage = storage
	storageMu.Unlock()
	return nil
}

// GetStorage returns the global storage instance
// Returns nil if storage is not initialized - caller must handle this
func GetStorage() MediaStorage {
	storageMu.RLock()
	defer storageMu.RUnlock()
	return globalStorage
}

// IsStorageInitialized returns true if storage has been initialized
func IsStorageInitialized() bool {
	storageMu.RLock()
	defer storageMu.RUnlock()
	return globalStorage != nil
}

// BuildMediaObjectKey builds the S3 object key for a media message.
// It is the single source of truth for the storage path so that uploads
// (sent and received media) and ConstructMediaURL always agree.
//
// Key Pattern: {deviceID}/{chatJID_sanitized}/{messageID} (no extension -
// S3 handles content-type via metadata). Returns "" if any segment is empty
// after sanitization.
func BuildMediaObjectKey(deviceID, chatJID, messageID string) string {
	if deviceID == "" || chatJID == "" || messageID == "" {
		return ""
	}

	dev := pathSegmentSanitizer.ReplaceAllString(deviceID, "_")
	jid := pathSegmentSanitizer.ReplaceAllString(chatJID, "_")
	msg := pathSegmentSanitizer.ReplaceAllString(messageID, "_")

	if dev == "" || jid == "" || msg == "" {
		return ""
	}

	return fmt.Sprintf("%s/%s/%s", dev, jid, msg)
}

// ConstructMediaURL constructs a direct media URL for S3 storage
// This allows clients to access media directly without calling the download endpoint
// Returns empty string if required parameters are missing
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
	// Validate S3 configuration
	if config.S3Bucket == "" || config.S3Endpoint == "" {
		return ""
	}

	// Build the (sanitized) object key; empty if any segment is missing
	path := BuildMediaObjectKey(deviceID, chatJID, messageID)
	if path == "" {
		return ""
	}

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

// IsS3ConfigValid returns true if S3 storage is properly configured
func IsS3ConfigValid() bool {
	return config.S3Bucket != "" && config.S3Endpoint != "" &&
		config.S3AccessKeyID != "" && config.S3SecretAccessKey != ""
}
