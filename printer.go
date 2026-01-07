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
	Init(p Printer)
	Terminate()
	Send(job Job) (done bool, err error)
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
		target: target,
	}
}

type proxyManager struct {
	proxies []*proxyPrinter
	target  Printer
}

func (p *proxyManager) Add(proxy Proxy) {
	printer := &proxyPrinter{
		manager: p,
		proxy:   proxy,
	}
	p.proxies = append(p.proxies, printer)
	printer.init()
}

func (p *proxyManager) terminate() {
	for _, proxy := range p.proxies {
		proxy.terminate()
	}
	p.proxies = nil
}

func (p *proxyManager) Send(job Job) error {
	if len(p.proxies) == 0 {
		return p.target.Send(job)
	}
	return p.sendTo(len(p.proxies)-1, job)
}

func (p *proxyManager) sendTo(index int, j Job) error {
	proxy := p.proxies[index]
	done, err := proxy.send(j)
	if done {
		p.proxies = slices.Delete(p.proxies, index, 1)
		proxy.terminate()
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

func (p *proxyPrinter) init() {
	p.proxy.Init(p)
}

func (p *proxyPrinter) terminate() {
	p.proxy.Terminate()
}

func (p *proxyPrinter) send(j Job) (bool, error) {
	return p.proxy.Send(j)
}

func (p *proxyPrinter) Send(job Job) error {
	return p.manager.sendFrom(p, job)
}
