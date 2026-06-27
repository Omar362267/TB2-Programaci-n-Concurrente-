package distributed

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
)

func SendRequest(ctx context.Context, address string, request Request) (Response, error) {
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return Response{}, err
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	if err := json.NewEncoder(conn).Encode(request); err != nil {
		return Response{}, err
	}
	var response Response
	if err := json.NewDecoder(bufio.NewReader(conn)).Decode(&response); err != nil {
		return Response{}, err
	}
	if response.Type == MessageError {
		return response, fmt.Errorf("nodo %s: %s", response.NodeID, response.Error)
	}
	return response, nil
}
