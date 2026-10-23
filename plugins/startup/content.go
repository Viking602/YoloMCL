package startup

import "sync"

type preparedContent[C any] struct {
	once    sync.Once
	prepare func()
	attach  func(*C)
}

func prepareContent[C any](prepare func(), attach func(*C)) *preparedContent[C] {
	return &preparedContent[C]{prepare: prepare, attach: attach}
}

func (p *preparedContent[C]) AttachContent(conn *C) {
	p.once.Do(p.prepare)
	p.attach(conn)
}
