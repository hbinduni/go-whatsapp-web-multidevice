package whatsapp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	"github.com/sirupsen/logrus"
)

// CleanupTemporaryFiles removes history files, QR images, and send items
func CleanupTemporaryFiles() error {
	var cleanupErrors []error

	// Clean up history files
	if files, err := filepath.Glob(fmt.Sprintf("./%s/history-*", config.PathStorages)); err == nil {
		for _, f := range files {
			if err := os.Remove(f); err != nil && !os.IsNotExist(err) {
				logrus.Warnf("[Cleanup] Failed to remove history file %s: %v", f, err)
				cleanupErrors = append(cleanupErrors, err)
			}
		}
	}

	// Clean up QR images
	if qrImages, err := filepath.Glob(fmt.Sprintf("./%s/scan-*", config.PathQrCode)); err == nil {
		for _, f := range qrImages {
			if err := os.Remove(f); err != nil && !os.IsNotExist(err) {
				logrus.Warnf("[Cleanup] Failed to remove QR image %s: %v", f, err)
				cleanupErrors = append(cleanupErrors, err)
			}
		}
	}

	// Clean up send items
	if sendItems, err := filepath.Glob(fmt.Sprintf("./%s/*", config.PathSendItems)); err == nil {
		for _, f := range sendItems {
			if !strings.Contains(f, ".gitignore") {
				if err := os.Remove(f); err != nil && !os.IsNotExist(err) {
					logrus.Warnf("[Cleanup] Failed to remove send item %s: %v", f, err)
					cleanupErrors = append(cleanupErrors, err)
				}
			}
		}
	}

	if len(cleanupErrors) > 0 {
		logrus.Warnf("[Cleanup] Completed with %d errors", len(cleanupErrors))
	}

	return nil
}
