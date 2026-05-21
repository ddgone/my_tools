//go:build windows || darwin || (linux && cgo)

package tui

import (
	"bytes"
	_ "embed"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"sync"

	"github.com/ebitengine/oto/v3"
	"github.com/jfreymuth/oggvorbis"
)

//go:embed bgm.ogg
var bgmBytes []byte

var bgmContextOnce sync.Once
var bgmContext *oto.Context
var bgmContextErr error

func getBGMContext(sampleRate int, channelCount int) (*oto.Context, error) {
	bgmContextOnce.Do(func() {
		op := &oto.NewContextOptions{
			SampleRate:   sampleRate,
			ChannelCount: channelCount,
			Format:       oto.FormatFloat32LE,
		}
		var ready chan struct{}
		bgmContext, ready, bgmContextErr = oto.NewContext(op)
		if bgmContextErr != nil {
			return
		}
		<-ready
	})
	return bgmContext, bgmContextErr
}

type loopingOggReader struct {
	data []byte
	dec  *oggvorbis.Reader
}

func newLoopingOggReader(data []byte) (*loopingOggReader, error) {
	dec, err := oggvorbis.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return &loopingOggReader{data: data, dec: dec}, nil
}

func (l *loopingOggReader) Read(buf []byte) (int, error) {
	// buf expects bytes, we need to fill it with float32 little-endian
	// buf size should be a multiple of 4 bytes (float32 size)
	samplesToRead := len(buf) / 4
	if samplesToRead == 0 {
		return 0, nil
	}

	floatBuf := make([]float32, samplesToRead)
	n, err := l.dec.Read(floatBuf)

	if err == io.EOF || n == 0 {
		_ = l.dec.SetPosition(0)
		n, err = l.dec.Read(floatBuf)
	}

	for i := 0; i < n; i++ {
		bits := math.Float32bits(floatBuf[i])
		binary.LittleEndian.PutUint32(buf[i*4:(i+1)*4], bits)
	}

	return n * 4, err
}

type bgmPlayer struct {
	player *oto.Player
	mu     sync.Mutex
}

func newBGMPlayer() (*bgmPlayer, error) {
	lr, err := newLoopingOggReader(bgmBytes)
	if err != nil {
		return nil, fmt.Errorf("解析内置 OGG 失败: %w", err)
	}

	ctx, err := getBGMContext(lr.dec.SampleRate(), lr.dec.Channels())
	if err != nil {
		return nil, fmt.Errorf("音频初始化失败: %w", err)
	}

	p := ctx.NewPlayer(lr)
	p.SetBufferSize(8192)
	p.Play()

	return &bgmPlayer{
		player: p,
	}, nil
}

func (b *bgmPlayer) stop() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.player != nil {
		b.player.Pause()
	}
}
