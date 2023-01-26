package main

import (
	"github.com/fsnotify/fsnotify"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

func stop(cmd *exec.Cmd) {
	pid := cmd.Process.Pid
	log.Println(pid, "dead")
	pgid, err := syscall.Getpgid(pid)
	if err == nil {
		syscall.Kill(-pgid, 15)
	}
	cmd.Wait()
}

func watch(paths []string) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()
	stackcmd := exec.Command("stack", "run")
	stackcmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	err = stackcmd.Start()
	if err != nil {
		log.Fatal(err)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	done := make(chan bool)

	go func() {
		<-sigs
		stop(stackcmd)
		done <- true
	}()

	go func() {
		for {
			pid := stackcmd.Process.Pid
			log.Println(pid, "up")
		st:
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				log.Println(event)
				for {
					timeup := time.After(1000 * time.Millisecond)
					select {
					case <-timeup:
						stop(stackcmd)
						log.Println("restart dev server")
						stackcmd = exec.Command("stack", "run")
						stackcmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
						err = stackcmd.Start()
						if err != nil {
							log.Fatal(err)
						}
						log.Println(pid, "up")
						break st
					case event, ok := <-watcher.Events:
						if !ok {
							return
						}
						log.Println("+", event)
					case err, ok := <-watcher.Errors:
						if !ok {
							return
						}
						log.Println("error:", err)
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("error:", err)
			}
		}
	}()

	for _, path := range paths {
		err = watcher.Add(path)
		if err != nil {
			log.Fatal(err)
		}
	}
	<-done
}

func main() {
	watch(os.Args[1:])
}
