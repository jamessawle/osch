package engineer

// maxCapturedBytes caps how much subprocess output is held in memory. Only the
// tail is ever consumed downstream (via lastTail), so retaining the entire
// stream — which can run into many MB for long claude runs — wastes memory.
const maxCapturedBytes = 256 * 1024

// tailBuffer is an io.Writer that retains only the last maxCapturedBytes of
// what was written. Safe for single-goroutine writers (the exec package
// serialises writes per stream).
type tailBuffer struct {
	data []byte
	max  int
}

func newTailBuffer() *tailBuffer { return &tailBuffer{max: maxCapturedBytes} }

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.data = append(t.data, p...)
	if len(t.data) > t.max {
		t.data = append(t.data[:0], t.data[len(t.data)-t.max:]...)
	}
	return len(p), nil
}

func (t *tailBuffer) String() string { return string(t.data) }
