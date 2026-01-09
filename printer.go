package gox

import (
	"context"
	"errors"
	"io"
	"slices"
)

type Job interface {
	Context() context.Context
	Output(w io.Writer) error
}

type JobProvider interface {
	Job(ctx context.Context) Job
}

type ProxyProvider interface {
	Proxy(ctx context.Context, p Printer) Proxy
}

type Proxy interface {
	Send(j Job) (done bool, err error)
}

type Printer interface {
	Send(j Job) error
}

type printer struct {
	w io.Writer
}

func (p *printer) Send(j Job) error {
	if j.Context().Err() != nil {
		return j.Context().Err()
	}
	return j.Output(p.w)
}

func NewPrinter(w io.Writer) Printer {
	return &printer{w: w}
}

func newProxyManager(target Printer) *proxyManager {
	return &proxyManager{
		target: target,
	}
}

type proxyManager struct {
	proxies []*proxyPrinter
	target  Printer
}

func (p *proxyManager) Add(ctx context.Context, provider ProxyProvider) {
	printer := &proxyPrinter{
		manager:       p,
		proxyProvider: provider,
	}
	p.proxies = append(p.proxies, printer)
	if !printer.init(ctx) {
		p.proxies = p.proxies[:len(p.proxies)-1]
	}
}

func (p *proxyManager) terminate() {
	for _, proxy := range p.proxies {
		proxy.terminate()
	}
	p.proxies = nil
}

func (p *proxyManager) Send(j Job) error {
	if len(p.proxies) == 0 {
		return p.target.Send(j)
	}
	return p.sendTo(len(p.proxies)-1, j)
}

func (p *proxyManager) sendTo(index int, j Job) error {
	proxy := p.proxies[index]
	done, err := proxy.send(j)
	if done {
		p.proxies = slices.Delete(p.proxies, index, 1)
	}
	return err
}

func (p *proxyManager) sendFrom(from *proxyPrinter, j Job) error {
	index := slices.Index(p.proxies, from)
	if index == -1 {
		return errors.New("sender not found")
	}
	if index == 0 {
		return p.target.Send(j)
	}
	return p.sendTo(index-1, j)
}

type proxyPrinter struct {
	manager       *proxyManager
	proxyProvider ProxyProvider
	proxy         Proxy
}

func (p *proxyPrinter) init(ctx context.Context) bool {
	p.proxy = p.proxyProvider.Proxy(ctx, p)
	p.proxyProvider = nil
	return p.proxy != nil
}

func (p *proxyPrinter) terminate() {
	p.proxy.Send(nil)
	p.proxy = nil
}

func (p *proxyPrinter) send(j Job) (bool, error) {
	return p.proxy.Send(j)
}

func (p *proxyPrinter) Send(j Job) error {
	if p.proxy == nil {
		return errors.New("proxy is used after termination")
	}
	if j == nil {
		return nil
	}
	return p.manager.sendFrom(p, j)
}
