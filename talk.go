// TODO: ssh -i /Users/bradfitz/.ssh/id_rsa_boot2docker -o StrictHostKeyChecking=no  -o UserKnownHostsFile=/dev/null -p 2022 docker@localhost
// for proxying ports

package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	dockerFlag = flag.String("docker", "", "Path to docker, for child process")
	listenFlag = flag.String("listen", "127.0.0.1:3998", "Listen address")
	tagFlag    = flag.String("tag", "", "If non-empty, we're the child process and we should start or attach to this gc14:<tag> VM.")
)

var presentURL = httputil.NewSingleHostReverseProxy(&url.URL{
	Scheme: "http",
	Host:   "127.0.0.1:3999",
	Path:   "/",
})

func handleRoot(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if path == "/" || strings.HasSuffix(path, ".slide") || strings.HasPrefix(path, "/static/") {
		presentURL.ServeHTTP(w, r)
		return
	}
	http.NotFound(w, r)
}

type Shell struct {
	p     *os.Process
	port  int
	proxy http.Handler
}

var (
	mu    sync.Mutex
	shell = map[string]*Shell{}
)

func handleShell(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/shell/")
	if i := strings.Index(name, "/"); i >= 0 {
		name = name[:i]
	}
	mu.Lock()
	p, ok := shell[name]
	mu.Unlock()
	if ok {
		p.proxy.ServeHTTP(w, r)
		return
	}
	groupsb, err := exec.Command("groups").Output()
	if err != nil {
		log.Fatal(err)
	}
	groups := strings.Fields(string(groupsb))

	dockerPath, _ := exec.LookPath("docker")
	script := fmt.Sprintf(`#!/bin/sh
export DOCKER_HOST=%s
%s --tag=%s --docker=%s
`, os.Getenv("DOCKER_HOST"), os.Args[0], name, dockerPath)
	f, err := ioutil.TempFile("", "")
	if err != nil {
		log.Fatalf("TempFile: %v", err)
	}
	f.Write([]byte(script))
	f.Close()
	os.Chmod(f.Name(), 0770)

	u, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}
	port := freePort()
	args := []string{
		"--no-beep",
		"--disable-ssl",
		fmt.Sprintf("--port=%d", port),
		fmt.Sprintf("--service=/shell/%s:%s:%s:%s:%s", name, u.Username, groups[0], u.HomeDir, f.Name()),
	}
	css := filepath.Join(os.Getenv("HOME"), "talks", "2014-04-Gophercon", "shell.css")
	if _, err := os.Stat(css); err == nil {
		args = append(args, "--css="+css)
	}
	cmd := exec.Command("shellinaboxd", args...)
	// log.Printf("Running: %q", cmd.Args)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	proxy := httputil.NewSingleHostReverseProxy(&url.URL{
		Scheme: "http",
		Host:   "127.0.0.1:" + strconv.Itoa(port),
		Path:   "/",
	})
	mu.Lock()
	shell[name] = &Shell{
		p:     cmd.Process,
		port:  port,
		proxy: proxy,
	}
	mu.Unlock()
	go func() {
		err := cmd.Wait()
		log.Printf("Image %s ended with: %v", name, err)
		mu.Lock()
		defer mu.Unlock()
		delete(shell, name)
	}()
	time.Sleep(150 * time.Millisecond) // warm up time
	proxy.ServeHTTP(w, r)
}

func freePort() int {
	for p := 3900; p < 4200; p++ {
		c, err := net.Dial("tcp", "localhost:"+strconv.Itoa(p))
		if err != nil {
			return p
		}
		c.Close()
	}
	panic("out of ports?")
}

func main() {
	flag.Parse()
	if *dockerFlag == "" {
		if err := exec.Command("docker", "ps").Run(); err != nil {
			os.Setenv("DOCKER_HOST", "tcp://localhost:4243")
			if err := exec.Command("docker", "ps").Run(); err != nil {
				log.Fatalf("Failed to run docker ps. Forget boot2docker up?")
			}
		}
	}
	if *tagFlag != "" {
		startAttachTag(*tagFlag)
		return
	}
	exec.Command("killall", "present").Run()
	exec.Command("killall", "shellinabox").Run()
	presentCmd := exec.Command("present", ".")
	if err := presentCmd.Start(); err != nil {
		log.Fatalf("Error starting present: %v", err)
	}
	defer presentCmd.Process.Kill()
	if _, err := exec.LookPath("shellinaboxd"); err != nil {
		log.Fatalf("Can't find shellinaboxd in path")
	}
	http.HandleFunc("/shell/", handleShell)
	http.HandleFunc("/", handleRoot)

	log.Printf("Presenting to Gophercon 2014 on http://%s", *listenFlag)
	go exec.Command("open", "http://"+*listenFlag+"/2014-04-Gophercon.slide#1").Start()
	log.Fatal(http.ListenAndServe(*listenFlag, nil))
}

func startAttachTag(tag string) {
	if *dockerFlag == "" {
		*dockerFlag, _ = exec.LookPath("docker")
	}
	outb, err := exec.Command(*dockerFlag, "ps", "--no-trunc").CombinedOutput()
	if err != nil {
		log.Fatalf("docker ps: %v: %s", err, outb)
	}
	sc := bufio.NewScanner(bytes.NewReader(outb))
	img := "gc14:" + tag
	for sc.Scan() {
		if strings.Contains(sc.Text(), img) {
			fields := strings.Fields(sc.Text())
			if err := syscall.Exec(*dockerFlag,
				[]string{*dockerFlag, "attach", fields[0]},
				os.Environ()); err != nil {
				log.Fatalf("docker attach exec: %v", err)
			}

		}
	}
	if err := syscall.Exec(*dockerFlag,
		[]string{*dockerFlag, "run", "-t", "-i", "-h", tag, "-w", "/home/gopher", img, "/bin/bash"},
		os.Environ()); err != nil {
		log.Fatalf("docker run exec: %v", err)
	}
}
