package whisper

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"
)

type Whisper struct{}

func (w *Whisper) Transcribe(ctx context.Context, audioFilePath string) (string, error) {
	tmpWav := "tmp.wav"
	defer os.Remove(tmpWav)

	output, err := execWithTimeout(2*time.Second, "ffmpeg",
		"-y", // overwrite output file without asking
		"-i", audioFilePath,
		"-ac", "1", // 1 channel
		"-ar", "16000", // 16 kHz
		"-acodec", "pcm_s16le", // 16-bit little-endian PCM
		tmpWav)
	if err != nil {
		return "", fmt.Errorf("ffmpeg error: %w, output: %s", err, string(output))
	}

	output, err = execWithTimeout(7*time.Second,
		"whisper-cli",
		"-f", tmpWav,
		"-otxt",
		"-of", "-",
	)
	if err != nil {
		return "", fmt.Errorf("whisper-cli error: %w, output: %s", err, string(output))
	}

	return string(output), nil
}

func execWithTimeout(timeout time.Duration, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	return out, err
}
