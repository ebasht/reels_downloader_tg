package instagram

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"video_download_bot/internal/domain"
)

const (
	maxVideoBytes    = 49 * 1024 * 1024
	targetVideoBytes = 45 * 1024 * 1024
	audioBitrate     = 128_000 // bits/s
)

type videoMeta struct {
	Width    int
	Height   int
	Duration int
}

// ReelDownloader downloads reels with yt-dlp and re-encodes them with ffmpeg
// into a Telegram-friendly mp4 under the bot upload limit.
type ReelDownloader struct {
	timeout     time.Duration
	maxDuration time.Duration
	tmpDir      string
}

func NewReelDownloader(timeout, maxDuration time.Duration) *ReelDownloader {
	return &ReelDownloader{timeout: timeout, maxDuration: maxDuration, tmpDir: os.TempDir()}
}

func (d *ReelDownloader) DownloadReel(ctx context.Context, reelURL string) (*domain.Video, error) {
	ctx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()

	videoPath, err := d.download(ctx, reelURL)
	if err != nil {
		return nil, fmt.Errorf("download: %w", err)
	}
	defer os.Remove(videoPath)

	// yt-dlp lets videos with unknown duration through its filter.
	if raw := probeVideo(ctx, videoPath); time.Duration(raw.Duration)*time.Second > d.maxDuration {
		return nil, fmt.Errorf("%w: %ds", domain.ErrVideoTooLong, raw.Duration)
	}

	compatPath, err := d.ensureTelegramCompatible(ctx, videoPath)
	if err != nil {
		return nil, fmt.Errorf("transcode: %w", err)
	}

	info, err := os.Stat(compatPath)
	if err != nil || info.Size() == 0 {
		_ = os.Remove(compatPath)
		return nil, fmt.Errorf("empty or missing file")
	}
	if info.Size() > maxVideoBytes {
		_ = os.Remove(compatPath)
		return nil, fmt.Errorf("file too large after transcode (%d bytes)", info.Size())
	}

	meta := probeVideo(ctx, compatPath)
	if meta.Width == 0 || meta.Height == 0 {
		_ = os.Remove(compatPath)
		return nil, fmt.Errorf("missing video dimensions")
	}
	log.Printf("instagram video meta: %dx%d duration=%ds size=%d", meta.Width, meta.Height, meta.Duration, info.Size())

	thumbPath, err := d.extractThumbnail(ctx, compatPath)
	if err != nil {
		log.Printf("instagram thumbnail: %v", err)
		thumbPath = ""
	}

	return &domain.Video{
		Path:          compatPath,
		ThumbnailPath: thumbPath,
		Width:         meta.Width,
		Height:        meta.Height,
		Duration:      meta.Duration,
		Release: func() {
			_ = os.Remove(compatPath)
			if thumbPath != "" {
				_ = os.Remove(thumbPath)
			}
		},
	}, nil
}

func (d *ReelDownloader) download(ctx context.Context, reelURL string) (string, error) {
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		return "", fmt.Errorf("yt-dlp not found in PATH: %w", err)
	}

	id := uuid.NewString()
	outTemplate := filepath.Join(d.tmpDir, fmt.Sprintf("ig_%s.%%(ext)s", id))

	// Do NOT use --max-filesize: yt-dlp can abort the video leg and still keep
	// the audio file, which then becomes a gray-screen Telegram video.
	args := []string{
		"--no-playlist",
		"--no-warnings",
		"--retries", "3",
		"--fragment-retries", "3",
		"--impersonate", "chrome",
		"--match-filter", fmt.Sprintf("!is_live & duration<=?%d", int(d.maxDuration.Seconds())),
		"-f", "bv*[height<=720]+ba/b[height<=720]/bv*+ba/b",
		"--merge-output-format", "mp4",
		"-o", outTemplate,
		reelURL,
	}

	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		d.removeAll(id)
		if ctx.Err() != nil {
			return "", fmt.Errorf("download timeout: %w", ctx.Err())
		}
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	// A filtered video is skipped with exit code 0 and no output file.
	if strings.Contains(string(out), "does not pass filter") {
		d.removeAll(id)
		return "", fmt.Errorf("%w: longer than %s or live", domain.ErrVideoTooLong, d.maxDuration)
	}

	path, err := d.findDownloadedVideo(id)
	if err != nil {
		d.removeAll(id)
		return "", err
	}
	if !hasVideoStream(ctx, path) {
		d.removeAll(id)
		return "", fmt.Errorf("downloaded file has no video stream")
	}
	d.cleanupArtifacts(id)
	return path, nil
}

func (d *ReelDownloader) findDownloadedVideo(id string) (string, error) {
	matches, err := filepath.Glob(filepath.Join(d.tmpDir, fmt.Sprintf("ig_%s*", id)))
	if err != nil {
		return "", err
	}

	var candidates []string
	for _, path := range matches {
		if isVideoFile(path) {
			candidates = append(candidates, path)
		} else {
			// Drop leftover audio-only fragments from aborted merges.
			_ = os.Remove(path)
		}
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("downloaded video file not found")
	}
	return candidates[0], nil
}

