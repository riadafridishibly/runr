package main

import (
	"context"
	"errors"
	"io"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/k0kubun/pp"
)

var ()

func f() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	done := make(chan bool)
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				log.Println("event:", event)
				if event.Op&fsnotify.Write == fsnotify.Write {
					log.Println("modified file:", event.Name)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("error:", err)
			}
		}
	}()

	err = watcher.Add("/tmp/foo")
	if err != nil {
		log.Fatal(err)
	}
	<-done

}

type Runner struct {
	command string
	args    []string
	stdin   io.Reader
	stdout  io.Writer
	stderr  io.Writer

	// generated
	context.CancelFunc
}

func NewRunnerWithDefault(command string, args ...string) *Runner {
	return &Runner{
		command: command,
		args:    args,
		stdin:   os.Stdin,
		stdout:  os.Stdout,
		stderr:  os.Stderr,
	}
}

func (r *Runner) Run() {
	ctx, cancel := context.WithCancel(context.Background())
	r.CancelFunc = cancel

	cmd := exec.CommandContext(ctx, r.command, r.args...)
	cmd.Stderr = r.stderr
	cmd.Stdin = r.stdin
	cmd.Stdout = r.stdout

	go func() {
		err := cmd.Run()
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				pp.Println(err)
			}
		}
	}()
}

func (r *Runner) ReRun() {
	r.CancelFunc()
	r.Run()
}

func run(args ...string) (*exec.Cmd, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, args[0], args[:1]...)

	return cmd, cancel
}

func runner(done chan bool, rerun chan bool) {
	cmd, cancel := run("hello", "world")
	go cmd.Run() // TODO: fix me
	for {
		select {
		case <-done:
			return
		case <-rerun:
			cancel()
			cmd, cancel = run("hello", "world")
			go cmd.Run()
		}
	}
}

func main() {
	r := NewRunnerWithDefault("echo", "Hello", "World")
	r.Run()
	time.Sleep(1 * time.Second)
	r.ReRun()
	time.Sleep(5 * time.Second)
	// app := &cli.App{
	// 	Name:  "runr",
	// 	Usage: "run any command when file changes",
	// 	Flags: []cli.Flag{
	// 		&cli.StringSliceFlag{
	// 			Name:    "ignore",
	// 			Aliases: []string{"i"},
	// 		},
	// 	},
	// 	Action: func(c *cli.Context) error {
	// 		fmt.Println(c.StringSlice("ignore"))
	// 		fmt.Println(c.Args())
	// 		return nil
	// 	},
	// }
	// app.Run(os.Args)
}
