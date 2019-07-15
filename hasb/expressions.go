package hasb

var prefixOps = []string{"_and", "_or", "_not"}

type ExpressionTree struct {
	Left  *ExpressionTree
	Right *ExpressionTree
	Val   string
	In    string
}

func (ct *ExpressionTree) String() string {
	if ct.In != "" {
		nct := *ct
		nct.In = ""
		return ct.In + `: {
            ` + nct.String() + `
        }`
	}
	for _, v := range prefixOps {
		if ct.Val == v {
			return ct.Val + `{
                ` + ct.Left.String() + `
                ` + ct.Right.String() + `
            }`
		}
	}
	return ct.Left.Val + ": { " + ct.Val + ": " + ct.Right.Val + "}"
}

type ExpressionTreeBuilder struct {
	working ExpressionTree
	loc     *ExpressionTree
	up      *expressionTreeStack
}

func NewExpTreeB() *ExpressionTreeBuilder {
	ret := new(ExpressionTreeBuilder)
	ret.loc = &ret.working
	return ret
}

func (ct *ExpressionTreeBuilder) Result() ExpressionTree {
	return ct.working
}

func (ct *ExpressionTreeBuilder) Left() *ExpressionTreeBuilder {
	ct.up.Push(ct.loc)
	if ct.working.Left == nil {
		ct.working.Left = new(ExpressionTree)
	}
	ct.loc = ct.loc.Left
	return ct
}
func (ct *ExpressionTreeBuilder) Right() *ExpressionTreeBuilder {
	ct.up.Push(ct.loc)
	if ct.working.Right == nil {
		ct.working.Right = new(ExpressionTree)
	}
	ct.loc = ct.loc.Right
	return ct
}

func (ct *ExpressionTreeBuilder) Up() *ExpressionTreeBuilder {
	ct.loc = ct.up.Pop()
	return ct
}

func (ct *ExpressionTreeBuilder) Val(v string) *ExpressionTreeBuilder {
	ct.loc.Val = v
	return ct
}

func (ct *ExpressionTreeBuilder) LRVal(v1 string, v2 string) *ExpressionTreeBuilder {
	if ct.loc.Left == nil {
		ct.loc.Left = new(ExpressionTree)
	}
	ct.loc.Left.Val = v1
	if ct.loc.Right == nil {
		ct.loc.Right = new(ExpressionTree)
	}
	ct.loc.Right.Val = v2
	return ct
}

type expressionTreeStack []*ExpressionTree

func (ets *expressionTreeStack) Push(v *ExpressionTree) {
	*ets = append(*ets, v)
}

func (ets *expressionTreeStack) Pop() *ExpressionTree {
	ret := (*ets)[len(*ets)-1]
	*ets = (*ets)[0 : len(*ets)-1]
	return ret
}
