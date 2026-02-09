package sshserver

/*
#include <libssh/libssh.h>
#include <libssh/server.h>
#include <libssh/callbacks.h>
#include <stdlib.h>

extern struct ssh_channel_callbacks_struct* vision3_new_channel_cb(void *userdata);
*/
import "C"
import (
	"log"
	"runtime/cgo"
	"unsafe"
)

//export go_auth_password_cb
func go_auth_password_cb(session C.ssh_session, user *C.char, password *C.char, userdata unsafe.Pointer) C.int {
	h := cgo.Handle(uintptr(userdata))
	cs := h.Value().(*connState)

	cs.username = C.GoString(user)
	_ = C.GoString(password) // Accept any password for now

	log.Printf("INFO: Password auth for user: %s", cs.username)
	return C.SSH_AUTH_SUCCESS
}

//export go_auth_none_cb
func go_auth_none_cb(session C.ssh_session, user *C.char, userdata unsafe.Pointer) C.int {
	h := cgo.Handle(uintptr(userdata))
	cs := h.Value().(*connState)

	cs.username = C.GoString(user)
	log.Printf("INFO: Auth none for user: %s", cs.username)
	return C.SSH_AUTH_SUCCESS
}

//export go_channel_open_cb
func go_channel_open_cb(session C.ssh_session, userdata unsafe.Pointer) C.ssh_channel {
	h := cgo.Handle(uintptr(userdata))
	cs := h.Value().(*connState)

	channel := C.ssh_channel_new(session)
	if channel == nil {
		log.Printf("ERROR: Failed to create SSH channel")
		return nil
	}

	cs.channel = channel

	// Set channel callbacks using the same handle for userdata
	chanCb := C.vision3_new_channel_cb(unsafe.Pointer(uintptr(cs.handle)))
	if chanCb == nil {
		log.Printf("ERROR: Failed to allocate channel callbacks")
		C.ssh_channel_free(channel)
		cs.channel = nil
		return nil
	}
	cs.chanCb = unsafe.Pointer(chanCb)

	C.ssh_set_channel_callbacks(channel, chanCb)

	log.Printf("INFO: Channel opened")
	return channel
}

//export go_channel_data_cb
func go_channel_data_cb(session C.ssh_session, channel C.ssh_channel, data unsafe.Pointer, length C.uint32_t, is_stderr C.int, userdata unsafe.Pointer) C.int {
	h := cgo.Handle(uintptr(userdata))
	cs := h.Value().(*connState)

	if length == 0 {
		return 0
	}

	dataCopy := C.GoBytes(data, C.int(length))

	select {
	case cs.readCh <- dataCopy:
		return C.int(length) // All bytes consumed
	default:
		return 0 // Buffer full, tell libssh to retry later
	}
}

//export go_channel_pty_request_cb
func go_channel_pty_request_cb(session C.ssh_session, channel C.ssh_channel, term *C.char, width C.int, height C.int, pxwidth C.int, pxheight C.int, userdata unsafe.Pointer) C.int {
	h := cgo.Handle(uintptr(userdata))
	cs := h.Value().(*connState)

	cs.pty = &PTYRequest{
		Term:   C.GoString(term),
		Width:  int(width),
		Height: int(height),
	}

	log.Printf("INFO: PTY request: term=%s, size=%dx%d", cs.pty.Term, cs.pty.Width, cs.pty.Height)
	return 0 // SSH_OK
}

//export go_channel_shell_request_cb
func go_channel_shell_request_cb(session C.ssh_session, channel C.ssh_channel, userdata unsafe.Pointer) C.int {
	h := cgo.Handle(uintptr(userdata))
	cs := h.Value().(*connState)

	log.Printf("INFO: Shell request for user: %s", cs.username)
	close(cs.shellReady)
	return 0 // SSH_OK
}

//export go_channel_pty_window_change_cb
func go_channel_pty_window_change_cb(session C.ssh_session, channel C.ssh_channel, width C.int, height C.int, pxwidth C.int, pxheight C.int, userdata unsafe.Pointer) C.int {
	h := cgo.Handle(uintptr(userdata))
	cs := h.Value().(*connState)

	w := Window{Width: int(width), Height: int(height)}
	log.Printf("INFO: Window change: %dx%d", w.Width, w.Height)

	select {
	case cs.winCh <- w:
	default:
		// Drop if channel full (window resize is best-effort)
	}
	return 0
}

//export go_channel_close_cb
func go_channel_close_cb(session C.ssh_session, channel C.ssh_channel, userdata unsafe.Pointer) {
	h := cgo.Handle(uintptr(userdata))
	cs := h.Value().(*connState)

	log.Printf("INFO: Channel close callback for user: %s", cs.username)
	cs.closer.Close()
}

//export go_channel_eof_cb
func go_channel_eof_cb(session C.ssh_session, channel C.ssh_channel, userdata unsafe.Pointer) {
	h := cgo.Handle(uintptr(userdata))
	cs := h.Value().(*connState)

	log.Printf("INFO: Channel EOF for user: %s", cs.username)
	cs.closer.Close()
}
