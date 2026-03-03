//go:build windows
// +build windows

package tea

import (
	"fmt"
	"io"
	"os"

	ansicon "github.com/bitbored/go-ansicon"
	"github.com/charmbracelet/x/term"
	"golang.org/x/sys/windows"
)

// vtWriter wraps the original console term.File, preserving Fd/Close/Read for
// terminal sizing and state queries, while delegating Write to an ANSI→Win32
// translator so escape sequences are rendered on consoles without VT support.
type vtWriter struct {
	term.File        // preserves Fd(), Close(), Read()
	writer io.Writer // ansicon translator
}

func (w *vtWriter) Write(p []byte) (int, error) {
	return w.writer.Write(p)
}

func (p *Program) initInput() (err error) {
	// Save stdin state and enable VT input
	// We also need to enable VT
	// input here.
	if f, ok := p.input.(term.File); ok && term.IsTerminal(f.Fd()) {
		p.ttyInput = f
		p.previousTtyInputState, err = term.MakeRaw(p.ttyInput.Fd())
		if err != nil {
			return fmt.Errorf("error making raw: %w", err)
		}

		// Enable VT input (best-effort: not available on Windows pre-1709 or some 32-bit builds).
		// BubbleTea reads input via coninput (raw Windows console events), so VT input mode
		// is not required for correct operation.
		var mode uint32
		if err := windows.GetConsoleMode(windows.Handle(p.ttyInput.Fd()), &mode); err == nil {
			_ = windows.SetConsoleMode(windows.Handle(p.ttyInput.Fd()), mode|windows.ENABLE_VIRTUAL_TERMINAL_INPUT)
		}
	}

	// Save output screen buffer state and enable VT processing.
	if f, ok := p.output.(term.File); ok && term.IsTerminal(f.Fd()) {
		p.ttyOutput = f
		p.previousOutputState, err = term.GetState(f.Fd())
		if err != nil {
			return fmt.Errorf("error getting state: %w", err)
		}

		// Enable VT output processing (best-effort: not available on Windows pre-1709
		// or some 32-bit builds). Without it the console will not render ANSI escape
		// sequences, but at least the program will not crash on startup.
		var mode uint32
		if err := windows.GetConsoleMode(windows.Handle(p.ttyOutput.Fd()), &mode); err == nil {
			_ = windows.SetConsoleMode(windows.Handle(p.ttyOutput.Fd()), mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
		}
	}

	return nil
}

// Open the Windows equivalent of a TTY.
func openInputTTY() (*os.File, error) {
	f, err := os.OpenFile("CONIN$", os.O_RDWR, 0o644) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("error opening file: %w", err)
	}
	return f, nil
}

// prepareOutput wraps p.output with an ANSI→Win32 API translator when the
// Windows console does not support ENABLE_VIRTUAL_TERMINAL_PROCESSING (e.g.
// Windows 10 pre-1709 or some 32-bit builds). The wrapped value is a vtWriter
// that still satisfies term.File so that initInput can correctly set
// p.ttyOutput and p.previousOutputState for resize events and shutdown.
// On consoles that do support VT processing this is a no-op — the mode is
// probed and immediately restored so that initInput can set it properly.
func (p *Program) prepareOutput() {
	f, ok := p.output.(term.File)
	if !ok {
		return
	}
	var mode uint32
	if err := windows.GetConsoleMode(windows.Handle(f.Fd()), &mode); err != nil {
		return
	}
	if windows.SetConsoleMode(windows.Handle(f.Fd()), mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING) != nil {
		// VT processing unsupported — wrap p.output with an ANSI→Win32 translator.
		// vtWriter embeds the original term.File so p.output still satisfies
		// term.File after wrapping; initInput can then correctly set p.ttyOutput
		// and p.previousOutputState via the normal type assertion.
		p.output = &vtWriter{File: f, writer: ansicon.Convert(f)}
		return
	}
	// VT supported: restore mode; initInput will set it again properly.
	_ = windows.SetConsoleMode(windows.Handle(f.Fd()), mode)
}

const suspendSupported = false

func suspendProcess() {}
