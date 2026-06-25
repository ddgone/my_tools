package main

const (
	defaultWindowWidth  = 1280
	defaultWindowHeight = 800
	minVisibleWidth     = 80
	minVisibleHeight    = 60
)

type windowFrame struct {
	X      int
	Y      int
	Width  int
	Height int
}

type windowSnapshot struct {
	State     WindowState
	Minimised bool
}

func (f windowFrame) valid() bool {
	return f.Width > 0 && f.Height > 0
}

func frameFromState(state WindowState) windowFrame {
	return windowFrame{
		X:      state.X,
		Y:      state.Y,
		Width:  state.Width,
		Height: state.Height,
	}
}

func shouldRestoreSavedFrame(state WindowState, frame windowFrame) bool {
	if state.Maximised || state.Fullscreen {
		return false
	}
	return frame.valid() && isWindowRectVisible(frame.X, frame.Y, frame.Width, frame.Height)
}
