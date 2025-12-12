package server

import (
	"context"
	"io"
	"log/slog"
	"time"

	"sync"
)

func NewServer(clientListener Listener, goplsDialer Dialer, timeout time.Duration) Server {
	ctx, cancel := context.WithCancel(context.Background())
	s := &server{
		ctx:            ctx,
		cancel:         cancel,
		clientListener: clientListener,
		goplsDialer:    goplsDialer,
		killTimeout:    timeout,
	}
	s.launch()
	return s
}

type server struct {
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	clientListener Listener
	goplsDialer    Dialer
	mu             sync.Mutex
	killCountDown  int
	killTimeout    time.Duration
	killTimer      *time.Timer
}

func (s *server) Wait() {
	s.wg.Wait()
}

func (s *server) launch() {
	s.setKillTimer()
	s.launchAccept()
	go s.waitDone()
}

func (s *server) waitDone() {
	<-s.ctx.Done()
	s.clientListener.Close()
}

func (s *server) launchAccept() {
	s.wg.Add(1)
	go s.accept()
}

func (s *server) accept() {
	defer s.wg.Done()
	if s.ctx.Err() != nil {
		return
	}
	rwc, err := s.clientListener.Accept()
	s.launchAccept()
	if err != nil {
		return
	}
	s.connect(rwc)
}

func (s *server) connect(clientRwc io.ReadWriteCloser) {
	if !s.onConnect() {
		clientRwc.Close()
		return
	}
	defer s.onDisconnect()
	goplsRwc, err := s.goplsDialer.Dial(s.ctx)
	if err != nil {
		clientRwc.Close()
		slog.Error("server connect error: " + err.Error())
		return
	}
	slog.Info("Bridging client and gopls")
	bridge := newBridge(s.ctx, clientRwc, goplsRwc)
	bridge.run()
}

func (s *server) onConnect() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.killCountDown += 1
	if s.killTimer != nil {
		ok := s.killTimer.Stop()
		s.killTimer = nil
		return ok
	}
	return true
}

func (s *server) setKillTimer() {
	s.killTimer = time.AfterFunc(s.killTimeout, s.cancel)
}

func (s *server) onDisconnect() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.killCountDown -= 1
	if s.killCountDown == 0 {
		s.setKillTimer()
	}
}
