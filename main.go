package main

import (
	"io"
	"io/ioutil"
	"log"
	"net"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// Based on example server code from golang.org/x/crypto/ssh and server_standalone
func main() {

	sshConf := &ssh.ServerConfig{
		NoClientAuth: true,
	}

	privateBytes, err := ioutil.ReadFile("id_rsa")
	if err != nil {
		log.Fatal("Failed to load private key", err)
	}

	private, err := ssh.ParsePrivateKey(privateBytes)
	if err != nil {
		log.Fatal("Failed to parse private key", err)
	}

	sshConf.AddHostKey(private)

	listener, err := net.Listen("tcp", "0.0.0.0:2022")
	if err != nil {
		log.Fatal("failed to listen for connection", err)
	}

	defer listener.Close()

	log.Println("Listening on ", listener.Addr())

	for {
		// Listen for an incoming connection.
		nConn, err := listener.Accept()
		if err != nil {
			log.Print("failed to accept incoming connection", err)
		}
		// Handle connections in a new goroutine.
		go handleRequest(nConn, sshConf)
	}
}

// Handles incoming requests.
func handleRequest(nConn net.Conn, sshConf *ssh.ServerConfig) {

	sconn, chans, reqs, err := ssh.NewServerConn(nConn, sshConf)
	if err != nil {
		log.Print("failed to handshake", err)
	}
	log.Println("login detected:", sconn.User())

	// The incoming Request channel must be serviced.
	go ssh.DiscardRequests(reqs)

	// Service the incoming Channel channel.
	for newChannel := range chans {
		// Channels have a type, depending on the application level
		// protocol intended. In the case of an SFTP session, this is "subsystem"
		// with a payload string of "<length=4>sftp"
		if newChannel.ChannelType() != "session" {
			newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}
		channel, requests, err := newChannel.Accept()
		if err != nil {
			log.Fatal("could not accept channel.", err)
		}

		// Sessions have out-of-band requests such as "shell",
		// "pty-req" and "env".  Here we handle only the
		// "subsystem" request.
		go func(in <-chan *ssh.Request) {
			for req := range in {
				ok := false
				switch req.Type {
				case "subsystem":
					if string(req.Payload[4:]) == "sftp" {
						ok = true
					}
				}
				req.Reply(ok, nil)
			}
		}(requests)

		root := sftp.InMemHandler()
		server := sftp.NewRequestServer(channel, root)
		if err := server.Serve(); err == io.EOF {
			server.Close()
			log.Print("sftp client exited session.")
		} else if err != nil {
			log.Fatal("sftp server completed with error:", err)
		}
	}
}
