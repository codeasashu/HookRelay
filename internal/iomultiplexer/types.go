package iomultiplexer

// Event is a platform independent representation of a subscribe event
// For linux platform, this is translated to syscall.EpollEvent
// For darwin platform, this is translated to syscall.Kevent_t
type Event struct {
	Fd int
    Filter int16
    Data   int64
}

// Common constants
const (
    EVFILT_READ  = -1
    EVFILT_WRITE = -2
)
