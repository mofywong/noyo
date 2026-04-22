package cascade

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
)

const (
	ChunkSize = 1024 * 10 // 10KB per chunk
)

// FileTransferInfo represents metadata for a file transfer
type FileTransferInfo struct {
	FileID     string `json:"file_id"`
	FileName   string `json:"file_name"`
	TotalSize  int64  `json:"total_size"`
	TotalChunk int    `json:"total_chunk"`
	MD5        string `json:"md5"` // For integrity check
}

// FileChunk represents a single chunk of the file
type FileChunk struct {
	FileID string `json:"file_id"`
	Index  int    `json:"index"`
	Data   []byte `json:"data"`
}

// FileReceiver handles receiving and assembling file chunks
type FileReceiver struct {
	mu         sync.Mutex
	Info       FileTransferInfo
	received   map[int]bool
	tempFile   *os.File
	logger     *zap.Logger
	onComplete func(filePath string)
	timeout    *time.Timer
}

// NewFileReceiver initializes a new receiver for a file
func NewFileReceiver(info FileTransferInfo, destDir string, logger *zap.Logger, onComplete func(string)) (*FileReceiver, error) {
	err := os.MkdirAll(destDir, 0755)
	if err != nil {
		return nil, err
	}

	tempPath := filepath.Join(destDir, info.FileID+".tmp")
	f, err := os.OpenFile(tempPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}

	// Pre-allocate space
	if err := f.Truncate(info.TotalSize); err != nil {
		f.Close()
		os.Remove(tempPath)
		return nil, err
	}

	fr := &FileReceiver{
		Info:       info,
		received:   make(map[int]bool),
		tempFile:   f,
		logger:     logger,
		onComplete: onComplete,
	}

	// Timeout if transfer doesn't complete in 5 minutes
	fr.timeout = time.AfterFunc(5*time.Minute, func() {
		fr.cleanup(false)
		logger.Error("File transfer timed out", zap.String("file_id", info.FileID))
	})

	return fr, nil
}

// ReceiveChunk writes a chunk to the temp file
func (fr *FileReceiver) ReceiveChunk(chunk FileChunk) error {
	fr.mu.Lock()
	defer fr.mu.Unlock()

	if fr.received[chunk.Index] {
		return nil // Already received
	}

	offset := int64(chunk.Index * ChunkSize)
	_, err := fr.tempFile.WriteAt(chunk.Data, offset)
	if err != nil {
		return fmt.Errorf("failed to write chunk: %w", err)
	}

	fr.received[chunk.Index] = true

	// Check if complete
	if len(fr.received) == fr.Info.TotalChunk {
		fr.timeout.Stop()
		fr.cleanup(true)
	}

	return nil
}

func (fr *FileReceiver) cleanup(success bool) {
	fr.tempFile.Close()
	tempPath := fr.tempFile.Name()

	if success {
		// Rename temp file to final name
		finalPath := filepath.Join(filepath.Dir(tempPath), fr.Info.FileName)
		os.Rename(tempPath, finalPath)
		fr.logger.Info("File transfer complete", zap.String("file", finalPath))
		if fr.onComplete != nil {
			go fr.onComplete(finalPath)
		}
	} else {
		os.Remove(tempPath)
	}
}

// FileSender handles reading and publishing a file in chunks
type FileSender struct {
	Info     FileTransferInfo
	filePath string
	publish  func(topic string, payload []byte) error
	logger   *zap.Logger
}

func NewFileSender(filePath string, publish func(string, []byte) error, logger *zap.Logger) (*FileSender, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}

	totalSize := fileInfo.Size()
	totalChunk := int(totalSize / int64(ChunkSize))
	if totalSize%int64(ChunkSize) != 0 {
		totalChunk++
	}

	fileID := fmt.Sprintf("%s-%d", fileInfo.Name(), time.Now().Unix())

	info := FileTransferInfo{
		FileID:     fileID,
		FileName:   fileInfo.Name(),
		TotalSize:  totalSize,
		TotalChunk: totalChunk,
	}

	return &FileSender{
		Info:     info,
		filePath: filePath,
		publish:  publish,
		logger:   logger,
	}, nil
}

func (fs *FileSender) Send(gwSn string) error {
	// Publish Metadata
	metaTopic := fmt.Sprintf("noyo/cascade/gw/%s/file/meta", gwSn)
	metaBytes, _ := json.Marshal(fs.Info)
	if err := fs.publish(metaTopic, metaBytes); err != nil {
		return err
	}

	// Wait briefly for receiver to prepare
	time.Sleep(500 * time.Millisecond)

	// Publish Chunks
	f, err := os.Open(fs.filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	buf := make([]byte, ChunkSize)
	for i := 0; i < fs.Info.TotalChunk; i++ {
		n, err := f.Read(buf)
		if err != nil {
			return err
		}

		chunk := FileChunk{
			FileID: fs.Info.FileID,
			Index:  i,
			Data:   buf[:n],
		}

		chunkBytes, _ := json.Marshal(chunk)
		chunkTopic := fmt.Sprintf("noyo/cascade/gw/%s/file/chunk", gwSn)
		if err := fs.publish(chunkTopic, chunkBytes); err != nil {
			fs.logger.Error("Failed to publish chunk", zap.Int("index", i), zap.Error(err))
		}

		// Throttle transmission to avoid overwhelming MQTT broker
		time.Sleep(10 * time.Millisecond)
	}

	fs.logger.Info("File chunks published successfully", zap.String("file", fs.Info.FileName))
	return nil
}
