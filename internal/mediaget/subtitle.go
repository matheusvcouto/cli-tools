package mediaget

import (
	"errors"
	"html"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/matheusvcouto/cli-tools/internal/safefs"
)

const maxSubtitleBytes = 32 << 20

var subtitleTime = regexp.MustCompile(`^(\d{2,}):(\d{2}):(\d{2}),(\d{3})\s+-->\s+(\d{2,}):(\d{2}):(\d{2}),(\d{3})(?:\s+.*)?$`)
var subtitleMarkup = regexp.MustCompile(`<[^>]*>`)

// SubtitleText validates SRT cues and emits UTF-8 prose. Overlapping rolling
// captions are joined without repeating their shared words; later repeats stay.
func SubtitleText(data []byte) (string, error) {
	if len(data) > maxSubtitleBytes || !utf8.Valid(data) {
		return "", errors.New("legenda excede 32 MiB ou não é UTF-8 válida")
	}
	raw := strings.TrimPrefix(string(data), "\ufeff")
	raw = strings.ReplaceAll(strings.ReplaceAll(raw, "\r\n", "\n"), "\r", "\n")
	var blocks []string
	var block []string
	for _, line := range strings.Split(raw, "\n") {
		if strings.TrimSpace(line) == "" {
			if len(block) > 0 {
				blocks = append(blocks, strings.Join(block, "\n"))
				block = nil
			}
			continue
		}
		block = append(block, line)
	}
	if len(block) > 0 {
		blocks = append(blocks, strings.Join(block, "\n"))
	}
	var output []string
	var previous []string
	var previousEnd int64 = -1
	for _, block := range blocks {
		lines := strings.Split(block, "\n")
		if len(lines) > 0 {
			if _, err := strconv.ParseUint(strings.TrimSpace(lines[0]), 10, 64); err == nil {
				lines = lines[1:]
			}
		}
		if len(lines) < 2 {
			return "", errors.New("legenda SRT inválida: bloco incompleto")
		}
		match := subtitleTime.FindStringSubmatch(strings.TrimSpace(lines[0]))
		if match == nil {
			return "", errors.New("legenda SRT inválida: marcação de tempo")
		}
		timestamp := func(offset int) (int64, error) {
			h, e := strconv.ParseInt(match[offset], 10, 32)
			if e != nil {
				return 0, e
			}
			m, _ := strconv.ParseInt(match[offset+1], 10, 64)
			s, _ := strconv.ParseInt(match[offset+2], 10, 64)
			ms, _ := strconv.ParseInt(match[offset+3], 10, 64)
			if m >= 60 || s >= 60 {
				return 0, errors.New("tempo inválido")
			}
			return ((h*60+m)*60+s)*1000 + ms, nil
		}
		start, err := timestamp(1)
		if err != nil {
			return "", err
		}
		end, err := timestamp(5)
		if err != nil || end < start {
			return "", errors.New("intervalo de legenda inválido")
		}
		text := html.UnescapeString(subtitleMarkup.ReplaceAllString(strings.Join(lines[1:], " "), ""))
		text = strings.Map(func(r rune) rune {
			if unicode.IsControl(r) {
				return ' '
			}
			return r
		}, text)
		words := strings.Fields(text)
		if len(words) == 0 {
			continue
		}
		overlap := 0
		if start < previousEnd {
			for n := min(len(previous), len(words)); n > 0; n-- {
				if strings.Join(previous[len(previous)-n:], " ") == strings.Join(words[:n], " ") {
					overlap = n
					break
				}
			}
		}
		if overlap < len(words) {
			output = append(output, strings.Join(words[overlap:], " "))
		}
		previous, previousEnd = words, end
	}
	if len(output) == 0 {
		return "", errors.New("legenda sem texto")
	}
	return strings.Join(output, "\n") + "\n", nil
}

func subtitleText(root *safefs.Root, source, work string) (string, os.FileInfo, error) {
	input, err := root.OpenFile(source, os.O_RDONLY, 0)
	if err != nil {
		return "", nil, err
	}
	data, err := io.ReadAll(io.LimitReader(input, maxSubtitleBytes+1))
	err = errors.Join(err, input.Close())
	if err != nil {
		return "", nil, err
	}
	text, err := SubtitleText(data)
	if err != nil {
		return "", nil, err
	}
	target := filepath.Join(work, "transcript.txt")
	file, err := root.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", nil, err
	}
	_, err = io.WriteString(file, text)
	err = errors.Join(err, file.Close())
	if err != nil {
		return "", nil, err
	}
	info, err := root.Lstat(target)
	return target, info, err
}
