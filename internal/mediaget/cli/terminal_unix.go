//go:build darwin || linux

package mediacli

import (
	"context"
	"errors"
	"io"
	"os"
	"time"
	"unicode/utf8"

	"golang.org/x/sys/unix"
)

func nativeTerminalAvailable() bool { return true }
func readByte(ctx context.Context, in *os.File, timeout time.Duration) (byte, error) {
	deadline := time.Now().Add(timeout)
	for {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		wait := 50
		if timeout > 0 {
			wait = max(0, min(wait, int(time.Until(deadline).Milliseconds())))
		}
		fds := []unix.PollFd{{Fd: int32(in.Fd()), Events: unix.POLLIN}}
		n, err := unix.Poll(fds, wait)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return 0, err
		}
		if n > 0 {
			var b [1]byte
			count, err := unix.Read(int(in.Fd()), b[:])
			if errors.Is(err, unix.EINTR) {
				continue
			}
			if err != nil {
				return 0, err
			}
			if count == 0 {
				return 0, io.EOF
			}
			return b[0], nil
		}
		if timeout > 0 && !time.Now().Before(deadline) {
			return 0, os.ErrDeadlineExceeded
		}
	}
}
func readTerminalKey(ctx context.Context, in *os.File) (string, error) {
	b, err := readByte(ctx, in, 150*time.Millisecond)
	if err != nil {
		return "", err
	}
	switch b {
	case 4:
		return "eof", nil
	case 1:
		return "home", nil
	case 5:
		return "end", nil
	case 3:
		return "interrupt", nil
	case 13, 10:
		return "enter", nil
	case 127, 8:
		return "backspace", nil
	case 27:
		next, err := readByte(ctx, in, 100*time.Millisecond)
		if errors.Is(err, os.ErrDeadlineExceeded) {
			return "cancel", nil
		}
		if err != nil {
			return "", err
		}
		if next != '[' && next != 'O' {
			return "", nil
		}
		sequence := make([]byte, 0, 16)
		for len(sequence) < 32 {
			value, e := readByte(ctx, in, 100*time.Millisecond)
			if errors.Is(e, os.ErrDeadlineExceeded) {
				return "", nil
			}
			if e != nil {
				return "", e
			}
			sequence = append(sequence, value)
			if value >= 0x40 && value <= 0x7e {
				break
			}
		}
		if len(sequence) == 0 {
			return "", nil
		}
		last := sequence[len(sequence)-1]
		switch last {
		case 'A':
			return "up", nil
		case 'B':
			return "down", nil
		case 'C':
			return "right", nil
		case 'D':
			return "left", nil
		case 'H':
			return "home", nil
		case 'F':
			return "end", nil
		case '~':
			switch string(sequence[:len(sequence)-1]) {
			case "1", "7":
				return "home", nil
			case "4", "8":
				return "end", nil
			case "3":
				return "delete", nil
			}
		}

		return "", nil
	}
	if b < 32 {
		return "", nil
	}
	data := []byte{b}
	for !utf8.FullRune(data) && len(data) < utf8.UTFMax {
		b, err = readByte(ctx, in, 100*time.Millisecond)
		if err != nil {
			return "", err
		}
		data = append(data, b)
	}
	r, _ := utf8.DecodeRune(data)
	if r == utf8.RuneError {
		return "", nil
	}
	return string(r), nil
}
