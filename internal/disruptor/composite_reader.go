package disruptor

import "sync"

type compositeReader []Reader

func (r compositeReader) Read() {
	var waiter sync.WaitGroup
	waiter.Add(len(r))

	for _, item := range r {
		go func(reader Reader) {
			reader.Read()
			waiter.Done()
		}(item)
	}

	waiter.Wait()
}

func (r compositeReader) Close() error {
	for _, item := range r {
		_ = item.Close()
	}

	return nil
}
