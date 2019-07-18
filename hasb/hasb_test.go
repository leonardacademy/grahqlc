package hasb

import (
	"context"
	"log"
	"math/rand"
	"os"
	"reflect"
	"testing"
	"testing/quick"

	"github.com/gofrs/uuid"
	"github.com/leonardacademy/graphqlc"
)

type alphanum string

func (a alphanum) Generate(rand *rand.Rand, size int) reflect.Value {
	var ret []rune
	alphanum_chars := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890")
	for i := 0; i < rand.Intn(size); i++ {
		ret = append(ret, alphanum_chars[rand.Intn(size)])
	}
	return reflect.ValueOf(alphanum(ret))
}

func TestGetUpdate(t *testing.T) {
	client := getClient()
	f := func(x int32, a alphanum) bool {
        id := "27f33f9b-c47b-4b26-bade-763b8774a338"
		s := string(a)
		log.Println(s)
		err := client.Run(context.Background(), UpdateRow("graphqlc_tests", uuid.FromStringOrNil(id), map[string]interface{}{"num": x, "sentence": s}), nil)
		if err != nil {
			log.Println(err)
			return false
		}
		var ret map[string]interface{}
		err = client.Run(context.Background(), GetRow("graphqlc_tests", uuid.FromStringOrNil(id), []string{"num", "sentence"}), &ret)
		if err != nil {
			log.Println(err)
			return false
		}
		log.Println(ret)
		ret = ret["graphqlc_tests"].([]interface{})[0].(map[string]interface{})
		return ret["num"] == float64(x) && ret["sentence"] == s
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 10}); err != nil {
		t.Error(err)
	}
}

func TestInsertDelete(t *testing.T) {
	client := getClient()
	f := func(x int32, a alphanum) bool {
        s := string(a)
        var resp MResp
        err := client.Run(context.Background(), InsertRowRet("graphqlc_tests", map[string]interface{}{"num": x, "sentence": s}, []string{"id"}), &resp)
        if err != nil {
            log.Println(err)
            return false
        }

        log.Println(resp)
        id, err := uuid.FromString(resp["insert_graphqlc_tests"].Returning[0]["id"].(string))
        if err != nil {
            log.Println(err)
            return false
        }

        err = client.Run(context.Background(), DeleteRow("graphqlc_tests", id), nil)
        if err != nil {
            log.Println(err)
            return false
        }
        return true
	}
    if err := quick.Check(f, &quick.Config{MaxCount: 10}); err != nil {
		t.Error(err)
	}
}

func getClient() *graphqlc.Client {
	ret := graphqlc.NewClient(os.Getenv("HASURA_URL"))
	ret.Header.Set("x-hasura-admin-secret", os.Getenv("HASURA_ADMIN_SECRET"))
	ret.Log = logGqlcError
	return ret
}

func logGqlcError(text string) {
	log.Println("gqlc: " + text)
}
