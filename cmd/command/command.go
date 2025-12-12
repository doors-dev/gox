package cmd

import (
	"errors"
	"flag"
	"os"
	"strings"
	"time"
)

type Starter interface {
	Default() error
	Serve(network string, address string, timeout time.Duration) error
}

func Execute(s Starter) (error, error) {
	args := os.Args[1:]
	command := ""
	if len(args) != 0 {
		command = args[0]
		args = args[1:]
	}
	switch command {
	case "":
		return nil, s.Default()
	case "serve":
		listen, timeout, err := parseServeArgs(args)
		if err != nil {
			return errors.New("serve arguments parse error: " + err.Error()), nil
		}
		network, address := parseListenArg(listen)
		return nil, s.Serve(network, address, timeout)
	default:
		return errors.New("unknown command: " + command), nil
	}
}

func parseServeArgs(args []string) (string, time.Duration, error) {
	set := flag.NewFlagSet("serve", flag.ContinueOnError)
	listen := set.String("listen", "", "address on which to listen for remote connections. If prefixed by 'unix;', the subsequent address is assumed to be a unix domain socket. Otherwise, TCP is used")
	timeout := set.Duration("listen.timeout", 0, "when used with -listen, shut down the server when there are no connected clients for this duration")
	err := set.Parse(args)
	return *listen, *timeout, err
}

func parseListenArg(listen string) (string, string) {
	if parts := strings.SplitN(listen, ";", 2); len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "tcp", listen
}
