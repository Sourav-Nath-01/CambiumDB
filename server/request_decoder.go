package server

import (
	"encoding/binary"
	"io"
	"net"
	"slices"
	"time"
)

var (
	noBodyOpCodes = []string{"P", "S", "C"}
)

type Request struct {
	opCode string
	body   []byte
}

func readNBytes(reader io.Reader, N int) ([]byte, error) {

	data := make([]byte, N)

	// a single Read may return fewer bytes than requested when a request is
	// split across TCP segments, so read until the buffer is full.
	if _, err := io.ReadFull(reader, data); err != nil {
		return nil, err
	}

	return data, nil
}

func readUInt32(reader io.Reader) (uint32, error) {

	data, err := readNBytes(reader, 4)

	if err != nil {
		return 0, err
	}

	return binary.LittleEndian.Uint32(data), nil
}

func readRequest(conn net.Conn) (*Request, error) {

	opCodeByte, err := readNBytes(conn, 1)

	if err != nil {
		return nil, err
	}

	// the read deadline only exists to poll for shutdown between requests.
	// clear it so the rest of the request is read to completion.
	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		return nil, err
	}

	opCode := string(opCodeByte)

	request := &Request{
		opCode: opCode,
	}

	if slices.Contains(noBodyOpCodes, request.opCode) {
		return request, nil
	}

	requestBodyLength, err := readUInt32(conn)

	if err != nil {
		return nil, err
	}

	requestBody, err := readNBytes(conn, int(requestBodyLength))

	if err != nil {
		return nil, err
	}

	request.body = requestBody

	return request, nil
}

func decodeInsertRequestBody(body []byte) (key []byte, value []byte) {

	pointer := 0
	keyLength := binary.LittleEndian.Uint32(body[pointer : pointer+4])

	pointer += 4

	key = make([]byte, keyLength)

	copy(key, body[pointer:pointer+int(keyLength)])

	pointer += int(keyLength)

	valueLength := binary.LittleEndian.Uint32(body[pointer : pointer+4])
	pointer += 4

	value = make([]byte, valueLength)

	copy(value, body[pointer:pointer+int(valueLength)])

	return key, value

}

func decodeGetRequestBody(body []byte) (key []byte) {

	pointer := 0
	keyLength := binary.LittleEndian.Uint32(body[pointer : pointer+4])

	pointer += 4

	key = make([]byte, keyLength)

	copy(key, body[pointer:pointer+int(keyLength)])

	return key
}

func decodeDeleteRequestBody(body []byte) (key []byte) {

	pointer := 0
	keyLength := binary.LittleEndian.Uint32(body[pointer : pointer+4])

	pointer += 4

	key = make([]byte, keyLength)

	copy(key, body[pointer:pointer+int(keyLength)])

	return key

}
