package graphqlc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gofrs/uuid"
	"github.com/pkg/errors"
	"golang.org/x/net/websocket"
)

func (c *Client) subscribe(ctx context.Context, req *Request, resp chan<-interface{}) error {
	var subscribed bool
	var id uuid.UUID
	var ws *websocket.Conn
	select {
	case <-ctx.Done():
		if subscribed {
			websocket.JSON.Send(ws, gowMsg{Payload: nil, Id: id.String(), Type: "GQL_STOP"})
		}
		return ctx.Err()
	default:
	}
	protocol := strings.SplitN(c.Endpoint, ":", 2)[0]
    if protocol == "http" {
        protocol = "ws"
    } else if protocol == "https" {
        protocol = "wss"
    } else if protocol != "ws" && protocol != "wss" {
		return errors.New("protocol for Endpoint url needs to be ws or wss when the query is a subscription")
	}
	ws, err := websocket.Dial(c.Endpoint, protocol, c.Endpoint)
	if err != nil {
		return errors.Wrap(err, "error during websocket Dial")
	}
	websocket.JSON.Send(ws, gqlConnectionInit)
	var recv gowMsg
	websocket.JSON.Receive(ws, &recv)
	if recv.Type != "connection_ack" {
		if recv.Type == "connection_error" {
			return errors.New("server did not acknowledge correctly")
		}
	}
	id, err = uuid.NewV4()
	if err != nil {
		return errors.Wrap(err, "failed to generate uuid during subscription")
	}
	start := gowMsg{
		Payload: struct {
			Query         string
			Variables     interface{}
			operationName string
		}{
			req.q,
			req.vars,
			"subscription",
		},
		Id:   id.String(),
		Type: "start",
	}
	websocket.JSON.Send(ws, start)
	subscribed = true
	for {
		websocket.JSON.Receive(ws, &recv)
		switch recv.Type {
		case "error":
			return responseError(recv.Payload)
        case "data":
            if pmap, ok := recv.Payload.(map[string]interface{}); ok {
                if pmap["data"] != nil {
                    resp <- pmap["data"]
                } else if err, exists := pmap["errors"]; exists {
                    c.logf("graphqlc: got resolver errrors during subscription: %s", responseError(err))
                } else {
                    c.logf("graphqlc: got a data response from the server during subscription, but no data.")
                }
            } else {
                c.logf("graphqlc: received data message during subscription but could not parse it into a json object.")
            }
		}
	}
}

func responseError(payload interface{}) error {
	ret := "received response error; attempting to print response payload:\n"
	switch v := payload.(type) {
	case string:
		ret += v
	case fmt.Stringer:
		ret += v.String()
	case map[string]interface{}:
		b, err := json.Marshal(v)
		if err == nil {
            ret += "graphqlc: payload was of type map[string]interface{} but we still couldn't json.Marshal it."
		} else {
			ret += string(b)
		}
	default:
		ret += "graphqlc: could not turn response error into string."
	}
	return errors.New(ret)
}

//Graphql over websocket message struct.
type gowMsg struct {
	Payload interface{} `json:"payload"`
	Id      string      `json:"id"`
	Type    string      `json:"type"`
}

var gqlConnectionInit = gowMsg{Payload: nil, Type: "GQL_CONNECTION_INIT"}
