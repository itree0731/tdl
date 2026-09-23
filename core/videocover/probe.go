package videocover

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/iyear/tdl/core/util/mediautil"
)

// Probe returns the metadata needed for Telegram's video attribute. MP4 files
// are parsed in-process; other containers use ffprobe from the ffmpeg package.
func Probe(ctx context.Context, path string) (time.Duration, int, int, error) {
	file, err := os.Open(path)
	if err == nil {
		seconds, width, height, parseErr := mediautil.GetMP4Info(file)
		_ = file.Close()
		if parseErr == nil && width > 0 && height > 0 {
			return time.Duration(seconds) * time.Second, width, height, nil
		}
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		return 0, 0, 0, fmt.Errorf("video metadata requires ffprobe for this file: %w", err)
	}
	output, err := exec.CommandContext(ctx, ffprobe, "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=width,height:format=duration", "-of", "json", path).Output()
	if err != nil {
		return 0, 0, 0, fmt.Errorf("probe video: %w", err)
	}
	var result struct {
		Streams []struct{ Width, Height int } `json:"streams"`
		Format  struct{ Duration string }     `json:"format"`
	}
	if err = json.Unmarshal(output, &result); err != nil {
		return 0, 0, 0, fmt.Errorf("parse video metadata: %w", err)
	}
	if len(result.Streams) == 0 || result.Streams[0].Width < 1 || result.Streams[0].Height < 1 {
		return 0, 0, 0, fmt.Errorf("video has no usable image stream")
	}
	seconds, err := strconv.ParseFloat(result.Format.Duration, 64)
	if err != nil || seconds < 0 {
		return 0, 0, 0, fmt.Errorf("invalid video duration %q", result.Format.Duration)
	}
	return time.Duration(seconds * float64(time.Second)), result.Streams[0].Width, result.Streams[0].Height, nil
}
