package usecase

import (
	"bytes"
	"context"
	"fmt"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	domainSend "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/send"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/infrastructure/whatsapp"
	pkgError "github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/error"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/utils"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/ui/rest/helpers"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/validations"
	"github.com/disintegration/imaging"
	fiberUtils "github.com/gofiber/fiber/v2/utils"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

func (service serviceSend) SendImage(ctx context.Context, request domainSend.ImageRequest) (response domainSend.GenericResponse, err error) {
	err = validations.ValidateSendImage(ctx, request)
	if err != nil {
		return response, err
	}
	dataWaRecipient, err := utils.ValidateJidWithLogin(whatsapp.GetClient(), request.Phone)
	if err != nil {
		return response, err
	}

	var (
		imagePath      string
		imageThumbnail string
		imageName      string
		deletedItems   []string
		oriImagePath   string
	)

	if request.ImageURL != nil && *request.ImageURL != "" {
		imageData, fileName, err := utils.DownloadImageFromURL(*request.ImageURL)
		if err != nil {
			return response, pkgError.InternalServerError(fmt.Sprintf("failed to download image from URL %v", err))
		}

		mimeType := http.DetectContentType(imageData)
		if mimeType == "image/webp" {
			webpImage, err := imaging.Decode(bytes.NewReader(imageData))
			if err != nil {
				return response, pkgError.InternalServerError(fmt.Sprintf("failed to decode WebP image %v", err))
			}

			if strings.HasSuffix(strings.ToLower(fileName), ".webp") {
				fileName = fileName[:len(fileName)-5] + ".png"
			} else {
				fileName = fileName + ".png"
			}

			var pngBuffer bytes.Buffer
			err = imaging.Encode(&pngBuffer, webpImage, imaging.PNG)
			if err != nil {
				return response, pkgError.InternalServerError(fmt.Sprintf("failed to convert WebP to PNG %v", err))
			}
			imageData = pngBuffer.Bytes()
		}

		oriImagePath = fmt.Sprintf("%s/%s", config.PathSendItems, fileName)
		imageName = fileName
		err = os.WriteFile(oriImagePath, imageData, 0644)
		if err != nil {
			return response, pkgError.InternalServerError(fmt.Sprintf("failed to save downloaded image %v", err))
		}
	} else if request.Image != nil {
		oriImagePath = fmt.Sprintf("%s/%s", config.PathSendItems, request.Image.Filename)
		err = fasthttp.SaveMultipartFile(request.Image, oriImagePath)
		if err != nil {
			return response, err
		}
		imageName = request.Image.Filename
	}
	deletedItems = append(deletedItems, oriImagePath)

	// Decode image once and reuse for both thumbnail and compression
	srcImage, err := imaging.Open(oriImagePath)
	if err != nil {
		return response, pkgError.InternalServerError(fmt.Sprintf("Failed to open image file '%s': %v", oriImagePath, err))
	}

	// Generate thumbnail from cached decoded image
	resizedImage := imaging.Resize(srcImage, 100, 0, imaging.Lanczos)
	imageThumbnail = fmt.Sprintf("%s/thumbnails-%s", config.PathSendItems, imageName)
	if err = imaging.Save(resizedImage, imageThumbnail); err != nil {
		return response, pkgError.InternalServerError(fmt.Sprintf("failed to save thumbnail %v", err))
	}
	deletedItems = append(deletedItems, imageThumbnail)

	if request.Compress {
		// Reuse already decoded srcImage instead of opening file again
		newImage := imaging.Resize(srcImage, 600, 0, imaging.Lanczos)
		newImagePath := fmt.Sprintf("%s/new-%s", config.PathSendItems, imageName)
		if err = imaging.Save(newImage, newImagePath); err != nil {
			return response, pkgError.InternalServerError(fmt.Sprintf("failed to save image %v", err))
		}
		deletedItems = append(deletedItems, newImagePath)
		imagePath = newImagePath
	} else {
		imagePath = oriImagePath
	}

	dataWaCaption := request.Caption
	dataWaImage, err := os.ReadFile(imagePath)
	if err != nil {
		return response, err
	}
	uploadedImage, err := service.uploadMedia(ctx, whatsmeow.MediaImage, dataWaImage, dataWaRecipient)
	if err != nil {
		logrus.Errorf("failed to upload image: %v", err)
		return response, err
	}
	dataWaThumbnail, err := os.ReadFile(imageThumbnail)
	if err != nil {
		return response, pkgError.InternalServerError(fmt.Sprintf("failed to read thumbnail %v", err))
	}

	msg := &waE2E.Message{ImageMessage: &waE2E.ImageMessage{
		JPEGThumbnail: dataWaThumbnail,
		Caption:       proto.String(dataWaCaption),
		URL:           proto.String(uploadedImage.URL),
		DirectPath:    proto.String(uploadedImage.DirectPath),
		MediaKey:      uploadedImage.MediaKey,
		Mimetype:      proto.String(http.DetectContentType(dataWaImage)),
		FileEncSHA256: uploadedImage.FileEncSHA256,
		FileSHA256:    uploadedImage.FileSHA256,
		FileLength:    proto.Uint64(uint64(len(dataWaImage))),
		ViewOnce:      proto.Bool(request.ViewOnce),
	}}

	if request.BaseRequest.IsForwarded {
		msg.ImageMessage.ContextInfo = &waE2E.ContextInfo{
			IsForwarded:     proto.Bool(true),
			ForwardingScore: proto.Uint32(100),
		}
	}

	if request.BaseRequest.Duration != nil && *request.BaseRequest.Duration > 0 {
		if msg.ImageMessage.ContextInfo == nil {
			msg.ImageMessage.ContextInfo = &waE2E.ContextInfo{}
		}
		msg.ImageMessage.ContextInfo.Expiration = proto.Uint32(uint32(*request.BaseRequest.Duration))
	}

	caption := "🖼️ Image"
	if request.Caption != "" {
		caption = "🖼️ " + request.Caption
	}
	ts, err := service.wrapSendMessage(ctx, dataWaRecipient, msg, caption)
	go func() {
		if errDelete := utils.RemoveFile(0, deletedItems...); errDelete != nil {
			logrus.Warnf("error deleting temporary image files: %v", errDelete)
		}
	}()
	if err != nil {
		return response, err
	}

	response.MessageID = ts.ID
	response.Status = fmt.Sprintf("Message sent to %s (server timestamp: %s)", request.BaseRequest.Phone, ts.Timestamp.String())
	return response, nil
}

func (service serviceSend) SendFile(ctx context.Context, request domainSend.FileRequest) (response domainSend.GenericResponse, err error) {
	err = validations.ValidateSendFile(ctx, request)
	if err != nil {
		return response, err
	}
	dataWaRecipient, err := utils.ValidateJidWithLogin(whatsapp.GetClient(), request.BaseRequest.Phone)
	if err != nil {
		return response, err
	}

	fileBytes := helpers.MultipartFormFileHeaderToBytes(request.File)
	fileMimeType := resolveDocumentMIME(request.File.Filename, fileBytes)

	uploadedFile, err := service.uploadMedia(ctx, whatsmeow.MediaDocument, fileBytes, dataWaRecipient)
	if err != nil {
		logrus.Errorf("failed to upload document: %v", err)
		return response, err
	}

	msg := &waE2E.Message{DocumentMessage: &waE2E.DocumentMessage{
		URL:           proto.String(uploadedFile.URL),
		Mimetype:      proto.String(fileMimeType),
		Title:         proto.String(request.File.Filename),
		FileSHA256:    uploadedFile.FileSHA256,
		FileLength:    proto.Uint64(uploadedFile.FileLength),
		MediaKey:      uploadedFile.MediaKey,
		FileName:      proto.String(request.File.Filename),
		FileEncSHA256: uploadedFile.FileEncSHA256,
		DirectPath:    proto.String(uploadedFile.DirectPath),
		Caption:       proto.String(request.Caption),
	}}

	if request.BaseRequest.IsForwarded {
		msg.DocumentMessage.ContextInfo = &waE2E.ContextInfo{
			IsForwarded:     proto.Bool(true),
			ForwardingScore: proto.Uint32(100),
		}
	}

	if request.BaseRequest.Duration != nil && *request.BaseRequest.Duration > 0 {
		if msg.DocumentMessage.ContextInfo == nil {
			msg.DocumentMessage.ContextInfo = &waE2E.ContextInfo{}
		}
		msg.DocumentMessage.ContextInfo.Expiration = proto.Uint32(uint32(*request.BaseRequest.Duration))
	}

	caption := "📄 Document"
	if request.Caption != "" {
		caption = "📄 " + request.Caption
	}
	ts, err := service.wrapSendMessage(ctx, dataWaRecipient, msg, caption)
	if err != nil {
		return response, err
	}

	response.MessageID = ts.ID
	response.Status = fmt.Sprintf("Document sent to %s (server timestamp: %s)", request.BaseRequest.Phone, ts.Timestamp.String())
	return response, nil
}

func resolveDocumentMIME(filename string, fileBytes []byte) string {
	extension := strings.ToLower(filepath.Ext(filename))
	if extension != "" {
		if mimeType, ok := utils.KnownDocumentMIMEByExtension(extension); ok {
			return mimeType
		}

		if mimeType := mime.TypeByExtension(extension); mimeType != "" {
			return mimeType
		}
	}

	return http.DetectContentType(fileBytes)
}

func (service serviceSend) SendVideo(ctx context.Context, request domainSend.VideoRequest) (response domainSend.GenericResponse, err error) {
	err = validations.ValidateSendVideo(ctx, request)
	if err != nil {
		return response, err
	}
	dataWaRecipient, err := utils.ValidateJidWithLogin(whatsapp.GetClient(), request.BaseRequest.Phone)
	if err != nil {
		return response, err
	}

	var (
		videoPath      string
		videoThumbnail string
		deletedItems   []string
	)

	defer func() {
		if len(deletedItems) > 0 {
			go func() {
				_ = utils.RemoveFile(1, deletedItems...)
			}()
		}
	}()

	generateUUID := fiberUtils.UUIDv4()
	var oriVideoPath string

	if request.VideoURL != nil && *request.VideoURL != "" {
		videoBytes, fileName, errDownload := utils.DownloadVideoFromURL(*request.VideoURL)
		if errDownload != nil {
			return response, pkgError.InternalServerError(fmt.Sprintf("failed to download video from URL %v", errDownload))
		}
		oriVideoPath = fmt.Sprintf("%s/%s", config.PathSendItems, generateUUID+fileName)
		if errWrite := os.WriteFile(oriVideoPath, videoBytes, 0644); errWrite != nil {
			return response, pkgError.InternalServerError(fmt.Sprintf("failed to store downloaded video in server %v", errWrite))
		}
	} else if request.Video != nil {
		oriVideoPath = fmt.Sprintf("%s/%s", config.PathSendItems, generateUUID+request.Video.Filename)
		err = fasthttp.SaveMultipartFile(request.Video, oriVideoPath)
		if err != nil {
			return response, pkgError.InternalServerError(fmt.Sprintf("failed to store video in server %v", err))
		}
	} else {
		return response, pkgError.ValidationError("either Video or VideoURL must be provided")
	}

	_, err = exec.LookPath("ffmpeg")
	if err != nil {
		return response, pkgError.InternalServerError("ffmpeg not installed")
	}

	thumbnailVideoPath := fmt.Sprintf("%s/%s", config.PathSendItems, generateUUID+".png")
	cmdThumbnail := exec.Command("ffmpeg", "-i", oriVideoPath, "-ss", "00:00:01.000", "-vframes", "1", thumbnailVideoPath)
	err = cmdThumbnail.Run()
	if err != nil {
		return response, pkgError.InternalServerError(fmt.Sprintf("failed to create thumbnail %v", err))
	}

	srcImage, err := imaging.Open(thumbnailVideoPath)
	if err != nil {
		return response, pkgError.InternalServerError(fmt.Sprintf("Failed to open generated video thumbnail image '%s': %v", thumbnailVideoPath, err))
	}
	resizedImage := imaging.Resize(srcImage, 100, 0, imaging.Lanczos)
	thumbnailResizeVideoPath := fmt.Sprintf("%s/thumbnails-%s", config.PathSendItems, generateUUID+".png")
	if err = imaging.Save(resizedImage, thumbnailResizeVideoPath); err != nil {
		return response, pkgError.InternalServerError(fmt.Sprintf("failed to save thumbnail %v", err))
	}

	deletedItems = append(deletedItems, thumbnailVideoPath)
	deletedItems = append(deletedItems, thumbnailResizeVideoPath)
	videoThumbnail = thumbnailResizeVideoPath

	if request.Compress {
		compresVideoPath := fmt.Sprintf("%s/%s", config.PathSendItems, generateUUID+".mp4")
		cmdCompress := exec.Command("ffmpeg", "-i", oriVideoPath,
			"-c:v", "libx264",
			"-crf", "28",
			"-preset", "fast",
			"-vf", "scale=720:-2",
			"-c:a", "aac",
			"-b:a", "128k",
			"-movflags", "+faststart",
			"-y",
			compresVideoPath)

		output, err := cmdCompress.CombinedOutput()
		if err != nil {
			logrus.Errorf("ffmpeg compression failed: %v, output: %s", err, string(output))
			return response, pkgError.InternalServerError(fmt.Sprintf("failed to compress video: %v", err))
		}

		videoPath = compresVideoPath
		deletedItems = append(deletedItems, compresVideoPath)
	} else {
		videoPath = oriVideoPath
	}
	deletedItems = append(deletedItems, oriVideoPath)

	dataWaVideo, err := os.ReadFile(videoPath)
	if err != nil {
		return response, err
	}
	uploaded, err := service.uploadMedia(ctx, whatsmeow.MediaVideo, dataWaVideo, dataWaRecipient)
	if err != nil {
		return response, pkgError.InternalServerError(fmt.Sprintf("Failed to upload file: %v", err))
	}
	dataWaThumbnail, err := os.ReadFile(videoThumbnail)
	if err != nil {
		return response, err
	}

	msg := &waE2E.Message{VideoMessage: &waE2E.VideoMessage{
		URL:                 proto.String(uploaded.URL),
		Mimetype:            proto.String(http.DetectContentType(dataWaVideo)),
		Caption:             proto.String(request.Caption),
		FileLength:          proto.Uint64(uploaded.FileLength),
		FileSHA256:          uploaded.FileSHA256,
		FileEncSHA256:       uploaded.FileEncSHA256,
		MediaKey:            uploaded.MediaKey,
		DirectPath:          proto.String(uploaded.DirectPath),
		ViewOnce:            proto.Bool(request.ViewOnce),
		JPEGThumbnail:       dataWaThumbnail,
		ThumbnailEncSHA256:  dataWaThumbnail,
		ThumbnailSHA256:     dataWaThumbnail,
		ThumbnailDirectPath: proto.String(uploaded.DirectPath),
	}}

	if request.BaseRequest.IsForwarded {
		msg.VideoMessage.ContextInfo = &waE2E.ContextInfo{
			IsForwarded:     proto.Bool(true),
			ForwardingScore: proto.Uint32(100),
		}
	}

	if request.BaseRequest.Duration != nil && *request.BaseRequest.Duration > 0 {
		if msg.VideoMessage.ContextInfo == nil {
			msg.VideoMessage.ContextInfo = &waE2E.ContextInfo{}
		}
		msg.VideoMessage.ContextInfo.Expiration = proto.Uint32(uint32(*request.BaseRequest.Duration))
	}

	caption := "🎥 Video"
	if request.Caption != "" {
		caption = "🎥 " + request.Caption
	}
	ts, err := service.wrapSendMessage(ctx, dataWaRecipient, msg, caption)
	if err != nil {
		return response, err
	}

	response.MessageID = ts.ID
	response.Status = fmt.Sprintf("Video sent to %s (server timestamp: %s)", request.BaseRequest.Phone, ts.Timestamp.String())
	return response, nil
}

func (service serviceSend) SendAudio(ctx context.Context, request domainSend.AudioRequest) (response domainSend.GenericResponse, err error) {
	err = validations.ValidateSendAudio(ctx, request)
	if err != nil {
		return response, err
	}

	dataWaRecipient, err := utils.ValidateJidWithLogin(whatsapp.GetClient(), request.BaseRequest.Phone)
	if err != nil {
		return response, err
	}

	var (
		audioBytes    []byte
		audioMimeType string
	)

	if request.AudioURL != nil && *request.AudioURL != "" {
		audioBytes, _, err = utils.DownloadAudioFromURL(*request.AudioURL)
		if err != nil {
			return response, pkgError.InternalServerError(fmt.Sprintf("failed to download audio from URL %v", err))
		}
		audioMimeType = http.DetectContentType(audioBytes)
	} else if request.Audio != nil {
		audioBytes = helpers.MultipartFormFileHeaderToBytes(request.Audio)
		audioMimeType = http.DetectContentType(audioBytes)
	}

	audioUploaded, err := service.uploadMedia(ctx, whatsmeow.MediaAudio, audioBytes, dataWaRecipient)
	if err != nil {
		err = pkgError.WaUploadMediaError(fmt.Sprintf("Failed to upload audio: %v", err))
		return response, err
	}

	msg := &waE2E.Message{
		AudioMessage: &waE2E.AudioMessage{
			URL:           proto.String(audioUploaded.URL),
			DirectPath:    proto.String(audioUploaded.DirectPath),
			Mimetype:      proto.String(audioMimeType),
			FileLength:    proto.Uint64(audioUploaded.FileLength),
			FileSHA256:    audioUploaded.FileSHA256,
			FileEncSHA256: audioUploaded.FileEncSHA256,
			MediaKey:      audioUploaded.MediaKey,
		},
	}

	if request.BaseRequest.IsForwarded {
		msg.AudioMessage.ContextInfo = &waE2E.ContextInfo{
			IsForwarded:     proto.Bool(true),
			ForwardingScore: proto.Uint32(100),
		}
	}

	if request.BaseRequest.Duration != nil && *request.BaseRequest.Duration > 0 {
		if msg.AudioMessage.ContextInfo == nil {
			msg.AudioMessage.ContextInfo = &waE2E.ContextInfo{}
		}
		msg.AudioMessage.ContextInfo.Expiration = proto.Uint32(uint32(*request.BaseRequest.Duration))
	}

	content := "🎵 Audio"

	ts, err := service.wrapSendMessage(ctx, dataWaRecipient, msg, content)
	if err != nil {
		return response, err
	}

	response.MessageID = ts.ID
	response.Status = fmt.Sprintf("Send audio success %s (server timestamp: %s)", request.BaseRequest.Phone, ts.Timestamp.String())
	return response, nil
}

func (service serviceSend) SendSticker(ctx context.Context, request domainSend.StickerRequest) (response domainSend.GenericResponse, err error) {
	err = validations.ValidateSendSticker(ctx, request)
	if err != nil {
		return response, err
	}

	dataWaRecipient, err := utils.ValidateJidWithLogin(whatsapp.GetClient(), request.Phone)
	if err != nil {
		return response, err
	}

	var (
		stickerPath  string
		deletedItems []string
		stickerBytes []byte
	)

	absBaseDir, err := filepath.Abs(config.PathSendItems)
	if err != nil {
		return response, pkgError.InternalServerError(fmt.Sprintf("failed to resolve base directory: %v", err))
	}

	defer func() {
		for _, path := range deletedItems {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				logrus.Warnf("Failed to cleanup temporary file %s: %v", path, err)
			}
		}
	}()

	if request.StickerURL != nil && *request.StickerURL != "" {
		imageData, _, err := utils.DownloadImageFromURL(*request.StickerURL)
		if err != nil {
			return response, pkgError.InternalServerError(fmt.Sprintf("failed to download sticker from URL: %v", err))
		}

		f, err := os.CreateTemp(absBaseDir, "sticker_*")
		if err != nil {
			return response, pkgError.InternalServerError(fmt.Sprintf("failed to create temp file: %v", err))
		}
		stickerPath = f.Name()
		if _, err := f.Write(imageData); err != nil {
			f.Close()
			return response, pkgError.InternalServerError(fmt.Sprintf("failed to write sticker: %v", err))
		}
		_ = f.Close()
		deletedItems = append(deletedItems, stickerPath)
	} else if request.Sticker != nil {
		f, err := os.CreateTemp(absBaseDir, "sticker_*")
		if err != nil {
			return response, pkgError.InternalServerError(fmt.Sprintf("failed to create temp file: %v", err))
		}
		stickerPath = f.Name()
		_ = f.Close()

		err = fasthttp.SaveMultipartFile(request.Sticker, stickerPath)
		if err != nil {
			return response, pkgError.InternalServerError(fmt.Sprintf("failed to save sticker: %v", err))
		}
		deletedItems = append(deletedItems, stickerPath)
	}

	srcImage, err := imaging.Open(stickerPath)
	if err != nil {
		return response, pkgError.InternalServerError(fmt.Sprintf("failed to open image for sticker conversion: %v", err))
	}

	bounds := srcImage.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	if width > 512 || height > 512 {
		if width > height {
			srcImage = imaging.Resize(srcImage, 512, 0, imaging.Lanczos)
		} else {
			srcImage = imaging.Resize(srcImage, 0, 512, imaging.Lanczos)
		}
	}

	webpPath := filepath.Join(absBaseDir, fmt.Sprintf("sticker_%s.webp", fiberUtils.UUIDv4()))
	deletedItems = append(deletedItems, webpPath)

	pngPath := filepath.Join(absBaseDir, fmt.Sprintf("temp_%s.png", fiberUtils.UUIDv4()))
	deletedItems = append(deletedItems, pngPath)

	err = imaging.Save(srcImage, pngPath)
	if err != nil {
		return response, pkgError.InternalServerError(fmt.Sprintf("failed to save temporary PNG: %v", err))
	}

	var convertCmd *exec.Cmd

	convCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	if _, err := exec.LookPath("ffmpeg"); err == nil {
		convertCmd = exec.CommandContext(convCtx, "ffmpeg", "-y", "-i", pngPath, "-vcodec", "libwebp", "-lossless", "0", "-compression_level", "6", "-q:v", "60", "-preset", "default", "-loop", "0", "-an", "-vsync", "0", webpPath)
	} else if _, err := exec.LookPath("cwebp"); err == nil {
		convertCmd = exec.CommandContext(convCtx, "cwebp", "-q", "60", "-o", webpPath, pngPath)
	} else {
		return response, pkgError.InternalServerError("neither ffmpeg nor cwebp is installed for WebP conversion")
	}

	var stderr bytes.Buffer
	convertCmd.Stderr = &stderr

	if err := convertCmd.Run(); err != nil {
		return response, pkgError.InternalServerError(fmt.Sprintf("failed to convert sticker to WebP: %v, stderr: %s", err, stderr.String()))
	}

	stickerBytes, err = os.ReadFile(webpPath)
	if err != nil {
		return response, pkgError.InternalServerError(fmt.Sprintf("failed to read WebP sticker: %v", err))
	}

	stickerUploaded, err := service.uploadMedia(ctx, whatsmeow.MediaImage, stickerBytes, dataWaRecipient)
	if err != nil {
		return response, pkgError.WaUploadMediaError(fmt.Sprintf("failed to upload sticker: %v", err))
	}

	msg := &waE2E.Message{
		StickerMessage: &waE2E.StickerMessage{
			URL:           proto.String(stickerUploaded.URL),
			DirectPath:    proto.String(stickerUploaded.DirectPath),
			Mimetype:      proto.String("image/webp"),
			FileLength:    proto.Uint64(stickerUploaded.FileLength),
			FileSHA256:    stickerUploaded.FileSHA256,
			FileEncSHA256: stickerUploaded.FileEncSHA256,
			MediaKey:      stickerUploaded.MediaKey,
			Width:         proto.Uint32(uint32(srcImage.Bounds().Dx())),
			Height:        proto.Uint32(uint32(srcImage.Bounds().Dy())),
			IsAnimated:    proto.Bool(false),
		},
	}

	if request.BaseRequest.IsForwarded {
		msg.StickerMessage.ContextInfo = &waE2E.ContextInfo{
			IsForwarded:     proto.Bool(true),
			ForwardingScore: proto.Uint32(100),
		}
	}

	if request.BaseRequest.Duration != nil && *request.BaseRequest.Duration > 0 {
		if msg.StickerMessage.ContextInfo == nil {
			msg.StickerMessage.ContextInfo = &waE2E.ContextInfo{}
		}
		msg.StickerMessage.ContextInfo.Expiration = proto.Uint32(uint32(*request.BaseRequest.Duration))
	}

	content := "🎨 Sticker"

	ts, err := service.wrapSendMessage(ctx, dataWaRecipient, msg, content)
	if err != nil {
		return response, err
	}

	response.MessageID = ts.ID
	response.Status = fmt.Sprintf("Sticker sent to %s (server timestamp: %s)", request.Phone, ts.Timestamp.String())
	return response, nil
}
