package command

import (
	"flag"
	"strings"
	"time"
)

func parseServeArgs(args []string) (ServeArgs, error) {
	set := flag.NewFlagSet("serve", flag.ContinueOnError)
	listen := set.String("listen", "", "address on which to listen for remote connections. If ommitted, defaults to stdio. If prefixed by 'unix;', the subsequent address is assumed to be a unix domain socket, otherwise, TCP is used")
	timeout := set.Duration("listen.timeout", 0, "when used with -listen, shut down the server when there are no connected clients for this duration")
	gopls := set.String("gopls", "gopls", "path to gopls")
	err := set.Parse(args)
	if err != nil {
		return ServeArgs{}, err
	}
	network, address := parseListenArg(*listen)
	return ServeArgs{
		network: network,
		address: address,
		timeout: *timeout,
		gopls:   *gopls,
	}, err
}

func parseListenArg(listen string) (string, string) {
	if listen == "" {
		return "", ""
	}
	if parts := strings.SplitN(listen, ";", 2); len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "tcp", listen
}

type ServeArgs struct {
	network string
	address string
	timeout time.Duration
	gopls   string
}

func (a ServeArgs) Gopls() string {
	if a.gopls == "" {
		return "gopls"
	}
	return a.gopls
}

func (a ServeArgs) Timeout() time.Duration {
	if a.network == "" {
		return 0
	}
	return a.timeout
}

func (a ServeArgs) Socket() (string, string, bool) {
	if a.network == "" {
		return "", "", false
	}
	return a.network, a.address, true
}
