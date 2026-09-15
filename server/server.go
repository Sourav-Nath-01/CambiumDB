package server

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	bplustree "github.com/Sourav-Nath-01/CambiumDB/bplustree"
)

type Server struct {
	addr     string
	listener net.Listener

	bPlusTree *bplustree.BPlusTree

	shutdown     chan struct{}
	shutdownOnce *sync.Once
}

func NewServer(addr string, bPlusTree *bplustree.BPlusTree) (*Server, error) {

	listener, err := net.Listen("tcp", addr)

	if err != nil {
		return nil, err
	}
	return &Server{
		bPlusTree:    bPlusTree,
		listener:     listener,
		addr:         addr,
		shutdown:     make(chan struct{}),
		shutdownOnce: &sync.Once{},
	}, nil
}

func handleShutdown(conn net.Conn) {

	message := encodeShutdownMessage()

	slog.Info(fmt.Sprintf("sending shutdown message %v", message))
	if _, err := conn.Write(message); err != nil {
		slog.Error(err.Error(), "msg", "error while sending shutdown message")
	}

	if err := conn.Close(); err != nil {
		slog.Error(err.Error(), "msg", "error while closing connection")
	}

}

func sendErrorResponse(conn net.Conn, err error, message string) {

	slog.Error(err.Error(), "msg", message)
	response := encodeErrorResponse(err)

	if _, err2 := conn.Write(response); err2 != nil {
		slog.Error(err2.Error(), "msg", "error while writing to connection")
	}
}

// errClientClosed is returned when the client asked to end the connection.
var errClientClosed = errors.New("client closed connection")

// handleRequest serves a single request. A non-nil error means the connection
// is finished and must be closed by the caller.
func (server *Server) handleRequest(conn net.Conn) error {

	// read request from connection
	request, err := readRequest(conn)

	// check for read timeout error
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return nil
	}
	// the connection is unusable once a read fails, so don't try to reply on it
	if err != nil {
		slog.Error(err.Error(), "msg", "error while reading request")
		return err
	}

	// interpret request body based on op code
	switch request.opCode {

	// handle PING request
	case "P":

		// create OK response
		response := encodeOKResponse()

		// send response
		if _, err := conn.Write(response); err != nil {
			slog.Error(err.Error(), "msg", "error while sending OK response")
		}

	// handle INSERT request
	case "I":

		// extract key value pair from request body
		key, value := decodeInsertRequestBody(request.body)

		// call insert function
		err = server.bPlusTree.Insert(key, value)

		// handle error
		if err != nil {
			sendErrorResponse(conn, err, "error occured in data structure layer")
			return nil

		}

		// create OK response
		response := encodeOKResponse()

		// send response
		if _, err = conn.Write(response); err != nil {
			slog.Error(err.Error(), "msg", "error while writing to conn")
		}

	// handle DELETE request
	case "D":

		// extract key from request body
		key := decodeDeleteRequestBody(request.body)

		// call delete function
		err = server.bPlusTree.Delete(key)

		// handle error
		if err != nil {
			sendErrorResponse(conn, err, "error occured in data structure layer")
			return nil

		}

		// create OK response
		response := encodeOKResponse()

		// send response
		if _, err = conn.Write(response); err != nil {
			slog.Error(err.Error(), "msg", "error while writing to conn")
		}

	// handle GET request
	case "G":

		// extract key from request body
		key := decodeGetRequestBody(request.body)

		// call get function
		value, err := server.bPlusTree.Get(key)

		// handle error
		if err != nil {
			sendErrorResponse(conn, err, "error occured in data structure layer")
			return nil

		}

		// create success response
		response := encodeGetResponse(key, value)

		// send response
		if _, err = conn.Write(response); err != nil {
			slog.Error(err.Error(), "msg", "error while writing to conn")
		}

	// handle CLOSE request
	case "C":

		// create OK response
		response := encodeOKResponse()

		// send response
		if _, err := conn.Write(response); err != nil {
			slog.Error(err.Error(), "msg", "error while writing to conn")
		}

		// the caller closes the connection
		return errClientClosed

	// handle SHUTDOWN request
	case "S":
		slog.Info("server received shut down message")

		// initiate server shutdown
		server.Shutdown()

	// handle invalid op code
	default:

		slog.Error("invalid op code")

		sendErrorResponse(conn, fmt.Errorf("invalid op code"), "invalid op code")

	}

	return nil
}
func (server *Server) handleClient(conn net.Conn, wg *sync.WaitGroup) {

	defer wg.Done()

	for {

		select {

		case <-server.shutdown:
			slog.Info("client exiting...")
			handleShutdown(conn)
			return

		default:

			// the deadline must be refreshed every pass, otherwise it expires
			// once and every later read fails immediately.
			if err := conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
				conn.Close()
				return
			}

			// any error means the connection is done, stop the goroutine
			if err := server.handleRequest(conn); err != nil {
				conn.Close()
				return
			}
		}

	}

}

func (server *Server) listen(listenerWaitGroup, clientWaitGroup *sync.WaitGroup) {

	defer listenerWaitGroup.Done()

	for {

		conn, err := server.listener.Accept()
		if errors.Is(err, net.ErrClosed) {
			slog.Error(err.Error(), "msg", "listener closed")
			return
		}
		// conn is nil on any other error, so don't dereference it
		if err != nil {
			slog.Error(err.Error(), "msg", "error while accepting connection")
			continue
		}
		slog.Info("client joined from " + conn.RemoteAddr().String())
		clientWaitGroup.Add(1)
		go server.handleClient(conn, clientWaitGroup)

	}

}

func (server *Server) Run() {

	PrintBanner()
	clientWaitGroup := &sync.WaitGroup{}
	listenerWaitGroup := &sync.WaitGroup{}

	listenerWaitGroup.Add(1)
	go server.listen(listenerWaitGroup, clientWaitGroup)

	slog.Info("waiting for shutdown...")
	listenerWaitGroup.Wait()
	slog.Info("waiting for clients to exit...")
	clientWaitGroup.Wait()
}

func (server *Server) Shutdown() {

	slog.Info("shutdown initiated...")
	server.shutdownOnce.Do(func() {
		server.bPlusTree.Close()
		server.listener.Close()
		close(server.shutdown)

	})

}

func PrintBanner() {
	fmt.Print(`
  ╔════════════════════════════════════════╗
  ║                                        ║
  ║   CambiumDB                            ║
  ║   A B+ tree storage engine, in Go.     ║
  ║                                        ║
  ╚════════════════════════════════════════╝
`)
}
