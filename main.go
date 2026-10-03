package main

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
)

const (
	SSLRequest       = 80877103
	StartupMessage30 = 196608
	StartupMessage31 = 196609
	StartupMessage32 = 196610
)

func main() {
	// setting up the logger
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)

	listner, err := net.Listen("tcp", "127.0.0.1:6769")
	if err != nil {
		slog.Error("Failed to bind the port", "error", err)
		os.Exit(1)
	}
	defer listner.Close()

	logger.Info("Listening http://localhost:6769/")

	for {
		conn, err := listner.Accept()
		if err != nil {
			logger.Error("Error accepting connection", "error", err)
			continue
		}

		slog.Info("Accepted a connection")
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)

	/*
		postgresql protocol starts with a 'startup' phase, where the messages
		are 4bytes + 4 bytes + n bytes. Once that phase is done messages will be in
		1byte + 4 bytes + 4 bytes. Therefore, there's going to be two loops
	*/

StartupLoop:
	for {
		header := make([]byte, 8)
		if _, err := io.ReadFull(reader, header); err != nil {
			slog.Error("Error reading header from startup message", "error", err)
			continue
		}

		length := binary.BigEndian.Uint32(header[:4])
		code := binary.BigEndian.Uint32(header[4:])

		// length is calculated as self(4) + code(4) + message(length)
		if length < 8 {
			slog.Error("Message length cannot be less than 8", "length", length)
			continue
		}

		switch code {
		case SSLRequest:
			slog.Info("Received SSLRequest")
			// ain't dealing with SSL, sending a no
			if _, err := conn.Write([]byte{'N'}); err != nil {
				slog.Error("Couldn't say no to SSL request", "error", err)
				continue
			}
			return

		case StartupMessage30, StartupMessage31, StartupMessage32:
			slog.Info("Received StartupMessage")

			body := make([]byte, length-8) // length self + code = 8
			if _, err := io.ReadFull(reader, body); err != nil {
				slog.Error("Error reading StartupMessage body", "error", err)
				continue
			}

			// don't care about username and password atm, return OK
			response := make([]byte, 9)
			response[0] = 'R'
			binary.BigEndian.PutUint32(response[1:5], 8)
			binary.BigEndian.PutUint32(response[5:9], 0)
			if _, err := conn.Write(response); err != nil {
				slog.Error("Cannot send ok to startup message", "error", err)
				continue
			}

			slog.Info("StartupMessage", "body", string(body))

			break StartupLoop

		default:
			slog.Error("Unsupported initial phase message code", "code", code)
			continue
		}
	}

	ready := make([]byte, 6)
	ready[0] = 'Z'
	binary.BigEndian.PutUint32(ready[1:5], 5)
	ready[5] = 'I' // I for idle

	if _, err := conn.Write(ready); err != nil {
		slog.Error("Failed to send ReadyForQuery", "error", err)
		return
	}
	slog.Info("ReadyForQuery sent")

	for {
		messageType := make([]byte, 1)
		if _, err := io.ReadFull(reader, messageType); err != nil {
			if errors.Is(err, io.EOF) {
				slog.Info("Connection closed")
				break
			}
			slog.Error("Error occured while reading message type", "error", err)
			continue
		}

		slog.Debug("Message Type %v", "message", messageType)

		length := make([]byte, 4)
		if _, err := io.ReadFull(reader, length); err != nil {
			slog.Error("Error reading length of the message", "error", err)
			continue
		}

		messageBuffer := make([]byte, binary.BigEndian.Uint32(length))
		if _, err := io.ReadFull(reader, messageBuffer); err != nil {
			slog.Error("Error reading message", "error", err)
			continue
		}

		slog.Info(fmt.Sprintf("Received message %v: %s", messageType, string(messageBuffer)))
	}
}
