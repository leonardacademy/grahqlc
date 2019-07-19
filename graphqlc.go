// Package graphqlc provides a low level GraphQL client.
//
//  // create a client (safe to share across requests)
//  client := graphqlc.NewClient("https://machinebox.io/graphql")
//
//  // make a request
//  req := graphqlc.NewRequest(`
//      query ($key: String!) {
//          items (id:$key) {
//              field1
//              field2
//              field3
//          }
//      }
//  `)
//  //(optional) set your own http client
//  client.HttpClient = http.DefaultClient
//  //(optional) set any variables
//  req.Var("key", "value")
//
//  // run it and capture the response
//  var respData ResponseStruct
//  if err := client.Run(ctx, req, &respData); err != nil {
//      log.Fatal(err)
//  }
package graphqlc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/pkg/errors"
)

// Client is a client for interacting with a GraphQL API.
type Client struct {
	//The graphql server endpoint.
	Endpoint string

	//net/http client used to make requests.
	HttpClient *http.Client

	// closeReq will close the request body immediately allowing for reuse of client
	CloseReq bool

	// Log is called with various debug information.
	// To log to standard out, use:
	//  client.Log = func(s string) { log.Println(s) }
	Log func(s string)

	//Determines the default http request headers for graphql queries.
	//If your graphql request has headers that contradict these, the
	//graphql request headers will take precedence.
	http.Header
}

// NewClient makes a new Client capable of making GraphQL requests.
func NewClient(endpoint string, opts ...ClientOption) *Client {
	c := &Client{
		Endpoint: endpoint,
		Log:      func(s string) {},
	}
	c.Header = make(http.Header)
	c.Header.Set("Accept", "application/json; charset=utf-8")
	c.HttpClient = http.DefaultClient
	for _, optionFunc := range opts {
		optionFunc(c)
	}
	return c
}

func (c *Client) logf(format string, args ...interface{}) {
	c.Log(fmt.Sprintf(format, args...))
}

func (c *Client) Run(req *Request) error {
    return c.RunCtxRet(context.Background(), req, nil)
}

func (c *Client) RunRet(req *Request, resp interface{}) error {
    return c.RunCtxRet(context.Background(), req, resp)
}

func (c *Client) RunCtx(ctx context.Context, req *Request) error {
    return c.RunCtxRet(context.Background(), req, nil)
}

// Run executes the query and unmarshals the response from the data field into
// the response object.
// Pass in a nil response object to skip response parsing.
// Pass in a channel for resp if the request a synchronization request in order
// to get updates.
// If the request fails or the server returns an error, the first error
// encountered will be returned.
func (c *Client) RunCtxRet(ctx context.Context, req *Request, resp interface{}) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if len(req.q) > len("subscription") && req.q[0:len("subscription")] == "subscription" {
        return errors.New("queries of type \"subscription\" should be sent using client.Subscribe()")
	}
	var requestBody bytes.Buffer
	var contentType string
	if len(req.files) > 0 {
		if err := encodeRequestBody(&requestBody, &contentType, req, true); err != nil {
			return err
		}
		c.logf(">> files: %d", len(req.files))
	} else {
		if err := encodeRequestBody(&requestBody, &contentType, req, false); err != nil {
			return err
		}

	}
	c.logf(">> variables: %v", req.vars)
	c.logf(">> query: %s", req.q)
	gr := &graphResponse{
		Data: resp,
	}
	r, err := http.NewRequest(http.MethodPost, c.Endpoint, &requestBody)
	if err != nil {
		return err
	}
	r.Close = c.CloseReq
	r.Header.Set("Content-Type", contentType)
	for key, values := range c.Header {
		r.Header.Set(key, values[0])
		for _, value := range values[1:] {
			r.Header.Add(key, value)
		}
	}
	for key, values := range req.Header {
		r.Header.Set(key, values[0])
		for _, value := range values[1:] {
			r.Header.Add(key, value)
		}
	}
	c.logf(">> headers: %v", r.Header)
	r = r.WithContext(ctx)
	res, err := c.HttpClient.Do(r)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, res.Body); err != nil {
		return errors.Wrap(err, "reading body")
	}
	c.logf("<< %s", buf.String())
	if err := json.NewDecoder(&buf).Decode(&gr); err != nil {
		if res.StatusCode != http.StatusOK {
			return fmt.Errorf("graphql: server returned a non-200 status code: %v", res.StatusCode)
		}
		return errors.Wrap(err, "decoding response")
	}
	if len(gr.Errors) > 0 {
		// return first error
		return gr.Errors[0]
	}
	return nil
}

func encodeRequestBody(requestBody *bytes.Buffer, contentType *string, req *Request, multiPartForm bool) error {
	if multiPartForm {
		writer := multipart.NewWriter(requestBody)
		*contentType = writer.FormDataContentType()
		if err := writer.WriteField("query", req.q); err != nil {
			return errors.Wrap(err, "write query field")
		}
		var variablesBuf bytes.Buffer
		if len(req.vars) > 0 {
			variablesField, err := writer.CreateFormField("variables")
			if err != nil {
				return errors.Wrap(err, "create variables field")
			}
			if err := json.NewEncoder(io.MultiWriter(variablesField, &variablesBuf)).Encode(req.vars); err != nil {
				return errors.Wrap(err, "encode variables")
			}
		}
		for i := range req.files {
			part, err := writer.CreateFormFile(req.files[i].Field, req.files[i].Name)
			if err != nil {
				return errors.Wrap(err, "create form file")
			}
			if _, err := io.Copy(part, req.files[i].R); err != nil {
				return errors.Wrap(err, "preparing file")
			}
		}
		if err := writer.Close(); err != nil {
			return errors.Wrap(err, "close writer")
		}
	} else {
		*contentType = "application/json; charset=utf-8"
		requestBodyObj := struct {
			Query     string                 `json:"query"`
			Variables map[string]interface{} `json:"variables"`
		}{
			Query:     req.q,
			Variables: req.vars,
		}
		if err := json.NewEncoder(requestBody).Encode(requestBodyObj); err != nil {
			return errors.Wrap(err, "encode body")
		}
	}
	return nil
}

// ClientOption are functions that are passed into NewClient to
// modify the behaviour of the Client.
type ClientOption func(*Client)

type graphErr struct {
	Message string
}

func (e graphErr) Error() string {
	return "graphql: " + e.Message
}

type graphResponse struct {
	Data   interface{}
	Errors []graphErr
}

// Request is a GraphQL request.
type Request struct {
	q     string
	vars  map[string]interface{}
	files []File

	// Header represent any request headers that will be set
	// when the request is made.
	Header http.Header
}

// NewRequest makes a new Request with the specified string.
func NewRequest(q string) *Request {
	req := &Request{
		q:      q,
		Header: make(http.Header),
	}
	return req
}

// Var sets a variable.
func (req *Request) Var(key string, value interface{}) {
	if req.vars == nil {
		req.vars = make(map[string]interface{})
	}
	req.vars[key] = value
}

// Vars gets the variables for this Request.
func (req *Request) Vars() map[string]interface{} {
	return req.vars
}

// Files gets the files in this request.
func (req *Request) Files() []File {
	return req.files
}

// Query gets the query string of this request.
func (req *Request) Query() string {
	return req.q
}

// File sets a file to upload.
// Files are only supported with a Client that was created with
// the UseMultipartForm option.
func (req *Request) File(fieldname, filename string, r io.Reader) {
	req.files = append(req.files, File{
		Field: fieldname,
		Name:  filename,
		R:     r,
	})
}

// File represents a file to upload.
type File struct {
	Field string
	Name  string
	R     io.Reader
}