func (d *ReelDownloader) cleanupArtifacts(id string) {
	matches, err := filepath.Glob(filepath.Join(d.tmpDir, fmt.Sprintf("ig_%s*", id)))
	if err != nil {
		return
	}
	for _, path := range matches {
		if !isVideoFile(path) {
			_ = os.Remove(path)
		}
	}
}

func (d *ReelDownloader) removeAll(id string) {
	matches, _ := filepath.Glob(filepath.Join(d.tmpDir, fmt.Sprintf("ig_%s*", id)))
	for _, path := range matches {
		_ = os.Remove(path)
	}
}

func isVideoFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp4", ".mkv", ".webm", ".mov":
		return true
	default:
		return false
	}
}

func hasVideoStream(ctx context.Context, path string) bool {
	cmd := exec.CommandContext(ctx, "ffprobe", "-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=codec_type",
		"-of", "csv=p=0",
		path,
	)
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(strings.TrimSpace(string(out)), "video")
}

func (d *ReelDownloader) ensureTelegramCompatible(ctx context.Context, inputPath string) (string, error) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return "", fmt.Errorf("ffmpeg not found in PATH: %w", err)
	}

	meta := probeVideo(ctx, inputPath)
	duration := meta.Duration
	if duration < 1 {
		duration = 1
	}

	// Keep under Telegram bot upload limit (~50MB) via target average bitrate.
	videoBitrate := int(float64(targetVideoBytes*8)/float64(duration)) - audioBitrate
	if videoBitrate < 250_000 {
		videoBitrate = 250_000
	}
	if videoBitrate > 2_500_000 {
		videoBitrate = 2_500_000
	}

	// Long reels: shrink frame size so the bitrate budget still looks OK.
	maxWidth := 720
	if duration > 180 {
		maxWidth = 540
	}

	outPath := filepath.Join(d.tmpDir, fmt.Sprintf("ig_tg_%s.mp4", uuid.NewString()))
	vf := fmt.Sprintf(
		"scale='min(%d,iw)':-2,scale=trunc(iw/2)*2:trunc(ih/2)*2,setsar=1",
		maxWidth,
	)

	cmd := exec.CommandContext(ctx, "ffmpeg", "-y", "-i", inputPath,
		"-map_metadata", "-1",
		"-vf", vf,
		"-metadata:s:v:0", "rotate=0",
		"-c:v", "libx264",
		"-profile:v", "baseline",
		"-level", "3.1",
		"-preset", "veryfast",
		"-b:v", strconv.Itoa(videoBitrate),
		"-maxrate", strconv.Itoa(videoBitrate*3/2),
		"-bufsize", strconv.Itoa(videoBitrate*3),
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-b:a", "128k",
		"-ac", "2",
		"-ar", "44100",
		"-movflags", "+faststart",
		outPath,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		_ = os.Remove(outPath)
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}

	info, err := os.Stat(outPath)
	if err != nil {
		_ = os.Remove(outPath)
		return "", err
	}
	if info.Size() > maxVideoBytes {
		_ = os.Remove(outPath)
		return "", fmt.Errorf("transcoded file still too large: %d bytes", info.Size())
	}
	if !hasVideoStream(ctx, outPath) {
		_ = os.Remove(outPath)
		return "", fmt.Errorf("transcoded file has no video stream")
	}
	return outPath, nil
}

func (d *ReelDownloader) extractThumbnail(ctx context.Context, path string) (string, error) {
	outPath := filepath.Join(d.tmpDir, fmt.Sprintf("ig_thumb_%s.jpg", uuid.NewString()))
	cmd := exec.CommandContext(ctx, "ffmpeg", "-y", "-ss", "0.5", "-i", path,
		"-frames:v", "1",
		"-q:v", "2",
		"-update", "1",
		outPath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		_ = os.Remove(outPath)
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return outPath, nil
}

func probeVideo(ctx context.Context, path string) videoMeta {
	cmd := exec.CommandContext(ctx, "ffprobe", "-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height:format=duration",
		"-of", "csv=p=0",
		path,
	)
	out, err := cmd.Output()
	if err != nil {
		return videoMeta{}
	}

	// csv output like: 720,1280\n71.633333
	parts := strings.FieldsFunc(strings.TrimSpace(string(out)), func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r'
	})
	meta := videoMeta{}
	if len(parts) >= 2 {
		meta.Width, _ = strconv.Atoi(strings.TrimSpace(parts[0]))
		meta.Height, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
	}
	if len(parts) >= 3 {
		if d, err := strconv.ParseFloat(strings.TrimSpace(parts[2]), 64); err == nil {
			meta.Duration = int(d + 0.5)
		}
	}
	return meta
}
