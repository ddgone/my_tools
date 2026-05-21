package tui

import (
	"encoding/binary"
	"fmt"
	"math"
	"sync"

	"github.com/ebitengine/oto/v3"
)

const (
	bgmSampleRate   = 44100
	bgmChannelCount = 1
	bgmBPM          = 150
)

var bgmContextOnce sync.Once
var bgmContext *oto.Context
var bgmContextErr error

func getBGMContext() (*oto.Context, error) {
	bgmContextOnce.Do(func() {
		op := &oto.NewContextOptions{
			SampleRate:   bgmSampleRate,
			ChannelCount: bgmChannelCount,
			Format:       oto.FormatSignedInt16LE,
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

type bgmNote struct {
	freq     float64
	duration int
}

const bgmBeatDuration = 60 * bgmSampleRate / bgmBPM

type melodyReader struct {
	notes    []bgmNote
	totalLen int64
	pos      int64
}

func newMelodyReader(notes []bgmNote) *melodyReader {
	total := int64(0)
	for _, n := range notes {
		total += int64(n.duration)
	}
	return &melodyReader{
		notes:    notes,
		totalLen: total,
	}
}

func squareWave(phase float64, duty float64) float64 {
	if phase < duty {
		return 1.0
	}
	return -1.0
}

func triangleWave(phase float64) float64 {
	if phase < 0.5 {
		return 4.0*phase - 1.0
	}
	return 3.0 - 4.0*phase
}

func (m *melodyReader) getNoteFreq(pos int64) (float64, float64) {
	off := pos % m.totalLen
	noteStart := int64(0)
	for _, n := range m.notes {
		if off < noteStart+int64(n.duration) {
			return n.freq, float64(off-noteStart) * n.freq / float64(bgmSampleRate)
		}
		noteStart += int64(n.duration)
	}
	return 0, 0
}

func (m *melodyReader) Read(buf []byte) (int, error) {
	num := int(2 * bgmChannelCount)
	n := len(buf) / num * num

	for i := 0; i < n; i += num {
		globalPos := m.pos + int64(i/num)

		melodyFreq, melodyPhase := m.getNoteFreq(globalPos)

		delay := int64(bgmBeatDuration / 4)
		bassFreq, bassPhase := m.getNoteFreq(globalPos - delay)
		bassFreq /= 2.0

		var melodyVal float64
		if melodyFreq > 0 {
			melodyVal = squareWave(melodyPhase-math.Floor(melodyPhase), 0.25) * 0.18
		}
		var bassVal float64
		if bassFreq > 0 {
			bassVal = triangleWave(bassPhase-math.Floor(bassPhase)) * 0.12
		}

		sample := int16((melodyVal + bassVal) * 32767)

		binary.LittleEndian.PutUint16(buf[i:i+2], uint16(sample))
	}

	m.pos += int64(n / num)
	return n, nil
}

type bgmPlayer struct {
	player *oto.Player
	mu     sync.Mutex
}

func newBGMPlayer() (*bgmPlayer, error) {
	ctx, err := getBGMContext()
	if err != nil {
		return nil, fmt.Errorf("音频初始化失败: %w", err)
	}

	notes := buildMelody()
	reader := newMelodyReader(notes)

	p := ctx.NewPlayer(reader)
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

func buildMelody() []bgmNote {
	b := func(beats float64) int {
		return int(float64(bgmBeatDuration) * beats)
	}

	return []bgmNote{
		{freq: 261.63, duration: b(0.25)},
		{freq: 329.63, duration: b(0.25)},
		{freq: 392.00, duration: b(0.25)},
		{freq: 523.25, duration: b(0.25)},
		{freq: 0, duration: b(0.25)},
		{freq: 523.25, duration: b(0.25)},
		{freq: 392.00, duration: b(0.25)},
		{freq: 329.63, duration: b(0.25)},

		{freq: 293.66, duration: b(0.25)},
		{freq: 349.23, duration: b(0.25)},
		{freq: 440.00, duration: b(0.25)},
		{freq: 587.33, duration: b(0.25)},
		{freq: 0, duration: b(0.25)},
		{freq: 587.33, duration: b(0.25)},
		{freq: 440.00, duration: b(0.25)},
		{freq: 349.23, duration: b(0.25)},

		{freq: 329.63, duration: b(0.25)},
		{freq: 392.00, duration: b(0.25)},
		{freq: 493.88, duration: b(0.25)},
		{freq: 659.25, duration: b(0.25)},
		{freq: 0, duration: b(0.25)},
		{freq: 659.25, duration: b(0.25)},
		{freq: 493.88, duration: b(0.25)},
		{freq: 392.00, duration: b(0.25)},

		{freq: 523.25, duration: b(0.25)},
		{freq: 659.25, duration: b(0.25)},
		{freq: 784.00, duration: b(0.25)},
		{freq: 1046.50, duration: b(0.50)},
		{freq: 784.00, duration: b(0.25)},
		{freq: 659.25, duration: b(0.25)},
		{freq: 523.25, duration: b(0.50)},

		{freq: 440.00, duration: b(0.25)},
		{freq: 523.25, duration: b(0.25)},
		{freq: 659.25, duration: b(0.25)},
		{freq: 880.00, duration: b(0.25)},
		{freq: 0, duration: b(0.25)},
		{freq: 784.00, duration: b(0.25)},
		{freq: 659.25, duration: b(0.25)},
		{freq: 523.25, duration: b(0.25)},

		{freq: 349.23, duration: b(0.25)},
		{freq: 440.00, duration: b(0.25)},
		{freq: 523.25, duration: b(0.25)},
		{freq: 698.46, duration: b(0.25)},
		{freq: 0, duration: b(0.25)},
		{freq: 659.25, duration: b(0.25)},
		{freq: 523.25, duration: b(0.25)},
		{freq: 440.00, duration: b(0.25)},

		{freq: 392.00, duration: b(0.25)},
		{freq: 493.88, duration: b(0.25)},
		{freq: 587.33, duration: b(0.25)},
		{freq: 784.00, duration: b(0.25)},
		{freq: 0, duration: b(0.25)},
		{freq: 698.46, duration: b(0.25)},
		{freq: 587.33, duration: b(0.25)},
		{freq: 493.88, duration: b(0.25)},

		{freq: 261.63, duration: b(0.25)},
		{freq: 329.63, duration: b(0.25)},
		{freq: 392.00, duration: b(0.25)},
		{freq: 523.25, duration: b(0.75)},
		{freq: 0, duration: b(0.25)},
		{freq: 329.63, duration: b(0.50)},
		{freq: 261.63, duration: b(0.50)},
	}
}
