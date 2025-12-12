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

type Proxy interface {
	Send(job Job, p Printer) (done bool, err error)
}

type Printer interface {
	Send(job Job) error
}

type printer struct {
	w io.Writer
}

func (p *printer) Send(job Job) error {
	if job.Context().Err() != nil {
		return job.Context().Err()
	}
	return job.Output(p.w)
}

func NewPrinter(w io.Writer) Printer {
	return &printer{w: w}
}

func newProxyManager(target Printer) *proxyManager {
	return &proxyManager{
		target:  target,
		proxies: []*proxyPrinter{},
	}
}

type proxyManager struct {
	proxies []*proxyPrinter
	target  Printer
}

func (p *proxyManager) Add(proxy Proxy) {
	p.proxies = append(p.proxies, &proxyPrinter{
		manager: p,
		proxy:   proxy,
	})
}

func (p *proxyManager) Send(job Job) error {
	if len(p.proxies) == 0 {
		return p.target.Send(job)
	}
	return p.sendTo(len(p.proxies)-1, job)
}

func (p *proxyManager) sendTo(index int, j Job) error {
	done, err := p.proxies[index].send(j)
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
	manager *proxyManager
	proxy   Proxy
}

func (p *proxyPrinter) send(j Job) (bool, error) {
	return p.proxy.Send(j, p)
}

func (p *proxyPrinter) Send(job Job) error {
	return p.manager.sendFrom(p, job)
}
