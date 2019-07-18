package hasb

import (
	"github.com/leonardacademy/graphqlc"
)

func InsertRowRet(tableName string, columns map[string]interface{}, returning []string) *graphqlc.Request {
	var iq InsertQuery
	iq.TableName = tableName
	iq.Objects = make(map[string]string)
	for k := range columns {
		iq.Objects[k] = k
	}
	if returning != nil {
		iq.Returning = make([]string, 0)
		for _, v := range returning {
			iq.Returning = append(iq.Returning, v)
		}
	} else {
		iq.AffectedRows = true
	}
	var q Query
	q.Vars = columns
	var mq MutationQuery
	mq.InsertQueries = []InsertQuery{iq}
	q.MutationQueries = []MutationQuery{mq}
	return q.Request()
}

func InsertRow(tableName string, columns map[string]interface{}) *graphqlc.Request {
	return InsertRowRet(tableName, columns, nil)
}

func DeleteRow(tableName string, rowId interface{}) *graphqlc.Request {
	var dq DeleteQuery
	dq.TableName = tableName
	dq.Where = NewExpTreeB().Val("_eq").LRVal("id", "$id").Result()
	dq.AffectedRows = true
	var mq MutationQuery
	mq.DeleteQueries = []DeleteQuery{dq}
	var q Query
    q.Vars = map[string]interface{}{"id": rowId}
	q.MutationQueries = []MutationQuery{mq}
	return q.Request()
}

func GetRow(tableName string, rowId interface{}, columns []string) *graphqlc.Request {
	var gqt GetQueryTable
	gqt.Name = tableName
	gqt.Objects = columns
    gqt.Where = NewExpTreeB().Val("_eq").LRVal("id", "$id").Result()
	var gq GetQuery
	gq.AddTables(gqt)
	var q Query
    q.Vars = map[string]interface{}{"id": rowId}
	q.AddGetQueries(gq)
	return q.Request()
}

func UpdateRow(tableName string, rowId interface{}, set map[string]interface{}) *graphqlc.Request {
	var uq UpdateQuery
	uq.TableName = tableName
	uq.Where = NewExpTreeB().Val("_eq").LRVal("id", "$id").Result()
	uq.Set = make(map[string]string)
	for k := range set {
		uq.Set[k] = k
	}
	uq.AffectedRows = true
	var q Query
	q.Vars = set
	q.Vars["id"] = rowId
	var mq MutationQuery
	mq.UpdateQueries = []UpdateQuery{uq}
	q.MutationQueries = []MutationQuery{mq}
	return q.Request()
}
