package saldo

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type Whisper struct{}

func NewWhisper() *Whisper {
	return &Whisper{}
}

func (w *Whisper) Transcribe(ctx context.Context, audioFilePath string) (string, error) {
	fileBase := filepath.Base(audioFilePath)
	fileExt := filepath.Ext(fileBase)
	fileName := fileBase[:len(fileBase)-len(fileExt)]
	tmpWav := fmt.Sprintf("/tmp/whisper/%s.wav", fileName)
	err := os.MkdirAll(filepath.Dir(tmpWav), 0755)
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpWav)

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-y", // overwrite output file without asking
		"-i", audioFilePath,
		"-ac", "1", // 1 channel
		"-ar", "16000", // 16 kHz
		"-acodec", "pcm_s16le", // 16-bit little-endian PCM
		tmpWav)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ffmpeg error: %w, output: %s", err, string(output))
	}

	cmd = exec.CommandContext(ctx,
		"whisper-cli",
		"-m", "models/ggml-base.bin",
		"-l", "ru",
		"-f", tmpWav,
		"-otxt",
		"-of", "-",
	)
	output, err = cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("whisper-cli error: %w, output: %s", err, string(output))
	}

	return string(output), nil
}
