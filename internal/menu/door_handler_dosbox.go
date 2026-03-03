package menu

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
	"time"
)

// dosboxConfTemplate is the built-in DOSBox-X config used when no base config is provided.
// {{.Port}} is replaced with the TCP port DOSBox-X should connect to on COM1.
// The [autoexec] section mounts drive C and the per-node dropfile directory (drive D),
// then runs EXTERNAL.BAT which contains the door commands.
var dosboxConfTemplate = template.Must(template.New("dosbox").Parse(`[sdl]
output=none

[dosbox]
title=Vision3 BBS

[cpu]
core=auto
cputype=486

[mixer]
nosound=true

[midi]
mpu401=none
mididevice=none

[sblaster]
sbtype=none

[gus]
gus=false

[speaker]
pcspeaker=false

[joystick]
joysticktype=none

[serial]
serial1=nullmodem server:127.0.0.1 port:{{.Port}} transparent:1
serial2=disabled
serial3=disabled
serial4=disabled

[autoexec]
MOUNT C "{{.DriveCPath}}"
MOUNT D "{{.NodePath}}"
C:
CALL D:\EXTERNAL.BAT
EXIT
`))

type dosboxConfData struct {
	Port       int
	DriveCPath string
	NodePath   string
}

// escapeDOSBoxPath sanitizes a filesystem path for use in a DOSBox config file.
// It strips control characters and double-quote characters to prevent breaking
// the quoted MOUNT arguments in the [autoexec] section.
func escapeDOSBoxPath(p string) string {
	var b strings.Builder
	for _, r := range p {
		if r < 0x20 || r == '"' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// resolveDOSEmulator returns "dosemu" or "dosbox" based on the door config,
// the current platform, and what is installed.
// "dosemu" is preferred on linux/amd64 and linux/386 when it is available.
// All other platforms default to "dosbox".
func resolveDOSEmulator(ctx *DoorCtx) string {
	switch strings.ToLower(ctx.Config.DOSEmulator) {
	case "dosemu":
		return "dosemu"
	case "dosbox":
		return "dosbox"
	}
	// "auto" or empty: prefer dosemu on x86 Linux, dosbox everywhere else
	if runtime.GOOS == "linux" && (runtime.GOARCH == "amd64" || runtime.GOARCH == "386") {
		dosemuPath := ctx.Config.DosemuPath
		if dosemuPath == "" {
			dosemuPath = "/usr/bin/dosemu"
		}
		if _, err := exec.LookPath(dosemuPath); err == nil {
			return "dosemu"
		}
	}
	return "dosbox"
}

// findDOSBoxBinary returns the path to the DOSBox-X binary.
// It checks the configured path first, then looks for "dosbox-x" and "dosbox" in $PATH.
func findDOSBoxBinary(configured string) (string, error) {
	if configured != "" {
		if path, err := exec.LookPath(configured); err == nil {
			return path, nil
		}
		return "", fmt.Errorf("configured dosbox binary %q not found", configured)
	}
	for _, name := range []string{"dosbox-x", "dosbox"} {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	return "", errors.New("no DOSBox binary found; install dosbox-x or dosbox, or set dosbox_path in the door config")
}

// writeDOSBoxBatch writes EXTERNAL.BAT for DOSBox-X execution.
// Unlike the dosemu batch, it uses D: for the dropfile directory (already mounted by
// the [autoexec] section) and ends with EXIT rather than exitemu.
func writeDOSBoxBatch(ctx *DoorCtx, batchPath string) error {
	log.Printf("INFO: Writing DOSBox batch file: %s", batchPath)
	crlf := "\r\n"

	var b strings.Builder
	b.WriteString("@echo off" + crlf)
	b.WriteString("C:" + crlf)

	for _, cmd := range ctx.Config.DOSCommands {
		processed := strings.ReplaceAll(cmd, "{NODE}", ctx.NodeNumStr)
		for key, val := range ctx.Subs {
			processed = strings.ReplaceAll(processed, key, val)
		}
		b.WriteString(processed + crlf)
	}

	b.WriteString("EXIT" + crlf)

	return os.WriteFile(batchPath, []byte(b.String()), 0600)
}

// writeDOSBoxNodeConf generates a per-node dosbox-x.conf from either a provided base
// config file or the built-in template, substituting {{.Port}}, {{.DriveCPath}}, and
// {{.NodePath}}.
func writeDOSBoxNodeConf(confPath string, port int, driveCPath, nodePath, baseConf string) error {
	data := dosboxConfData{
		Port:       port,
		DriveCPath: escapeDOSBoxPath(driveCPath),
		NodePath:   escapeDOSBoxPath(nodePath),
	}

	if baseConf != "" {
		// User supplied a base template — parse and execute it
		raw, err := os.ReadFile(baseConf)
		if err != nil {
			return fmt.Errorf("failed to read base dosbox config %q: %w", baseConf, err)
		}
		tmpl, err := template.New("dosbox-base").Parse(string(raw))
		if err != nil {
			return fmt.Errorf("failed to parse base dosbox config %q: %w", baseConf, err)
		}
		f, err := os.OpenFile(confPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
		if err != nil {
			return err
		}
		defer f.Close()
		return tmpl.Execute(f, data)
	}

	f, err := os.OpenFile(confPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	return dosboxConfTemplate.Execute(f, data)
}

// executeDOSBoxDoor launches a DOS door via DOSBox-X.
//
// I/O path: DOSBox-X serial1=nullmodem connects to a TCP loopback socket opened by Go.
// Go bridges that TCP connection directly to the SSH session, passing raw CP437 bytes
// in both directions — same semantics as the dosemu PTY bridge, without requiring a PTY.
func executeDOSBoxDoor(ctx *DoorCtx) error {
	dosboxBin, err := findDOSBoxBinary(ctx.Config.DOSBoxPath)
	if err != nil {
		return err
	}

	// Resolve drive_c path
	driveCPath := ctx.Config.DriveCPath
	if driveCPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		driveCPath = filepath.Join(homeDir, ".dosemu", "drive_c")
	}

	// Create per-node temp directory (same layout as dosemu: drive_c/nodes/temp{N}/)
	nodeDir := fmt.Sprintf("temp%d", ctx.NodeNumber)
	nodePath := filepath.Join(driveCPath, "nodes", nodeDir)
	if err := os.MkdirAll(nodePath, 0700); err != nil {
		return fmt.Errorf("failed to create node directory %s: %w", nodePath, err)
	}

	// Generate all standard dropfiles
	if err := generateAllDropfiles(ctx, nodePath); err != nil {
		return fmt.Errorf("failed to generate dropfiles: %w", err)
	}
	defer cleanupDropfiles(nodePath)

	// Write EXTERNAL.BAT
	batchPath := filepath.Join(nodePath, "EXTERNAL.BAT")
	if err := writeDOSBoxBatch(ctx, batchPath); err != nil {
		return fmt.Errorf("failed to write batch file: %w", err)
	}

	// Bind a TCP listener on a free loopback port.
	// We keep the listener open so the port stays reserved until DOSBox-X connects.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("failed to open TCP listener for DOSBox-X serial: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	defer listener.Close()

	// Write the per-node DOSBox-X config
	confPath := filepath.Join(nodePath, fmt.Sprintf("dosbox-node%d.conf", ctx.NodeNumber))
	if err := writeDOSBoxNodeConf(confPath, port, driveCPath, nodePath, ctx.Config.DOSBoxConfig); err != nil {
		return fmt.Errorf("failed to write dosbox config: %w", err)
	}
	defer os.Remove(confPath)

	// Build the DOSBox-X command
	args := []string{"-silent", "-conf", confPath}
	log.Printf("INFO: Node %d: Launching DOS door %q via DOSBox-X: %s %v (serial port %d)",
		ctx.NodeNumber, ctx.DoorName, dosboxBin, args, port)

	cmd := exec.Command(dosboxBin, args...)
	cmd.Dir = driveCPath
	cmd.Env = os.Environ()
	// On non-Windows, suppress any SDL display requirement
	if runtime.GOOS != "windows" {
		cmd.Env = append(cmd.Env, "SDL_VIDEODRIVER=dummy", "SDL_AUDIODRIVER=dummy")
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start DOSBox-X: %w", err)
	}

	// Accept the serial connection from DOSBox-X.
	// Use a channel so we can handle DOSBox-X dying before it connects.
	// Race against: (1) Accept, (2) timeout, (3) DOSBox-X exiting before connecting.
	const acceptTimeout = 60 * time.Second
	type acceptResult struct {
		conn net.Conn
		err  error
	}
	acceptCh := make(chan acceptResult, 1)
	go func() {
		conn, err := listener.Accept()
		acceptCh <- acceptResult{conn, err}
	}()

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	var conn net.Conn
	select {
	case res := <-acceptCh:
		if res.err != nil {
			cmd.Process.Kill() //nolint:errcheck
			<-waitCh
			return fmt.Errorf("DOSBox-X serial connection failed: %w", res.err)
		}
		conn = res.conn
		listener.Close() // port no longer needed; single connection expected
	case <-time.After(acceptTimeout):
		cmd.Process.Kill() //nolint:errcheck
		<-waitCh
		listener.Close()
		return fmt.Errorf("timeout waiting for DOSBox-X to connect (%v)", acceptTimeout)
	case waitErr := <-waitCh:
		listener.Close()
		return fmt.Errorf("DOSBox-X exited before connecting: %w", waitErr)
	}
	defer conn.Close()

	// Set up read interrupt for clean shutdown (SSH sessions support this)
	readInterrupt := make(chan struct{})
	hasInterrupt := false
	if ri, ok := ctx.Session.(interface{ SetReadInterrupt(<-chan struct{}) }); ok {
		ri.SetReadInterrupt(readInterrupt)
		defer ri.SetReadInterrupt(nil)
		hasInterrupt = true
	}

	// Bidirectional bridge: SSH session ↔ TCP connection (raw CP437 bytes)
	inputDone := make(chan struct{})
	outputDone := make(chan struct{})
	go func() {
		defer close(inputDone)
		_, err := io.Copy(conn, ctx.Session)
		if err != nil && err != io.EOF && !errors.Is(err, net.ErrClosed) {
			log.Printf("WARN: Node %d: Error copying session to DOSBox-X socket: %v", ctx.NodeNumber, err)
		}
	}()
	go func() {
		defer close(outputDone)
		_, err := io.Copy(ctx.Session, conn)
		if err != nil && err != io.EOF && !errors.Is(err, net.ErrClosed) {
			log.Printf("WARN: Node %d: Error copying DOSBox-X socket to session: %v", ctx.NodeNumber, err)
		}
	}()

	// Wait for DOSBox-X to exit, then clean up I/O goroutines
	cmdErr := <-waitCh
	log.Printf("DEBUG: Node %d: DOSBox-X process exited for door %q", ctx.NodeNumber, ctx.DoorName)

	close(readInterrupt)
	if hasInterrupt {
		<-inputDone
	}
	conn.Close()
	<-outputDone

	if cmdErr != nil {
		log.Printf("ERROR: Node %d: DOS door %q (DOSBox-X) failed: %v", ctx.NodeNumber, ctx.DoorName, cmdErr)
		return cmdErr
	}

	log.Printf("INFO: Node %d: DOS door %q completed successfully (DOSBox-X)", ctx.NodeNumber, ctx.DoorName)
	return nil
}