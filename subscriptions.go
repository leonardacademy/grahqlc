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

type SubscriptionEvent struct {
    NewData bool
    Err     error
    Closing bool
}

func (c *Client) Subscribe(ctx context.Context, req *Request, resp interface{}, notifications chan SubscriptionEvent) {
    if id, ws, err := c.startSubscription(req); err == nil {
        c.handleSubscription(ctx, id, ws, resp, notifications)
    } else {
        notifications <- SubscriptionEvent{Err: err, Closing: true}
    }
}

func (c *Client) startSubscription(req *Request) (uuid.UUID, *websocket.Conn, error) {
    var id uuid.UUID
    s := strings.SplitN(c.Endpoint, ":", 2)
    if s[0] == "http" {
        s[0] = "ws"
    } else {
        s[0] = "wss"
    }
    wsc, err := websocket.NewConfig(s[0]+":"+s[1], c.Endpoint)
    if err != nil {
        return id, nil, errors.Wrap(err, "error during websocket config generation")
    }
    for key, values := range c.Header {
        wsc.Header.Set(key, values[0])
        for _, value := range values[1:] {
            wsc.Header.Add(key, value)
        }
    }
    for key, values := range req.Header {
        wsc.Header.Set(key, values[0])
        for _, value := range values[1:] {
            wsc.Header.Add(key, value)
        }
    }
    wsc.Protocol = []string{"graphql-ws"}
    ws, err := websocket.DialConfig(wsc)
    if err != nil {
        return id, nil, errors.Wrap(err, "error during websocket dial")
    }
    c.logf(">> %s", jsonStr(gqlConnectionInit))
    websocket.JSON.Send(ws, gqlConnectionInit)
    var recv gowMsg
    err = websocket.JSON.Receive(ws, &recv)
    if err != nil {
        return id, nil, errors.Wrap(err, "could not decode server response after init")
    }
    for recv.Type != "connection_ack" {
        switch recv.Type {
        case "ka":
            c.logf("<< %s", jsonStr(recv))
        case "connection_error":
            return id, nil, errors.Wrap(jsonError(recv), "server responded with error after init;")
        default:
            return id, nil, errors.Wrap(jsonError(recv), "expected an ack but server gave something else")
        }
        err := websocket.JSON.Receive(ws, &recv)
        if err != nil {
            return id, nil, errors.Wrap(err, "could not decode server response after init")
        }
    }
    c.logf("<< %s", jsonStr(recv))
    id, err = uuid.NewV4()
    if err != nil {
        return id, nil, errors.Wrap(err, "failed to generate uuid during subscription")
    }
    start := gowMsg{
        Payload: struct {
            Query     string      `json:"query"`
            Variables interface{} `json:"variables"`
        }{
            req.q,
            req.vars,
        },
        Id:   id.String(),
        Type: "start",
    }
    c.logf(">> %s", jsonStr(start))
    websocket.JSON.Send(ws, start)
    return id, ws, nil
}

func (c *Client) handleSubscription(ctx context.Context, id uuid.UUID, ws *websocket.Conn, resp interface{}, notifications chan SubscriptionEvent) {
    select {
    case <-ctx.Done():
        notifications <- SubscriptionEvent{false, ctx.Err(), true}
        return
    default:
    }
    defer websocket.JSON.Send(ws, gowMsg{Payload: nil, Id: id.String(), Type: "stop"})
    for {
        recv := &gowMsg{Payload: graphResponse{Data: resp}}
        err := websocket.JSON.Receive(ws, &recv)
        if err == nil {
            c.logf("<< %s", jsonStr(recv))
            switch recv.Type {
            case "ka":
            case "error":
                notifications <- SubscriptionEvent{false, jsonError(recv.Payload), false}
            case "connection_error":
                notifications <- SubscriptionEvent{false, jsonError(recv.Payload), false}
            case "data":
                if pmap, ok := recv.Payload.(map[string]interface{}); ok {
                    if pmap["data"] != nil && pmap["data"] != "" {
                        notifications <- SubscriptionEvent{NewData: true}
                    } else if err, exists := pmap["errors"]; exists {
                        c.logf("got resolver errrors during subscription: %s", jsonStr(err))
                    } else {
                        c.logf("got a data response from the server during subscription, but no data.")
                    }
                } else {
                    c.logf("received data message during subscription but could not parse it into a json object.")
                }
            default:
                notifications <- SubscriptionEvent{Err: errors.Wrap(jsonError(recv), "could not identify response message.")}
            }
        } else {
            notifications <- SubscriptionEvent{Err: errors.Wrap(err, "could not parse response into a json object")}
        }
    }
}
func jsonError(payload interface{}) error {
    return errors.New(jsonStr(payload))
}
func jsonStr(payload interface{}) string {
    if b, err := json.Marshal(payload); err == nil {
        return string(b)
    }
    return fmt.Sprintf("%s", payload)
}

//Graphql over websocket message struct.
type gowMsg struct {
    Payload interface{} `json:"payload"`
    Id      string      `json:"id"`
    Type    string      `json:"type"`
}

var gqlConnectionInit = gowMsg{Payload: nil, Type: "connection_init"}
