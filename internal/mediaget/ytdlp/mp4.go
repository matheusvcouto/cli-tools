package ytdlp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/matheusvcouto/cli-tools/internal/mediaget"
	"github.com/matheusvcouto/cli-tools/internal/safefs"
)

type videoStreams struct {
	Streams []struct {
		Type   string `json:"codec_type"`
		Codec  string `json:"codec_name"`
		Pixels string `json:"pix_fmt"`
	} `json:"streams"`
}

func (s videoStreams) codecs() (video, audio string, compatible bool) {
	for _, stream := range s.Streams {
		if stream.Type == "video" && video == "" {
			video = stream.Codec
			compatible = video == "h264" && stream.Pixels == "yuv420p"
		}
		if stream.Type == "audio" && audio == "" {
			audio = stream.Codec
		}
	}
	return video, audio, compatible
}
func (a Adapter) probeVideo(ctx context.Context, path string) (videoStreams, error) {
	ffmpeg, err := dependency("ffmpeg", a.FFmpeg)
	if err != nil {
		return videoStreams{}, err
	}
	probe, err := dependency("ffprobe", filepath.Join(filepath.Dir(ffmpeg), "ffprobe"))
	if err != nil {
		return videoStreams{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := command(ctx, probe, []string{"-v", "error", "-show_entries", "stream=codec_type,codec_name,pix_fmt", "-of", "json", path})
	var out bytes.Buffer
	cmd.Stdout = &boundedWriter{w: &out, remaining: 1 << 20}
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return videoStreams{}, conversionError(ctx, err, "verificar codecs")
	}
	var streams videoStreams
	if json.Unmarshal(out.Bytes(), &streams) != nil {
		return streams, errors.New("ffprobe retornou codecs inválidos")
	}
	video, _, _ := streams.codecs()
	if video == "" {
		return streams, errors.New("arquivo não contém uma faixa de vídeo válida")
	}
	return streams, nil
}
func conversionError(ctx context.Context, err error, action string) error {
	if ctx.Err() != nil {
		return fmt.Errorf("%s: %w", action, ctx.Err())
	}
	// Child diagnostics may contain URLs; retain cause without echoing stderr.
	return fmt.Errorf("FFmpeg/ffprobe falhou ao %s; verifique os codecs/encoders da instalação: %w", action, err)
}
func (a Adapter) compatibleMP4(ctx context.Context, work string, progress func(mediaget.Progress)) error {
	root, err := safefs.Open(work)
	if err != nil {
		return err
	}
	defer root.Close()
	dir, err := root.Open(".")
	if err != nil {
		return err
	}
	entries, err := dir.ReadDir(-1)
	err = errors.Join(err, dir.Close())
	if err != nil {
		return err
	}
	if len(entries) != 1 || !entries[0].Type().IsRegular() {
		return errors.New("conversão MP4 exige exatamente um arquivo regular")
	}
	source := entries[0].Name()
	original, err := root.Lstat(source)
	if err != nil {
		return err
	}
	if !original.Mode().IsRegular() || original.Size() == 0 {
		return errors.New("vídeo vazio ou inválido")
	}
	input := filepath.Join(work, source)
	streams, err := a.probeVideo(ctx, input)
	if err != nil {
		return err
	}
	_, audio, videoOK := streams.codecs()
	if videoOK && (audio == "" || audio == "aac") && filepath.Ext(source) == ".mp4" {
		return nil
	}
	if progress != nil {
		progress(mediaget.Progress{Stage: mediaget.Processing})
	}
	ffmpeg, err := dependency("ffmpeg", a.FFmpeg)
	if err != nil {
		return err
	}
	target := "compatible.mp4"
	if source == target {
		return errors.New("nome intermediário reservado")
	}
	args := []string{"-nostdin", "-n", "-v", "error", "-i", input, "-map", "0:v:0", "-map", "0:a:0?"}
	if videoOK {
		args = append(args, "-c:v", "copy")
	} else {
		args = append(args, "-c:v", "libx264", "-crf", "18", "-preset", "medium", "-pix_fmt", "yuv420p")
	}
	if audio == "" || audio == "aac" {
		args = append(args, "-c:a", "copy")
	} else {
		args = append(args, "-c:a", "aac", "-b:a", "192k")
	}
	args = append(args, "-movflags", "+faststart", filepath.Join(work, target))
	cmd := command(ctx, ffmpeg, args)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return conversionError(ctx, err, "gerar MP4 compatível")
	}
	file, err := root.Lstat(target)
	if err != nil {
		return err
	}
	if !file.Mode().IsRegular() || file.Size() == 0 {
		return errors.New("conversão gerou MP4 vazio ou inválido")
	}
	verified, err := a.probeVideo(ctx, filepath.Join(work, target))
	if err != nil {
		return err
	}
	_, finalAudio, ok := verified.codecs()
	if !ok || (audio != "" && finalAudio == "") || (finalAudio != "" && finalAudio != "aac") {
		return errors.New("MP4 final não preservou vídeo/áudio compatíveis")
	}
	current, err := root.Lstat(source)
	if err != nil {
		return err
	}
	if !os.SameFile(original, current) {
		return errors.New("vídeo intermediário mudou durante a conversão")
	}
	// Only discard the generated intermediate after validating the complete result.
	return root.Remove(source)
}
