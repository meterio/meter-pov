package consensus

import "fmt"

func (p *Pacemaker) onIncomingMsg(mi *msgParcel) {
	defer p.incomingWriteMutex.Unlock()
	p.incomingWriteMutex.Lock()
	// instead of drop the latest message, drop the oldest one in front of queue
	for len(p.incomingCh) >= cap(p.incomingCh) {
		dropped := <-p.incomingCh
		p.logger.Warn(fmt.Sprintf("dropped %s due to cap", dropped.Msg.String()), "from", dropped.Peer.NameAndIP())
	}

	// TODO: check if this caused a dead lock for putting message into a full channel
	p.logger.Info(fmt.Sprintf("recv %s", mi.Msg.String()), "from", mi.Peer.NameAndIP())
	p.incomingCh <- *mi
}

func (p *Pacemaker) drainIncomingMsg() {
	defer p.incomingWriteMutex.Unlock()
	p.incomingWriteMutex.Lock()
	for len(p.incomingCh) > 0 {
		<-p.incomingCh
	}
}
