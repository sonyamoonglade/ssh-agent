package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

var ErrTimeoutExceeded = "timeout is reached"

//TODO:
//decompose into functions
//read configs
//interactive mode
//dry (no stderr/stdout)
//verbose(stderr+stdout)

func main() {

	pathToKey := flag.String("key-path", "", "absolute path to ssh private key in local system")
	pkRAW := flag.String("key", "", "raw ssh private key")

	user := flag.String("user", "", "user to log in as")
	host := flag.String("host", "", "host to log in")

	timeout := flag.Int("timeout", 0, "execution timeout")

	askedCommand := flag.String("cmd", "", "command to be executed via ssh session")

	flag.Parse()
	if *timeout == 0 {
		*timeout = 5
	}

	if *pathToKey == "" && *pkRAW == "" {
		log.Fatal("key path nor RAW key are not provided")
	}

	if *askedCommand == "" {
		log.Fatal("command is not provided")
	}
	rawMode := *pkRAW != "" // use RAW key or file

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(*timeout))
	defer cancel()

	var cmd *exec.Cmd
	userAndHost := fmt.Sprintf("%s@%s", *user, *host)
	if rawMode {

		f, err := os.CreateTemp("./", "*")
		if err != nil {
			log.Fatalf("could not create temporary ssh_key file: %s\n", err.Error())
		}
		defer f.Close()

		content := *pkRAW + "\n" // ssh require \n at the end

		_, err = f.Write([]byte(content))
		if err != nil && err != io.EOF {
			log.Fatalf("could not write ssh_key content: %s", err.Error())
		}

		stat, err := f.Stat()
		if err != nil {
			log.Fatalf("could not get temporary's file stat: %s", err.Error())
		}

		sshPath := fmt.Sprintf("./%s", stat.Name())
		cmd = exec.CommandContext(ctx, "ssh", "-i", sshPath, userAndHost)

		defer func() {
			err := os.RemoveAll(sshPath)
			if err != nil {
				log.Fatalf("could not remove temorary file: %s", err.Error())
			}
		}()

	} else {
		cmd = exec.CommandContext(ctx, "ssh", "-i", *pathToKey, userAndHost)
	}

	cmd.Stderr = os.Stderr
	sshStdin, err := cmd.StdinPipe()
	if err != nil {
		log.Fatalf("could not link stdin to remote ssh: %s\n", err.Error())
	}

	io.WriteString(sshStdin, *askedCommand)
	sshStdin.Close()

	err = cmd.Run()
	if err != nil {
		if parseTimeoutErr(err) {
			log.Println(ErrTimeoutExceeded)
			return
		}
		log.Printf("exited from a session: %s\n", err.Error())
	}
}

func parseTimeoutErr(err error) bool {
	return strings.Contains(err.Error(), "signal: killed") //sent if timeout is reached
}
