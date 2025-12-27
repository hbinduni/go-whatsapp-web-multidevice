package constants

// Image MIME types
const (
	MimeJPEG = "image/jpeg"
	MimeJPG  = "image/jpg" // Non-standard but commonly sent by browsers
	MimePNG  = "image/png"
	MimeGIF  = "image/gif"
	MimeWebP = "image/webp"
)

// Video MIME types
const (
	MimeMP4      = "video/mp4"
	MimeMKV      = "video/x-matroska"
	MimeAVI      = "video/avi"
	MimeMSVideo  = "video/x-msvideo"
	MimeWebM     = "video/webm"
	MimeOggVideo = "video/ogg"
)

// Audio MIME types
const (
	MimeAAC     = "audio/aac"
	MimeAMR     = "audio/amr"
	MimeFLAC    = "audio/flac"
	MimeM4A     = "audio/m4a"
	MimeM4R     = "audio/m4r"
	MimeMP3     = "audio/mp3"
	MimeMPEG    = "audio/mpeg"
	MimeOgg     = "audio/ogg"
	MimeWMA     = "audio/wma"
	MimeWMALong = "audio/x-ms-wma"
	MimeWAV     = "audio/wav"
	MimeVndWAV  = "audio/vnd.wav"
	MimeVndWave = "audio/vnd.wave"
	MimeWave    = "audio/wave"
	MimePnWAV   = "audio/x-pn-wav"
	MimeXWAV    = "audio/x-wav"
)

// Document MIME types
const (
	MimePDF = "application/pdf"
)

// AllowedImageTypes returns a map of valid image MIME types for image uploads
func AllowedImageTypes() map[string]bool {
	return map[string]bool{
		MimeJPEG: true,
		MimeJPG:  true, // Non-standard alias for JPEG
		MimePNG:  true,
	}
}

// AllowedImageTypesWithGIF returns allowed image types including GIF
func AllowedImageTypesWithGIF() map[string]bool {
	return map[string]bool{
		MimeJPEG: true,
		MimeJPG:  true, // Non-standard alias for JPEG
		MimePNG:  true,
		MimeGIF:  true,
	}
}

// AllowedGroupPhotoTypes returns valid MIME types for group photos
func AllowedGroupPhotoTypes() map[string]bool {
	return map[string]bool{
		MimeJPEG: true,
		MimeJPG:  true, // Non-standard alias for JPEG
		MimePNG:  true,
		MimeGIF:  true,
		MimeWebP: true,
	}
}

// AllowedStickerTypes returns valid MIME types for stickers
func AllowedStickerTypes() map[string]bool {
	return map[string]bool{
		MimeJPEG: true,
		MimeJPG:  true, // Non-standard alias for JPEG
		MimePNG:  true,
		MimeWebP: true,
		MimeGIF:  true,
	}
}

// AllowedVideoTypes returns valid MIME types for video uploads
func AllowedVideoTypes() map[string]bool {
	return map[string]bool{
		MimeMP4:     true,
		MimeMKV:     true,
		MimeAVI:     true,
		MimeMSVideo: true,
	}
}

// AllowedAudioTypes returns valid MIME types for audio uploads
func AllowedAudioTypes() map[string]bool {
	return map[string]bool{
		MimeAAC:     true,
		MimeAMR:     true,
		MimeFLAC:    true,
		MimeM4A:     true,
		MimeM4R:     true,
		MimeMP3:     true,
		MimeMPEG:    true,
		MimeOgg:     true,
		MimeWMA:     true,
		MimeWMALong: true,
		MimeWAV:     true,
		MimeVndWAV:  true,
		MimeVndWave: true,
		MimeWave:    true,
		MimePnWAV:   true,
		MimeXWAV:    true,
	}
}

// IsAllowedImageType checks if the MIME type is a valid image type
func IsAllowedImageType(mimeType string) bool {
	return AllowedImageTypes()[mimeType]
}

// IsAllowedVideoType checks if the MIME type is a valid video type
func IsAllowedVideoType(mimeType string) bool {
	return AllowedVideoTypes()[mimeType]
}

// IsAllowedAudioType checks if the MIME type is a valid audio type
func IsAllowedAudioType(mimeType string) bool {
	return AllowedAudioTypes()[mimeType]
}

// IsAllowedStickerType checks if the MIME type is a valid sticker type
func IsAllowedStickerType(mimeType string) bool {
	return AllowedStickerTypes()[mimeType]
}

// IsAllowedGroupPhotoType checks if the MIME type is valid for group photos
func IsAllowedGroupPhotoType(mimeType string) bool {
	return AllowedGroupPhotoTypes()[mimeType]
}

// AllowedAudioTypesString returns a comma-separated string of allowed audio MIME types
func AllowedAudioTypesString() string {
	types := []string{
		MimeAAC, MimeAMR, MimeFLAC, MimeM4A, MimeM4R, MimeMP3, MimeMPEG,
		MimeOgg, MimeVndWAV, MimeVndWave, MimeWAV, MimeWave, MimeWMA,
		MimeWMALong, MimePnWAV, MimeXWAV,
	}
	result := ""
	for _, t := range types {
		result += t + ","
	}
	return result
}
