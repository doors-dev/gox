package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
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
	delay := 5 * time.Millisecond
	for s.ctx.Err() == nil {
		rwc, err := s.clientListener.Accept()
		if err != nil {
			if s.ctx.Err() != nil {
				return
			}
			var netErr net.Error
			if !errors.As(err, &netErr) || !netErr.Temporary() {
				slog.Error("server accept failed, no longer accepting connections: " + err.Error())
				return
			}
			slog.Error("server accept error: " + err.Error())
			select {
			case <-s.ctx.Done():
				return
			case <-time.After(delay):
			}
			if delay *= 2; delay > time.Second {
				delay = time.Second
			}
			continue
		}
		delay = 5 * time.Millisecond
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.connect(rwc)
		}()
	}
}

func (s *server) connect(clientRwc io.ReadWriteCloser) {
	if !s.onConnect() {
		clientRwc.Close()
		return
	}
	defer s.onDisconnect()
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	goplsRwc, err := s.goplsDialer.Dial(ctx)
	cancel()
	if err != nil {
		clientRwc.Close()
		slog.Error("server connect error: " + err.Error())
		return
	}
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

func (s *server) setKillTimer() bool {
	if s.killTimeout == 0 {
		return false
	}
	s.killTimer = time.AfterFunc(s.killTimeout, s.cancel)
	return true
}

func (s *server) onDisconnect() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.killCountDown -= 1
	if s.killCountDown == 0 {
		if s.setKillTimer() {
			return
		}
		s.cancel()
	}
}
