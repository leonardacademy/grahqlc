package hasb

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/leonardacademy/graphqlc"
)

type EventPayload struct {
	Event     EventDetails  `json:"event"`
	CreatedAt time.Time     `json:"created_at"`
	Id        uuid.UUID     `json:"id"`
	Table     TableMetadata `json:"table"`
}

type EventDetails struct {
	SessionAttributes map[string]interface{} `json:"session_variables"`
	Op                string                 `json:"op"`
	RowChange         RowChange              `json:"data"`
}

type RowChange struct {
	OldRow map[string]interface{} `json:"old"`
	NewRow map[string]interface{} `json:"new"`
}

type TableMetadata struct {
	Schema string `json:"schema"`
	Name   string `json:"name"`
}

type EventTrigger struct {
	Name string `json:"name"`
}

func hasuraTypeOf(x interface{}) string {
	switch x.(type) {
	case int:
		return "Int"
	case int16:
		return "Int"
	case int32:
		return "Int"
	case bool:
		return "Boolean"
	case uuid.UUID:
		return "uuid"
	case string:
		return "String"
	}
	log.Println("Could not identify type of variable", x, " while generating hasura request")
	return ""
}

func GetEventPayload(r *http.Request) (*EventPayload, error) {
	var err error
	payload := new(EventPayload)
	if r.Body != nil {
		err = json.NewDecoder(r.Body).Decode(payload)
	} else {
		err = errors.New("request body is nil")
	}
	return payload, err
}

func DeleteWhere(tableName string, vars map[string]interface{}, where ExpressionTree) *graphqlc.Request {
	reqs := "mutation ("
	for k, v := range vars {
		varType := hasuraTypeOf(v)
		reqs += "$" + k + ": " + varType + "!, "
	}
	reqs = strings.TrimSuffix(reqs, ", ")
	reqs += `) {
        delete_` + tableName + "(where: {" + where.String() + `}) {
            affected_rows
        }
    }`
	req := graphqlc.NewRequest(reqs)
	for k, v := range vars {
		req.Var(k, v)
	}
	return req
}

func DeleteRow(tableName string, id interface{}) *graphqlc.Request {
	where := NewExpTreeB().Val("_eq").LRVal("id", "$key").Result()
	vars := make(map[string]interface{})
	vars["id"] = id
	return DeleteWhere(tableName, vars, where)
}

func UpdateRowCol(tableName string, rowId interface{}, columnName string, rowColVal interface{}) *graphqlc.Request {
	return UpdateRow(tableName, rowId, map[string]interface{}{columnName: rowColVal})
}

//it just works
func UpdateRow(tableName string, rowId interface{}, columnValues map[string]interface{}) *graphqlc.Request {
	reqs := "mutation ($id: " + hasuraTypeOf(rowId) + "!, "
	for k := range columnValues {
		varType := hasuraTypeOf(columnValues[k])
		reqs += "$" + k + ": " + varType + "!, "
	}
	reqs = strings.TrimSuffix(reqs, ", ")
	reqs += `) {
        update_` + tableName + "(where: {id: {_eq: $id}}, _set: {"
	for k := range columnValues {
		reqs += k + ": " + "$" + k + ", "
	}
	reqs = strings.TrimSuffix(reqs, ", ")
	reqs += `}) {
            affected_rows
        }
    }`
	req := graphqlc.NewRequest(reqs)
	req.Var("id", rowId)
	for k, v := range columnValues {
		req.Var(k, v)
	}
	return req
}

func GetRowCol(tableName string, rowId interface{}, columnName string) *graphqlc.Request {
	return GetRow(tableName, rowId, []string{columnName})
}

func GetRow(tableName string, rowId interface{}, columns []string) *graphqlc.Request {
	reqs := "query ($id: " + hasuraTypeOf(rowId) + "!) {\n"
	reqs += "\t" + tableName + "(where: {id: {_eq: $id}}) {\n"
	for _, v := range columns {
		reqs += "\t\t" + v + "\n"
	}
	reqs += "\t}\n"
	reqs += "}"
	req := graphqlc.NewRequest(reqs)
	req.Var("id", rowId)
	return req
}
