package hasb

import (
	"encoding/json"
	"errors"
	"fmt"
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

type QResp map[string][]map[string]interface{}

type MResp map[string]MInnerResp

type MInnerResp struct {
    AffectedRows int `json:"affected_rows"`
    Returning []map[string]interface{} `json:"returning"`
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

type Query struct {
	Vars            map[string]interface{}
	getQueries      []GetQuery
	MutationQueries []MutationQuery
}

type GetQuery struct {
	tables []GetQueryTable
}

type GetQueryTable struct {
	Name    string
	Where   *ExpressionTree
	Objects []string
}

type MutationQuery struct {
	UpdateQueries []UpdateQuery
	InsertQueries []InsertQuery
	DeleteQueries []DeleteQuery
}

type DeleteQuery struct {
	TableName    string
	Where        *ExpressionTree
	Returning    []string
	AffectedRows bool
}

type InsertQuery struct {
	TableName    string
	Objects      map[string]string
	Returning    []string
	AffectedRows bool
}

type UpdateQuery struct {
	TableName    string
	Where        *ExpressionTree
	Set          map[string]string
	Returning    []string
	AffectedRows bool
}

func (q *Query) String() string {
	var ret string
	for i, gq := range q.getQueries {
		ret += "query gq" + fmt.Sprintf("%d", i)
		ret += wrapQuery(q.Vars, gq.String())
	}
	ret += "\n"
	for i, mq := range q.MutationQueries {
		ret += "mutation mq" + fmt.Sprintf("%d", i)
		ret += wrapQuery(q.Vars, mq.String())
	}
	return ret
}

func (q *Query) Request() *graphqlc.Request {
	ret := graphqlc.NewRequest(q.String())
	for k, v := range q.Vars {
		ret.Var(k, v)
	}
	return ret
}

func wrapQuery(Vars map[string]interface{}, q string) string {
	ret := "("
	for k, v := range Vars {
		if strings.Contains(q, "$"+k) {
			ret += "$" + k + ": " + hasuraTypeOf(v) + ", "
		}
	}
	ret = strings.TrimSuffix(ret, ", ")
	ret += ")"
	ret = strings.TrimSuffix(ret, "()") //In case the above query does not contain any variables.
	ret += " " + q
	return ret
}

func (q *Query) AddGetQueries(gq ...GetQuery) {
	if q.getQueries == nil {
		q.getQueries = make([]GetQuery, 0)
	}
	for _, v := range gq {
		q.getQueries = append(q.getQueries, v)
	}
}

func (q *Query) AddMutationQueries(mq ...MutationQuery) {
	if q.MutationQueries == nil {
		q.MutationQueries = make([]MutationQuery, 0)
	}
	for _, v := range mq {
		q.MutationQueries = append(q.MutationQueries, v)
	}
}

func (gq *GetQuery) AddTables(gqt ...GetQueryTable) {
	if gq.tables == nil {
		gq.tables = make([]GetQueryTable, 0)
	}
	for _, v := range gqt {
		gq.tables = append(gq.tables, v)
	}
}

func (gq *GetQuery) String() string {
	ret := "{\n"
	for _, gtq := range gq.tables {
		ret += gtq.String() + "\n"
	}
	ret += "}"
	return ret
}

func (gtq *GetQueryTable) String() string {
	ret := gtq.Name
	if gtq.Where != nil {
		ret += " (where: {" + gtq.Where.String() + "})"
	}
	ret += " {\n"
	for _, v := range gtq.Objects {
		ret += v + "\n"
	}
	ret += "}"
	return ret
}

func (mq *MutationQuery) String() string {
	ret := "{\n"
	for _, uq := range mq.UpdateQueries {
		ret += uq.String() + "\n"
	}
	for _, iq := range mq.InsertQueries {
		ret += iq.String() + "\n"
	}
	for _, dq := range mq.DeleteQueries {
		ret += dq.String() + "\n"
	}
	ret += "}"
	return ret
}

func (uq *UpdateQuery) String() string {
	ret := "update_" + uq.TableName
	ret += "("
	if uq.Where != nil {
		ret += "where: {" + uq.Where.String() + "}, "
	}
	ret += "_set:{"
	for k, v := range uq.Set {
		ret += k + ": $" + v + ", "
	}
	ret = strings.TrimSuffix(ret, ", ")
	ret += "}) {\n"
	if uq.AffectedRows {
		ret += "affected_rows\n"
	}
	if uq.Returning != nil && len(uq.Returning) > 0 {
		ret += "returning {\n"
		for _, v := range uq.Returning {
			ret += v + "\n"
		}
		ret += "}\n"
	}
	ret += "}"
	return ret
}

func (iq *InsertQuery) String() string {
	ret := "insert_" + iq.TableName + "(objects:{"
	for k, v := range iq.Objects {
		ret += k + ": $" + v + ", "
	}
	ret = strings.TrimSuffix(ret, ", ")
	ret += "}) {\n"
	if iq.AffectedRows {
		ret += "affected_rows\n"
	}
	if iq.Returning != nil && len(iq.Returning) > 0 {
		ret += "returning {\n"
		for _, v := range iq.Returning {
			ret += v + "\n"
		}
		ret += "}\n"
	}
	ret += "}"
	return ret
}

func (dq *DeleteQuery) String() string {
	ret := "delete_" + dq.TableName + "(where: {" + dq.Where.String() + "}){\n"
	if dq.AffectedRows {
		ret += "affected_rows\n"
	}
	if dq.Returning != nil {
		for _, v := range dq.Returning {
			ret += v + "\n"
		}
	}
	ret += "}"
	return ret
}
