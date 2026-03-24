package embed

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

const (
	hfRepo     = "nomic-ai/nomic-embed-text-v1.5"
	hfBaseURL  = "https://huggingface.co"
	onnxModel  = "onnx/model_quantized.onnx"
	vocabFile  = "vocab.txt"
	localModel = "model.onnx"
)

// ensureModelFiles downloads the ONNX embedding model and vocabulary if they
// are not already present in modelDir.
func ensureModelFiles(modelDir string) error {
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		return err
	}

	modelPath := filepath.Join(modelDir, localModel)
	vocabPath := filepath.Join(modelDir, vocabFile)

	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		url := fmt.Sprintf("%s/%s/resolve/main/%s", hfBaseURL, hfRepo, onnxModel)
		if err := downloadModelFile(url, modelPath); err != nil {
			return fmt.Errorf("download model: %w", err)
		}
	}

	if _, err := os.Stat(vocabPath); os.IsNotExist(err) {
		url := fmt.Sprintf("%s/%s/resolve/main/%s", hfBaseURL, hfRepo, vocabFile)
		if err := downloadModelFile(url, vocabPath); err != nil {
			return fmt.Errorf("download vocab: %w", err)
		}
	}

	return nil
}

func downloadModelFile(url, destPath string) error {
	log.Printf("embed: downloading %s...", url)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s returned %d", url, resp.StatusCode)
	}

	tmpPath := destPath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	n, err := io.Copy(f, resp.Body)
	f.Close()
	if err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("write: %w", err)
	}

	if err := os.Rename(tmpPath, destPath); err != nil {
		os.Remove(tmpPath)
		return err
	}

	log.Printf("embed: downloaded %s (%d MB)", filepath.Base(destPath), n/1024/1024)
	return nil
}
