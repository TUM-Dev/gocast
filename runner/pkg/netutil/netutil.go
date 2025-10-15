package netutil

import (
	"log/slog"
	"net"
	"os"
)

var logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
	Level: slog.LevelDebug,
})).With("service", "bot")

// GetFreePort returns a free port for tcp use.
func GetFreePort() (port int, err error) {
	var a *net.TCPAddr
	if a, err = net.ResolveTCPAddr("tcp", "localhost:0"); err == nil {
		var l *net.TCPListener
		if l, err = net.ListenTCP("tcp", a); err == nil {
			defer func(l *net.TCPListener) {
				err := l.Close()
				if err != nil {
					logger.Error("failed to close listener: %v", "err", err)
				}
			}(l)
			return l.Addr().(*net.TCPAddr).Port, nil
		}
	}
	return port, err
}
