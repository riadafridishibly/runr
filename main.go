package main

import (
	"context"
	"errors"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"

	"github.com/fsnotify/fsnotify"
	"github.com/urfave/cli"
)

type Watcher struct {
	directories []string
	done        chan bool

	*fsnotify.Watcher
}

func NewWatcher(dirs ...string) *Watcher {
	return &Watcher{
		directories: dirs,
		done:        make(chan bool),
	}
}

func (w *Watcher) Watch(changed chan<- bool) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}

	w.Watcher = watcher

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}

				log.Println("change detected in:", event.Name)
				if event.Op&fsnotify.Write == fsnotify.Write {
					changed <- true
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("error:", err)
			case <-w.done:
				w.Watcher.Close()
				return
			}
		}
	}()

	for _, dir := range w.directories {
		err = watcher.Add(dir)
		if err != nil {
			log.Fatal(err)
		}
	}
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
	ctx, cancel := context.WithCancel(context.TODO())
	r.CancelFunc = cancel
	cmd := exec.CommandContext(ctx, r.command, r.args...)
	cmd.Stderr = r.stderr
	cmd.Stdin = r.stdin
	cmd.Stdout = r.stdout

	go func() {
		log.Println("running: ", cmd.String())
		err := cmd.Start()
		if err != nil {
			if errors.Is(err, context.Canceled) {
				log.Println("terminated current process due to change in file")
			}
		}
	}()
}

func (r *Runner) ReRun() {
	r.CancelFunc()
	r.Run()
}

func (r *Runner) Exit() {
	r.CancelFunc()
}

func start(done <-chan bool, dirs []string, cmds []string) {
	r := NewRunnerWithDefault(cmds[0], cmds[1:]...)
	r.Run()
	fileChanged := make(chan bool)
	w := NewWatcher(dirs...)
	go w.Watch(fileChanged)

	for {
		select {
		case <-done:
			w.done <- true
			r.Exit()
			return
		case <-fileChanged:
			r.ReRun()
		}
	}
}

func main() {
	app := &cli.App{
		Name:      "runr",
		Usage:     "run any command when file changes",
		UsageText: "runr -watch [dir] cmd cmd-args...",
		Flags: []cli.Flag{
			&cli.StringSliceFlag{
				Name:  "watch",
				Usage: "directory to watch",
				Value: &cli.StringSlice{"."},
			},
		},
		Action: func(ctx *cli.Context) error {
			args := ctx.Args()
			if len(args) == 0 {
				log.Fatal("require commands to run")
			}

			done := make(chan bool)
			go start(done, ctx.StringSlice("watch"), ctx.Args())
			c := make(chan os.Signal, 1)
			signal.Notify(c, os.Interrupt)
			<-c // wait for user to press CTRL+C
			done <- true

			return nil
		},
	}
	app.Run(os.Args)
}
