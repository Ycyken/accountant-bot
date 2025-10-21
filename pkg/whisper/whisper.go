package whisper

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

type Whisper struct{}

func (w *Whisper) Transcribe(ctx context.Context, audioFilePath string) (string, error) {
	tmpWav := "tmp.wav"
	defer os.Remove(tmpWav)
	cmd := exec.Command(
		"ffmpeg",
		"-y", // overwrite output file without asking
		"-i", audioFilePath,
		"-ac", "1", // 1 channel
		"-ar", "16000", // 16 kHz
		"-acodec", "pcm_s16le", // 16-bit little-endian PCM
		tmpWav,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ffmpeg error: %v, output: %s", err, string(output))
	}

	cmd = exec.Command(
		"whisper-cli",
		"-f", tmpWav,
		"-otxt",
		"-of", "-",
	)

	output, err = cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("whisper-cli error: %v, output: %s", err, string(output))
	}

	return string(output), nil
}
