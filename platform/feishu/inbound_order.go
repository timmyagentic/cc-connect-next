package feishu

// enqueueInboundDispatch reserves order before starting asynchronous work.
// Locking inside dispatchMessage is too late: a newer text goroutine can win
// the lock while an earlier image is still waiting to be scheduled. Ordering
// includes resource downloads and batch flushes, not the Agent's turn (the
// engine starts that asynchronously). Different sessions remain independent.
func (p *Platform) enqueueInboundDispatch(sessionKey string, dispatch func()) {
	p.inboundOrderMu.Lock()
	if p.inboundOrderTail == nil {
		p.inboundOrderTail = make(map[string]chan struct{})
	}
	previous := p.inboundOrderTail[sessionKey]
	done := make(chan struct{})
	p.inboundOrderTail[sessionKey] = done
	p.inboundOrderMu.Unlock()

	go func() {
		defer func() {
			p.inboundOrderMu.Lock()
			if p.inboundOrderTail[sessionKey] == done {
				delete(p.inboundOrderTail, sessionKey)
			}
			close(done)
			p.inboundOrderMu.Unlock()
		}()
		if previous != nil {
			<-previous
		}
		// Always run dispatch's cleanup, even if its context was cancelled:
		// thread bootstrap reservations must be released for later messages.
		dispatch()
	}()
}
