package main

import (
	"errors"
	"fmt"
	"io"
	"net"
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
	conn.conn, err = conn.connDialer.Dial("tcp", "localhost:45001")
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
			conn.conn, err = conn.connDialer.Dial("tcp", "localhost:45001")
			if err == nil {
				conn.isReady = true
			} else {
				conn.isReady = false
				fmt.Println("will retry in 5 seconds")
				time.Sleep(5 * time.Second)
			}
		}
	}()
}

func (conn *cigwConn) addAfterRetryInit() {
	go func() {
		var err error
		for {
			conn.conn, err = conn.connDialer.Dial("tcp", "localhost:45001")
			if err == nil {
				conn.isReady = true
				conn.addRetryConnection()
				return
			}
			if conn.markAsDestroy {
				conn.destroy()
				return
			}
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

		_, writeErr := conn.conn.Write([]byte(message))
		conn.conn.SetReadDeadline(time.Now().Add(25 * time.Second))
		var buff = make([]byte, 1263)

		_, readErr := io.ReadFull(conn.conn, buff)

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
			result <- string(buff)
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
		aitr = aitr + 1
		go func(temp int) {
			account := fmt.Sprintf("%017d", temp)
			rrn := fmt.Sprintf("%012d", temp)
			msg := "HHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHH0005200005    1         0100FFCA1802E2007FE6160004000011277438   3911000000000001000000000000000000000000181017  1017092909000000000000000000047640000000000000000P2P VIA IB     00000000000000000000000000FFFFFFFFFFFFFFFF1400112774380001     101234567890         0A000000000000000000000000000000000000000000000000KBNKPROMPT PAY VIA IB        PROMPT PAY VIA IB        I293632B004B09230923000192920 80+00004KBNK000000000000000000001017000000000000000000000000370004000011277438=1299=330001234=10000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000004011810171127743809290910090210201001                                     20181017                                                         -                                                 04040201BBLA0000000000001017" + rrn + "D0000000000000000000000000000001108ACCTID      1234567890                                                                                                                      " + account + "                                                                                                                                1000000000                                                        "
			aa1 := <-pool.SendMessage(msg)
			if len(aa1) < 1263 || account != aa1[1052:1069] {
				fmt.Println(account + " " + aa1)
			}
		}(aitr)
		// time.Sleep(3 * time.Millisecond)
		// aitr = aitr + 1
		// go func(temp int) {
		// 	account := fmt.Sprintf("%017d", temp)
		// 	rrn := fmt.Sprintf("%012d", temp)
		// 	msg := "HHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHH0005200005    1         0100FFCA1802E2007FE6160004000011277438   3911000000000001000000000000000000000000181017  1017092909000000000000000000047640000000000000000P2P VIA IB     00000000000000000000000000FFFFFFFFFFFFFFFF1400112774380001     101234567890         0A000000000000000000000000000000000000000000000000KBNKPROMPT PAY VIA IB        PROMPT PAY VIA IB        I293632B004B09230923000192920 80+00004KBNK000000000000000000001017000000000000000000000000370004000011277438=1299=330001234=10000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000004011810171127743809290910090210201001                                     20181017                                                         -                                                 04040201BBLA0000000000001017" + rrn + "D0000000000000000000000000000001108ACCTID      1234567890                                                                                                                      " + account + "                                                                                                                                1000000000                                                        "
		// 	aa1 := <-pool.SendMessage(msg)
		// 	if len(aa1) < 1263 || account != aa1[1052:1069] {
		// 		fmt.Println(account + " " + aa1)
		// 	}
		// }(aitr)
		time.Sleep(10 * time.Millisecond)
	}
}
