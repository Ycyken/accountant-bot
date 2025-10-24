package saldo

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func ConvertOggToWav(ctx context.Context, oggFilePath string) (string, error) {
	fileBase := filepath.Base(oggFilePath)
	fileExt := filepath.Ext(fileBase)
	fileName := fileBase[:len(fileBase)-len(fileExt)]
	wav := fmt.Sprintf("/tmp/saldo/%s.wav", fileName)
	err := os.MkdirAll(filepath.Dir(wav), 0755)
	if err != nil {
		return "", err
	}

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-y", // overwrite output file without asking
		"-i", oggFilePath,
		"-ac", "1", // 1 channel
		"-ar", "16000", // 16 kHz
		"-acodec", "pcm_s16le", // 16-bit little-endian PCM
		wav)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ffmpeg error: %w, output: %s", err, string(output))
	}

	return wav, nil
}
