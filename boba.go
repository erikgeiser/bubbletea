package boba

import (
	"io"
	"os"
	"strings"
	"time"

	"github.com/muesli/termenv"
)

// Msg represents an action and is usually the result of an IO operation. It's
// triggers the  Update function, and henceforth, the UI.
type Msg interface{}

// Model contains the program's state.
type Model interface{}

// Cmd is an IO operation that runs once. If it's nil it's considered a no-op.
type Cmd func() Msg

// Batch peforms a bunch of commands concurrently with no ordering guarantees
// about the results.
func Batch(cmds ...Cmd) Cmd {
	if len(cmds) == 0 {
		return nil
	}
	return func() Msg {
		return batchMsg(cmds)
	}
}

// Init is the first function that will be called. It returns your initial
// model and runs an optional command
type Init func() (Model, Cmd)

// Update is called when a message is received. It may update the model and/or
// send a command.
type Update func(Msg, Model) (Model, Cmd)

// View produces a string which will be rendered to the terminal
type View func(Model) string

// Program is a terminal user interface
type Program struct {
	init   Init
	update Update
	view   View
}

// Quit is a command that tells the program to exit
func Quit() Msg {
	return quitMsg{}
}

// Signals that the program should quit
type quitMsg struct{}

// batchMsg is used to perform a bunch of commands
type batchMsg []Cmd

// NewProgram creates a new Program
func NewProgram(init Init, update Update, view View) *Program {
	return &Program{
		init:   init,
		update: update,
		view:   view,
	}
}

// Start initializes the program
func (p *Program) Start() error {
	var (
		model         Model
		cmd           Cmd
		cmds          = make(chan Cmd)
		msgs          = make(chan Msg)
		errs          = make(chan error)
		done          = make(chan struct{})
		linesRendered int
	)

	err := initTerminal()
	if err != nil {
		return err
	}
	defer restoreTerminal()

	// Initialize program
	model, cmd = p.init()
	if cmd != nil {
		go func() {
			cmds <- cmd
		}()
	}

	// Render initial view
	linesRendered = p.render(model, linesRendered)

	// Subscribe to user input
	go func() {
		for {
			msg, err := ReadKey(os.Stdin)
			if err != nil {
				errs <- err
			}
			msgs <- KeyMsg(msg)
		}
	}()

	// Process commands
	go func() {
		for {
			select {
			case <-done:
				return
			case cmd := <-cmds:
				if cmd != nil {
					go func() {
						msgs <- cmd()
					}()
				}
			}
		}
	}()

	// Handle updates and draw
	for {
		select {
		case err := <-errs:
			close(done)
			return err
		case msg := <-msgs:

			// Handle quit message
			if _, ok := msg.(quitMsg); ok {
				close(done)
				return nil
			}

			// Process batch commands
			if batchedCmds, ok := msg.(batchMsg); ok {
				for _, cmd := range batchedCmds {
					cmds <- cmd
				}
				continue
			}

			model, cmd = p.update(msg, model)              // run update
			cmds <- cmd                                    // process command (if any)
			linesRendered = p.render(model, linesRendered) // render to terminal
		}
	}
}

// Render a view to the terminal. Returns the number of lines rendered.
func (p *Program) render(model Model, linesRendered int) int {
	view := p.view(model)

	// We need to add carriage returns to ensure that the cursor travels to the
	// start of a column after a newline
	view = strings.Replace(view, "\n", "\r\n", -1)

	if linesRendered > 0 {
		termenv.ClearLines(linesRendered)
	}
	_, _ = io.WriteString(os.Stdout, view)
	return strings.Count(view, "\r\n")
}

// AltScreen exits the altscreen. This is just a wrapper around the termenv
// function
func AltScreen() {
	termenv.AltScreen()
}

// ExitAltScreen exits the altscreen. This is just a wrapper around the termenv
// function
func ExitAltScreen() {
	termenv.ExitAltScreen()
}

type EveryMsg time.Time

// Every is a command that ticks in sync with the system clock. So, if you
// wanted to tick with the system clock every second, minute or hour you
// could use this. It's also handy for having different things tick in sync.
//
// Note that because we're ticking with the system clock the tick will likely
// not run for the entire specified duration. For example, if we're ticking for
// one minute and the clock is at 12:34:20 then the next tick will happen at
// 12:35:00, 40 seconds later.
func Every(duration time.Duration, fn func(time.Time) Msg) Cmd {
	return func() Msg {
		n := time.Now()
		d := n.Truncate(duration).Add(duration).Sub(n)
		t := time.NewTimer(d)
		select {
		case now := <-t.C:
			return fn(now)
		}
	}
}

// Tick is a subscription that at an interval independent of the system clock
// at the given duration. That is, the timer begins when precisely when invoked,
// and runs for its entire duration.
func Tick(d time.Duration, fn func(time.Time) Msg) Cmd {
	return func() Msg {
		t := time.NewTimer(d)
		select {
		case now := <-t.C:
			return fn(now)
		}
	}
}
