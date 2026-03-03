//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris || aix || zos
// +build darwin dragonfly freebsd linux netbsd openbsd solaris aix zos

package tea

import (
	"os"
	"os/signal"
	"syscall"
)

// listenForResize listens for SIGWINCH and triggers a resize check.
// It exits when the program context is canceled and then closes done.
func (p *Program) listenForResize(done chan struct{}) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGWINCH)

	defer func() {
		signal.Stop(sig)
		close(done)
	}()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-sig:
			p.checkResize()
		}
	}
}
