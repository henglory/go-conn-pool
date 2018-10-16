package main

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

type cigwConn struct {
	connDialer     net.Dialer
	conn           net.Conn
	markAsDestroy  bool
	brokenListener chan error
	isReady        bool
}

func (conn *cigwConn) initConn() error {
	var err error
	conn.conn, err = conn.connDialer.Dial("tcp", "localhost:8081")
	if err != nil {
		return err
	}
	return nil
}

func (conn *cigwConn) addRetryConnection() {
	go func() {
		var err error
		for {
			if conn.isReady {
				<-conn.brokenListener
			}
			if conn.markAsDestroy {
				conn.destroy()
				return
			}
			conn.conn, err = conn.connDialer.Dial("tcp", "localhost:8081")
			if err == nil {
				conn.isReady = true
			} else {
				conn.isReady = false
				// fmt.Println("will retry in 5 seconds")
				time.Sleep(5 * time.Second)
			}
		}
	}()
}

func (conn *cigwConn) addAfterRetryInit() {
	go func() {
		var err error
		for {
			conn.conn, err = conn.connDialer.Dial("tcp", "localhost:8081")
			if err == nil {
				conn.isReady = true
				conn.addRetryConnection()
				return
			}
			if conn.markAsDestroy {
				conn.conn.Close()
				return
			}
			// fmt.Println("will retry in 5 seconds")
			time.Sleep(5 * time.Second)
		}
	}()
}

func (conn *cigwConn) destroy() {
	if conn.conn != nil {
		conn.conn.Close()
	}
}

/*CigwPool is pool
 */
type CigwPool struct {
	size       int
	poolMember chan *cigwConn
}

//InitPool is init
func (pool *CigwPool) InitPool(size int) {
	pool.poolMember = make(chan *cigwConn, size)
	for x := 0; x < size; x++ {
		conn := constructConn()
		pool.poolMember <- conn
	}
	pool.size = size
}

//SendMessage send msg by pool
func (pool *CigwPool) SendMessage(message string) <-chan string {
	result := make(chan string)
	timeout := make(chan bool, 1)
	go func() {
		time.Sleep(1 * time.Second)
		timeout <- true
	}()

	var conn *cigwConn
	select {
	case conn = <-pool.poolMember:
		break
	case <-timeout:
		conn = constructConn()
		if !conn.isReady {
			result <- "connection is not ready"
			pool.release(conn)
			close(result)
			return result
		}
	}

	go func() {
		if !conn.isReady {
			result <- "connection is not ready"
			pool.release(conn)
			close(result)
			return
		}

		_, writeErr := conn.conn.Write([]byte(message + " \n"))
		conn.conn.SetReadDeadline(time.Now().Add(15 * time.Second))
		response, readErr := bufio.NewReader(conn.conn).ReadString('\n')

		if timeoutErr, ok := readErr.(net.Error); ok && timeoutErr.Timeout() {
			conn.markAsDestroy = true
			conn.brokenListener <- timeoutErr
			result <- "timeout"
			newConn := constructConn()
			pool.release(newConn)
			close(result)
			return
		}

		if writeErr != nil || readErr != nil {
			conn.brokenListener <- errors.New("connection broken")
			result <- "connection broken"
			pool.release(conn)
			close(result)
		} else {
			result <- response
			pool.release(conn)
			close(result)
		}

	}()

	return result
}

func (pool *CigwPool) release(conn *cigwConn) {
	if len(pool.poolMember) < pool.size {
		pool.poolMember <- conn
	} else {
		if conn.conn != nil {
			conn.conn.Close()
		}
		conn.markAsDestroy = true
		conn.brokenListener <- errors.New("conn more than limit")
	}
}

func constructConn() *cigwConn {
	cigwConn := &cigwConn{
		connDialer: net.Dialer{
			KeepAlive: 1 * time.Second,
		},
		brokenListener: make(chan error),
		isReady:        false,
		markAsDestroy:  false,
	}
	errInitConn := cigwConn.initConn()
	if errInitConn != nil {
		cigwConn.addAfterRetryInit()
		return cigwConn
	}
	cigwConn.isReady = true
	cigwConn.addRetryConnection()
	return cigwConn
}

func main() {

	pool := &CigwPool{}
	pool.InitPool(20)
	var aitr int = 0
	for {
		go func() {
			laitr := aitr
			aitr = aitr + 1
			aa1 := <-pool.SendMessage("Ping a " + strconv.Itoa(laitr))
			if ("Pong a " + strconv.Itoa(laitr)) != strings.Trim(strings.Replace(aa1, "\n", "", 1), " ") {
				fmt.Println("Ping a " + strconv.Itoa(laitr) + " & " + aa1)
			}
		}()
		time.Sleep(100 * time.Millisecond)
	}
}
