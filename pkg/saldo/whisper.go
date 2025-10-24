package saldo

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

type LocalWhisper struct{}

func NewLocalWhisper() *LocalWhisper {
	return &LocalWhisper{}
}

func (w *LocalWhisper) Transcribe(ctx context.Context, oggFilePath string) (string, error) {
	tmpWav, err := ConvertOggToWav(ctx, oggFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to convert ogg to wav: %w", err)
	}
	defer os.Remove(tmpWav)

	cmd := exec.CommandContext(ctx,
		"whisper-cli",
		"-m", "models/ggml-base.bin",
		"-l", "ru",
		"-f", tmpWav,
		"-otxt",
		"-of", "-",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("whisper-cli error: %w, output: %s", err, string(output))
	}

	return string(output), nil
}
