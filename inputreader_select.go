//go:build solaris || darwin || freebsd || netbsd || openbsd || dragonfly
// +build solaris darwin freebsd netbsd openbsd dragonfly

// nolint:revive
package tea

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

// newSelectInputReader returns a cancelable reader. If the passed reader is an
// *os.File, the cancel method can be used to interrupt a blocking call read
// call. In this case, the cancel method returns true if the call was cancelled
// successfully. If the input reader is not a *os.File or the file descriptor
// is 1024 or larger, the cancel method does nothing and always returns false.
// The generic Unix implementation is based on the POSIX select syscall.
func newSelectInputReader(reader io.Reader) (inputReader, error) {
	file, ok := reader.(*os.File)
	if !ok || file.Fd() >= unix.FD_SETSIZE {
		return newFallbackInputReader(reader)
	}
	r := &selectInputReader{file: file}

	var err error

	r.cancelSignalReader, r.cancelSignalWriter, err = os.Pipe()
	if err != nil {
		return nil, err
	}

	return r, nil
}

type selectInputReader struct {
	file               *os.File
	cancelSignalReader *os.File
	cancelSignalWriter *os.File
	cancelMixin
}

func (r *selectInputReader) ReadInput() ([]Msg, error) {
	if r.isCancelled() {
		return nil, errCanceled
	}

	for {
		err := waitForRead(r.file, r.cancelSignalReader)
		if err != nil {
			if errors.Is(err, unix.EINTR) {
				continue // try again if the syscall was interrupted
			}

			if errors.Is(err, errCanceled) {
				// remove signal from pipe
				var b [1]byte
				_, readErr := r.cancelSignalReader.Read(b[:])
				if readErr != nil {
					return nil, fmt.Errorf("reading cancel signal: %w", readErr)
				}
			}

			return nil, err
		}

		msg, err := parseInputMsgFromReader(r.file)
		if err != nil {
			return nil, err
		}

		return []Msg{msg}, nil
	}
}

func (r *selectInputReader) Cancel() bool {
	r.setCancelled()

	// send cancel signal
	_, err := r.cancelSignalWriter.Write([]byte{'c'})
	return err == nil
}

func (r *selectInputReader) Close() error {
	var errMsgs []string

	// close pipe
	err := r.cancelSignalWriter.Close()
	if err != nil {
		errMsgs = append(errMsgs, fmt.Sprintf("closing cancel signal writer: %v", err))
	}

	err = r.cancelSignalReader.Close()
	if err != nil {
		errMsgs = append(errMsgs, fmt.Sprintf("closing cancel signal reader: %v", err))
	}

	if len(errMsgs) > 0 {
		return fmt.Errorf(strings.Join(errMsgs, ", "))
	}

	return nil
}

func waitForRead(reader *os.File, abort *os.File) error {
	readerFd := int(reader.Fd())
	abortFd := int(abort.Fd())

	maxFd := readerFd
	if abortFd > maxFd {
		maxFd = abortFd
	}

	// this is a limitation of the select syscall
	if maxFd >= unix.FD_SETSIZE {
		return fmt.Errorf("cannot select on file descriptor %d which is larger than 1024", maxFd)
	}

	fdSet := &unix.FdSet{}
	fdSet.Set(int(reader.Fd()))
	fdSet.Set(int(abort.Fd()))

	_, err := unix.Select(maxFd+1, fdSet, nil, nil, nil)
	if err != nil {
		return fmt.Errorf("select: %w", err)
	}

	if fdSet.IsSet(abortFd) {
		return errCanceled
	}

	if fdSet.IsSet(readerFd) {
		return nil
	}

	return fmt.Errorf("select returned without setting a file descriptor")
}
