package webbuild

import (
	"bytes"
	"compress/flate"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type zipEntry struct {
	name     string
	method   uint16
	compSize int64
	localOff int64
}

func le16(b []byte) uint16 { return binary.LittleEndian.Uint16(b) }
func le32(b []byte) uint32 { return binary.LittleEndian.Uint32(b) }

func fetchBytes(ctx context.Context, url string, start, end int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func readCentralDir(ctx context.Context, url string, size int64) ([]zipEntry, error) {
	tailLen := int64(256 * 1024)
	if size < tailLen {
		tailLen = size
	}
	tail, err := fetchBytes(ctx, url, size-tailLen, size-1)
	if err != nil {
		return nil, err
	}
	eocd := -1
	for i := len(tail) - 22; i >= 0; i-- {
		if le32(tail[i:]) == 0x06054b50 {
			eocd = i
			break
		}
	}
	if eocd < 0 {
		return nil, errors.New("EOCD not found")
	}
	cdSize := int64(le32(tail[eocd+12:]))
	cdOff := int64(le32(tail[eocd+16:]))
	if cdSize <= 0 || cdOff+cdSize > size {
		return nil, errors.New("bad central directory")
	}
	var cd []byte
	if cdOff >= size-tailLen {
		start := cdOff - (size - tailLen)
		cd = tail[start : start+cdSize]
	} else {
		cd, err = fetchBytes(ctx, url, cdOff, cdOff+cdSize-1)
		if err != nil {
			return nil, err
		}
	}
	var out []zipEntry
	for i := 0; i+46 <= len(cd); {
		if le32(cd[i:]) != 0x02014b50 {
			return nil, errors.New("bad central directory entry")
		}
		method := le16(cd[i+10:])
		comp := int64(le32(cd[i+20:]))
		nameLen := int(le16(cd[i+28:]))
		extraLen := int(le16(cd[i+30:]))
		commentLen := int(le16(cd[i+32:]))
		local := int64(le32(cd[i+42:]))
		if comp == 0xFFFFFFFF || local == 0xFFFFFFFF {
			return nil, errors.New("zip64 not supported")
		}
		if i+46+nameLen > len(cd) {
			return nil, errors.New("truncated central directory")
		}
		out = append(out, zipEntry{
			name:     string(cd[i+46 : i+46+nameLen]),
			method:   method,
			compSize: comp,
			localOff: local,
		})
		i += 46 + nameLen + extraLen + commentLen
	}
	return out, nil
}

func extractRemoteSDK(ctx context.Context, url string, size int64, prefix, dst string, onBytes func(done, total int64)) error {
	entries, err := readCentralDir(ctx, url, size)
	if err != nil {
		return err
	}
	var wanted []zipEntry
	minOff := int64(1 << 62)
	maxOff := int64(0)
	for _, e := range entries {
		if strings.HasSuffix(e.name, "/") {
			continue
		}
		if e.name == prefix+"renpy.py" || strings.HasPrefix(e.name, prefix+"renpy/") {
			wanted = append(wanted, e)
			if e.localOff < minOff {
				minOff = e.localOff
			}
			end := e.localOff + 30 + int64(len(e.name)) + 512 + e.compSize
			if end > maxOff {
				maxOff = end
			}
		}
	}
	if len(wanted) == 0 {
		return errors.New("SDK 中没有 renpy 运行库")
	}
	if maxOff > size {
		maxOff = size
	}
	if minOff >= maxOff {
		return errors.New("SDK 偏移异常")
	}
	span := maxOff - minOff
	if span <= 0 || span > 64<<20 {
		return fmt.Errorf("renpy 子树过大：%d 字节", span)
	}
	buf, err := fetchSpan(ctx, url, minOff, maxOff-1, onBytes)
	if err != nil {
		return err
	}
	for _, e := range wanted {
		rel := e.name[len(prefix):]
		if rel == "renpy.py" {
			if err := writeBytes(filepath.Join(dst, "renpy.py"), buf, e, minOff); err != nil {
				return err
			}
			continue
		}
		rel = strings.TrimPrefix(rel, "renpy/")
		if rel == "" {
			continue
		}
		if err := writeBytes(filepath.Join(dst, "renpy", filepath.FromSlash(rel)), buf, e, minOff); err != nil {
			return err
		}
	}
	return nil
}

func writeBytes(dst string, buf []byte, e zipEntry, base int64) error {
	rel := e.localOff - base
	if rel < 0 || rel+30 > int64(len(buf)) {
		return errors.New("条目越界")
	}
	h := buf[rel:]
	if le32(h) != 0x04034b50 {
		return errors.New("local header 异常")
	}
	nameLen := int(le16(h[26:]))
	extraLen := int(le16(h[28:]))
	dataOff := rel + 30 + int64(nameLen) + int64(extraLen)
	if dataOff+e.compSize > int64(len(buf)) {
		return fmt.Errorf("条目数据越界：%s", e.name)
	}
	raw := buf[dataOff : dataOff+e.compSize]
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	switch e.method {
	case 0:
		_, err = out.Write(raw)
	case 8:
		fr := flate.NewReader(bytes.NewReader(raw))
		_, err = io.Copy(out, fr)
		fr.Close()
	default:
		return fmt.Errorf("不支持的压缩方式 %d", e.method)
	}
	return err
}

func fetchSpan(ctx context.Context, url string, start, end int64, onBytes func(done, total int64)) ([]byte, error) {
	total := end - start + 1
	const workers = 8
	chunk := (total + workers - 1) / workers
	parts := make([][]byte, workers)
	var done int64
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	for i := 0; i < workers; i++ {
		s := start + int64(i)*chunk
		e := s + chunk - 1
		if e > end {
			e = end
		}
		if s > end {
			break
		}
		wg.Add(1)
		go func(i int, s, e int64) {
			defer wg.Done()
			var b []byte
			var err error
			for attempt := 0; attempt < 4; attempt++ {
				b, err = fetchBytes(ctx, url, s, e)
				if err == nil {
					break
				}
				select {
				case <-ctx.Done():
					return
				case <-time.After(time.Duration(attempt+1) * 2 * time.Second):
				}
			}
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				return
			}
			parts[i] = b
			atomic.AddInt64(&done, int64(len(b)))
			mu.Lock()
			if onBytes != nil {
				onBytes(atomic.LoadInt64(&done), total)
			}
			mu.Unlock()
		}(i, s, e)
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	var buf bytes.Buffer
	for _, p := range parts {
		buf.Write(p)
	}
	if int64(buf.Len()) != total {
		return nil, fmt.Errorf("子树大小不一致：%d/%d", buf.Len(), total)
	}
	return buf.Bytes(), nil
}
