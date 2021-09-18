package main

import (
	"context"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"

	"github.com/fsnotify/fsnotify"
	"github.com/k0kubun/pp"
)

type Watcher struct {
	directories      []string
	exclue           []string
	operationToWatch fsnotify.Op // fsnotify.Write | fsnotify.Remove
	done             chan bool

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
				log.Println("event:", event)
				if event.Op&fsnotify.Write == fsnotify.Write {
					log.Println("modified file:", event.Name)
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
		err := cmd.Start()
		if err != nil {
			pp.Println(err)
			// fmt.Println("run error:", err.Error(), cmd.String())
			// fmt.Println(cmd.ProcessState.SysUsage().(*syscall.Rusage).Maxrss)
			// if !errors.Is(err, context.Canceled) {
			// 	pp.Println(err)
			// }
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

func start(done <-chan bool) {
	r := NewRunnerWithDefault(os.Args[1], os.Args[2:]...)
	r.Run()
	fileChanged := make(chan bool)
	w := NewWatcher(".")
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
	// time.Sleep(5 * time.Second)
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
	done := make(chan bool)
	go start(done)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c // wait for user to press CTRL+C
	done <- true
}
