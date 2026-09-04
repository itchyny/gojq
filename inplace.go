package gojq

// Static analysis deciding whether the accumulated value of a reduce
// expression may be updated in place; see compiler#compileReduce. The checks
// are conservative: failing them only keeps the previous copying behavior.

// inPlaceUpdate reports whether the query is a single assignment applied to
// the input value, and returns its operands.
func (e *Query) inPlaceUpdate() (l, r *Query, op Operator, ok bool) {
	if e.Term != nil || len(e.FuncDefs) > 0 {
		return nil, nil, 0, false
	}
	switch e.Op {
	case OpModify:
		// _modify releases the allocator before calling the update function.
		return e.Left, e.Right, e.Op, true
	case OpAssign:
		// A constant path compiles to setpath, which uses no allocator.
		if e.Left.toIndices(nil) != nil {
			return nil, nil, 0, false
		}
		fallthrough
	case OpUpdateAdd, OpUpdateSub, OpUpdateMul,
		OpUpdateDiv, OpUpdateMod, OpUpdateAlt:
		// The right-hand side must neither observe the input value, which it
		// could otherwise alias, nor yield multiple values, which would run
		// the assignment against an already modified input.
		if !e.Right.readsInput() && e.Right.singleValued() {
			return e.Left, e.Right, e.Op, true
		}
	}
	return nil, nil, 0, false
}

// readsInput reports whether evaluating the query may observe its input value.
// It answers true unless it can prove otherwise.
func (e *Query) readsInput() bool {
	if len(e.FuncDefs) > 0 {
		return true
	}
	if e.Term != nil {
		return e.Term.readsInput()
	}
	switch e.Op {
	case OpPipe:
		// Binding variables also exposes the input to the right-hand side.
		if len(e.Patterns) > 0 {
			return e.Left.readsInput() || e.Right.readsInput()
		}
		return e.Left.readsInput()
	case OpComma, OpAdd, OpSub, OpMul, OpDiv, OpMod, OpEq, OpNe,
		OpGt, OpLt, OpGe, OpLe, OpAnd, OpOr, OpAlt:
		return e.Left.readsInput() || e.Right.readsInput()
	default:
		return true
	}
}

// singleValued reports whether the query yields at most one value.
// It answers false unless it can prove otherwise.
func (e *Query) singleValued() bool {
	if len(e.FuncDefs) > 0 {
		return false
	}
	if e.Term != nil {
		return e.Term.singleValued()
	}
	switch e.Op {
	case OpPipe:
		return len(e.Patterns) == 0 &&
			e.Left.singleValued() && e.Right.singleValued()
	case OpAdd, OpSub, OpMul, OpDiv, OpMod, OpEq, OpNe,
		OpGt, OpLt, OpGe, OpLe, OpAnd, OpOr:
		return e.Left.singleValued() && e.Right.singleValued()
	default:
		return false
	}
}

func (e *Term) readsInput() bool {
	for _, s := range e.SuffixList {
		if s.readsInput() {
			return true
		}
	}
	switch e.Type {
	case TermTypeNull, TermTypeTrue, TermTypeFalse, TermTypeNumber:
		return false
	case TermTypeString:
		return e.Str.readsInput()
	case TermTypeObject:
		for _, kv := range e.Object.KeyVals {
			if kv.readsInput() {
				return true
			}
		}
		return false
	case TermTypeArray:
		return e.Array.Query != nil && e.Array.Query.readsInput()
	case TermTypeUnary:
		return e.Unary.Term.readsInput()
	case TermTypeFunc:
		// Any function may be redefined, so only $x is known not to read it.
		return e.Func.Name[0] != '$' || len(e.Func.Args) > 0
	case TermTypeQuery:
		return e.Query.readsInput()
	default:
		return true
	}
}

func (e *Term) singleValued() bool {
	for _, s := range e.SuffixList {
		if !s.singleValued() {
			return false
		}
	}
	switch e.Type {
	case TermTypeNull, TermTypeTrue, TermTypeFalse, TermTypeNumber:
		return true
	case TermTypeString:
		return e.Str.singleValued()
	case TermTypeObject:
		for _, kv := range e.Object.KeyVals {
			if !kv.singleValued() {
				return false
			}
		}
		return true
	case TermTypeArray:
		return true
	case TermTypeUnary:
		return e.Unary.Term.singleValued()
	case TermTypeFunc:
		return e.Func.Name[0] == '$' && len(e.Func.Args) == 0
	case TermTypeQuery:
		return e.Query.singleValued()
	default:
		return false
	}
}

func (e *Suffix) readsInput() bool {
	// Only an index is evaluated against the input value.
	return e.Index != nil && e.Index.readsInput()
}

func (e *Suffix) singleValued() bool {
	return !e.Iter && (e.Index == nil || e.Index.singleValued())
}

func (e *Index) readsInput() bool {
	if e.Str != nil {
		return e.Str.readsInput()
	}
	return e.Start != nil && e.Start.readsInput() ||
		e.End != nil && e.End.readsInput()
}

func (e *Index) singleValued() bool {
	if e.Str != nil {
		return e.Str.singleValued()
	}
	return (e.Start == nil || e.Start.singleValued()) &&
		(e.End == nil || e.End.singleValued())
}

func (e *String) readsInput() bool {
	for _, q := range e.Queries {
		if q.readsInput() {
			return true
		}
	}
	return false
}

func (e *String) singleValued() bool {
	for _, q := range e.Queries {
		if !q.singleValued() {
			return false
		}
	}
	return true
}

func (e *ObjectKeyVal) readsInput() bool {
	if e.Val == nil {
		// {foo} reads the input value, {$foo} does not.
		return e.Key == "" || e.Key[0] != '$'
	}
	if e.KeyString != nil && e.KeyString.readsInput() ||
		e.KeyQuery != nil && e.KeyQuery.readsInput() {
		return true
	}
	return e.Val.readsInput()
}

func (e *ObjectKeyVal) singleValued() bool {
	if e.KeyString != nil && !e.KeyString.singleValued() ||
		e.KeyQuery != nil && !e.KeyQuery.singleValued() {
		return false
	}
	return e.Val == nil || e.Val.singleValued()
}
