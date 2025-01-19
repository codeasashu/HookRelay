package iomultiplexer

import (
    "log"
	"syscall"
)

type Epoll struct {
	// fd stores the file descriptor of the epoll instance
	fd int
	// ePollEvents acts as a buffer for the events returned by the EpollWait syscall
	ePollEvents []syscall.EpollEvent

    EventQueue chan Event
}

// New creates a new Epoll instance
func New(maxClients int32) (*Epoll, error) {
	if maxClients < 0 {
		return nil, ErrInvalidMaxClients
	}

	fd, err := syscall.EpollCreate1(0)
	if err != nil {
		return nil, err
	}

	return &Epoll{
		fd:          fd,
		ePollEvents: make([]syscall.EpollEvent, maxClients),
        EventQueue: make(chan Event, maxClients),
	}, nil
}

func (ep *Epoll) AddFD(fd int) error {
    event := syscall.EpollEvent{
        Events: syscall.EPOLLIN,
        Fd:     int32(fd),
    }

    return syscall.EpollCtl(ep.fd, syscall.EPOLL_CTL_ADD, fd, &event)
}

// Poll polls for all the subscribed events simultaneously
// and returns all the events that were triggered
func (ep *Epoll) Poll() error {
	nEvents, err := syscall.EpollWait(ep.fd, ep.ePollEvents, 0)
	if err != nil {
		// return nil, fmt.Errorf("epoll poll: %w", err)
        return err
	}

	for i := 0; i < nEvents; i++ {
        evt := Event{
                Fd:     int(ep.ePollEvents[i].Fd),
                Filter: filterFromEpollEvents(ep.ePollEvents[i].Events),
                Data:   int64(ep.ePollEvents[i].Events),
        }
        select {
            case ep.EventQueue <- evt:
            default:
                log.Printf("Warning: job queue full, dropped event for fd %d", evt.Fd)
        }
	}

	// return ep.hookRelayEvents[:nEvents], nil
	return nil
}

// Close closes the Epoll instance
func (ep *Epoll) Close() error {
	return syscall.Close(ep.fd)
}

func filterFromEpollEvents(events uint32) int16 {
    if events&syscall.EPOLLIN != 0 {
        return EVFILT_READ
    }
    if events&syscall.EPOLLOUT != 0 {
        return EVFILT_WRITE
    }
    return 0
}
