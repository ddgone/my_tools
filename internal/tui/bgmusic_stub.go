//go:build !windows && !darwin && !(linux && cgo)

package tui

type bgmPlayer struct {
}

func newBGMPlayer() (*bgmPlayer, error) {
	// 在没有 CGO 的 Linux 环境下，不提供背景音乐功能
	return &bgmPlayer{}, nil
}

func (b *bgmPlayer) stop() {
	// 空实现
}
