package graphqlc

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/gofrs/uuid"
)

var serverUrl = "http://hasura.api.leonardacademy.com/v1/graphql"

func logger(s string) {
	log.Println("subscription test: " + s)
}
func TestSubscription(t *testing.T) {
	cl := NewClient(serverUrl)
	cl.Log = logger
	cl.Header.Set("Authorization", getToken())
	if cl.Header.Get("Authorization") == "" {
		t.Fatal("could not contact auth0 servers for token; cannot finish test")
	}
	rNum := 1
	rSentence := "Hello World!"
	r := NewRequest(`mutation ($num: Int!, $sentence: String!) {
        insert_graphqlc_tests(objects: {num: $num, sentence: $sentence}) {
            returning {
                id
                num
                sentence
            }
        }
    }`)
	r.Var("num", rNum)
	r.Var("sentence", rSentence)
	var insertResp testRowInsertResp
	err := cl.Run(context.Background(), r, &insertResp)
	if err != nil {
		t.Fatal("encountered error on insertion request: ", err)
	}
	row := insertResp.Ret["returning"][0]
	if row.Num != rNum || row.Sentence != rSentence {
		t.Fatal("returned array does not match insertion: ", rNum, rSentence, row.Num, row.Sentence)
	}

	r = NewRequest(`subscription ($key: uuid!){
        graphqlc_tests(where: {id: {_eq: $key}}) {
            num
            sentence
            id
        }
    }`)
	r.Var("key", row.Id.String())
	eventsChan := make(chan (SubscriptionEvent))
	var resp testRowResp
	go cl.Subscribe(context.Background(), r, &resp, eventsChan)
	select {
	case event := <-eventsChan:
		if event.NewData == false {
			t.Error("encountered error during subscription: ", event.Err)
		} else {
			log.Println(fmt.Sprintf("%v", resp))
			row = resp.Row[0]
			if row.Num != rNum || row.Sentence != rSentence {
				t.Error("returned array does not match insertion: ", rNum, rSentence, row.Num, row.Sentence)
			}
		}
	}
}

type token struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type testRowInsertResp struct {
	Ret map[string][]testRow `json:"insert_graphqlc_tests"`
}

type testRowResp struct {
	Row []testRow `json:"graphqlc_tests"`
}

type testRow struct {
	Id       uuid.UUID `json:"id"`
	Num      int       `json:"num"`
	Sentence string    `json:"sentence"`
}

func getToken() string {
	secret := os.Getenv("AUTH0_CLIENT_SECRET")
	clientId := "9JO148EPLVONrv4vwWPTk0fB40Xv2m2h"
	auth0_url := "https://dev-yb5uauf5.auth0.com/oauth/token"
	identifier := "https://hasura.api.leonardacademy.com"
	payload := strings.NewReader("grant_type=client_credentials&client_id=" + clientId + "&client_secret=" + secret + "&audience=" + identifier)

	req, _ := http.NewRequest("POST", auth0_url, payload)
	req.Header.Add("content-type", "application/x-www-form-urlencoded")

	if res, err := http.DefaultClient.Do(req); err == nil {
		defer res.Body.Close()
		var t token
		if err := json.NewDecoder(res.Body).Decode(&t); err == nil {
			return t.TokenType + " " + t.AccessToken
		}
	}
	return ""
}
