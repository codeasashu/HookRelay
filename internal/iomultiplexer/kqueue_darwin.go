package iomultiplexer

import (
	"syscall"
    "log"
)

// KQueue implements the IOMultiplexer interface for Darwin-based systems
type KQueue struct {
	// fd stores the file descriptor of the kqueue instance
	fd int
	// kQEvents acts as a buffer for the events returned by the Kevent syscall
	kQEvents []syscall.Kevent_t

    EventQueue chan Event
}


// New creates a new KQueue instance
func New(maxClients int32) (*KQueue, error) {
	if maxClients < 0 {
		return nil, ErrInvalidMaxClients
	}

	fd, err := syscall.Kqueue()
	if err != nil {
		return nil, err
	}

	return &KQueue{
		fd:         fd,
		kQEvents:   make([]syscall.Kevent_t, maxClients),
        EventQueue: make(chan Event, maxClients),
	}, nil
}

func (kq *KQueue) AddFD(fd int) error {
    event := syscall.Kevent_t{
        Ident:  uint64(fd),
        Filter: EVFILT_READ,
        Flags:  syscall.EV_ADD | syscall.EV_ENABLE,
        Fflags: 0,
        Data:   0,
        Udata:  nil,
    }

    events := []syscall.Kevent_t{event}
    _, err := syscall.Kevent(kq.fd, events, nil, nil)
    return err
}

// Poll polls for all the subscribed events simultaneously
// and returns all the events that were triggered
// It blocks until at least one event is triggered or the timeout is reached
func (kq *KQueue) Poll() error {
    timeout := &syscall.Timespec{
        Sec:  0,
        Nsec: 0,
    }
	nEvents, err := syscall.Kevent(kq.fd, nil, kq.kQEvents, timeout)
	if err != nil {
		// return fmt.Errorf("kqueue poll: %w", err)
		return err
	}

	for i := 0; i < nEvents; i++ {
        evt := Event{
                Fd:     int(kq.kQEvents[i].Ident),
                Filter: kq.kQEvents[i].Filter,
                Data:   kq.kQEvents[i].Data,
        }
        select {
            case kq.EventQueue <- evt:
            default:
                log.Printf("Warning: job queue full, dropped event for fd %d", evt.Fd)
        }
	}

	// return kq.hookRelayEvents[:nEvents], nil
	return nil
}

// Close closes the KQueue instance
func (kq *KQueue) Close() error {
	return syscall.Close(kq.fd)
}
