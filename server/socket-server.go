package main

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"time"
)

// only needed below for sample processing

func main() {

	fmt.Println("Launching server...")

	// listen on all interfaces
	ln, _ := net.Listen("tcp", ":8081")

	// accept connection on port
	for {
		conn, _ := ln.Accept()
		// run loop forever (or until ctrl-c)
		go process(conn)
	}
}

func process(conn net.Conn) {
	for {
		// will listen for message to process ending in newline (\n)
		if conn == nil {
			fmt.Println("conn nil")
			return
		}
		message, err := bufio.NewReader(conn).ReadString('\n')
		if err != nil {
			break
		}

		reply := strings.Replace(message, "Ping", "Pong", 1)
		if strings.Contains(message, "1") {
			fmt.Println("will timeout " + message)
			time.Sleep(16 * time.Second)
		} else {
			fmt.Println(reply)
		}
		// send new string back to client
		conn.Write([]byte(reply + "\n"))
	}
}
