%{
package gojq

//go:generate go run _tools/gen_string.go -o string.go

// Parse parses a query.
func Parse(src string) (*Query, error) {
	l := newLexer(src)
	if yyParse(l) > 0 {
		return nil, l.err
	}
	return l.result, nil
}
%}

%union {
  value    interface{}
  token    string
  operator Operator
}

%type<value> program moduleheader programbody imports import metaopt funcdefs funcdef funcdefargs query
%type<value> bindpatterns pattern arraypatterns objectpatterns objectpattern
%type<value> term suffix args ifelifs ifelse trycatch
%type<value> object objectkeyval objectval
%type<value> constterm constobject constobjectkeyvals constobjectkeyval constarray constarrayelems
%type<token> tokIdentVariable tokIdentModuleIdent tokVariableModuleVariable tokKeyword objectkey
%token<operator> tokAltOp tokUpdateOp tokDestAltOp tokOrOp tokAndOp tokCompareOp
%token<token> tokModule tokImport tokInclude tokDef tokAs tokLabel tokBreak
%token<token> tokNull tokTrue tokFalse
%token<token> tokIdent tokVariable tokModuleIdent tokModuleVariable
%token<token> tokIndex tokNumber tokString tokFormat tokInvalid
%token<token> tokIf tokThen tokElif tokElse tokEnd
%token<token> tokTry tokCatch tokReduce tokForeach
%token tokRecurse tokFuncDefPost tokTermPost tokEmptyCatch

%nonassoc tokFuncDefPost tokTermPost tokEmptyCatch
%right '|'
%left ','
%right tokAltOp
%nonassoc tokUpdateOp
%left tokOrOp
%left tokAndOp
%nonassoc tokCompareOp
%left '+' '-'
%left '*' '/' '%'
%nonassoc tokAs tokIndex '.' '?'
%nonassoc '[' tokTry tokCatch

%%

program
    : moduleheader programbody
    {
        if $1 != nil { $2.(*Query).Meta = $1.(*ConstObject) }
        yylex.(*lexer).result = $2.(*Query)
    }

moduleheader
    :
    {
        $$ = nil
    }
    | tokModule constobject ';'
    {
        $$ = $2;
    }

programbody
    : imports funcdefs
    {
        $$ = &Query{Imports: $1.([]*Import), FuncDefs: $2.([]*FuncDef), Term: &Term{Identity: true}}
    }
    | imports query
    {
        if $1 != nil { $2.(*Query).Imports = $1.([]*Import) }
        $$ = $2
    }

imports
    :
    {
        $$ = []*Import(nil)
    }
    | import imports
    {
        $$ = append([]*Import{$1.(*Import)}, $2.([]*Import)...)
    }

import
    : tokImport tokString tokAs tokIdentVariable metaopt ';'
    {
        $$ = &Import{ImportPath: $2, ImportAlias: $4, Meta: $5.(*ConstObject)}
    }
    | tokInclude tokString metaopt ';'
    {
        $$ = &Import{IncludePath: $2, Meta: $3.(*ConstObject)}
    }

metaopt
    :
    {
        $$ = (*ConstObject)(nil)
    }
    | constobject
    {
        $$ = $1
    }

funcdefs
    :
    {
        $$ = []*FuncDef(nil)
    }
    | funcdef funcdefs
    {
        $$ = append([]*FuncDef{$1.(*FuncDef)}, $2.([]*FuncDef)...)
    }

funcdef
    : tokDef tokIdent ':' query ';'
    {
        $$ = &FuncDef{Name: $2, Body: $4.(*Query)}
    }
    | tokDef tokIdent '(' funcdefargs ')' ':' query ';'
    {
        $$ = &FuncDef{$2, $4.([]string), $7.(*Query)}
    }

funcdefargs
    : tokIdentVariable
    {
        $$ = []string{$1}
    }
    | funcdefargs ';' tokIdentVariable
    {
        $$ = append($1.([]string), $3)
    }

tokIdentVariable
    : tokIdent {}
    | tokVariable {}

query
    : funcdef query %prec tokFuncDefPost
    {
        $2.(*Query).FuncDefs = append([]*FuncDef{$1.(*FuncDef)}, $2.(*Query).FuncDefs...)
        $$ = $2
    }
    | query '|' query
    {
        $$ = &Query{Left: $1.(*Query), Op: OpPipe, Right: $3.(*Query)}
    }
    | term tokAs bindpatterns '|' query
    {
        $$ = &Query{Term: $1.(*Term), Bind: &Bind{$3.([]*Pattern), $5.(*Query)}}
    }
    | tokLabel tokVariable '|' query
    {
        $$ = &Query{Term: &Term{Label: &Label{$2, $4.(*Query)}}}
    }
    | query ',' query
    {
        $$ = &Query{Left: $1.(*Query), Op: OpComma, Right: $3.(*Query)}
    }
    | query tokAltOp query
    {
        $$ = &Query{Left: $1.(*Query), Op: $2, Right: $3.(*Query)}
    }
    | query tokUpdateOp query
    {
        $$ = &Query{Left: $1.(*Query), Op: $2, Right: $3.(*Query)}
    }
    | query tokOrOp query
    {
        $$ = &Query{Left: $1.(*Query), Op: OpOr, Right: $3.(*Query)}
    }
    | query tokAndOp query
    {
        $$ = &Query{Left: $1.(*Query), Op: OpAnd, Right: $3.(*Query)}
    }
    | query tokCompareOp query
    {
        $$ = &Query{Left: $1.(*Query), Op: $2, Right: $3.(*Query)}
    }
    | query '+' query
    {
        $$ = &Query{Left: $1.(*Query), Op: OpAdd, Right: $3.(*Query)}
    }
    | query '-' query
    {
        $$ = &Query{Left: $1.(*Query), Op: OpSub, Right: $3.(*Query)}
    }
    | query '*' query
    {
        $$ = &Query{Left: $1.(*Query), Op: OpMul, Right: $3.(*Query)}
    }
    | query '/' query
    {
        $$ = &Query{Left: $1.(*Query), Op: OpDiv, Right: $3.(*Query)}
    }
    | query '%' query
    {
        $$ = &Query{Left: $1.(*Query), Op: OpMod, Right: $3.(*Query)}
    }
    | term %prec tokTermPost
    {
        $$ = &Query{Term: $1.(*Term)}
    }

bindpatterns
    : pattern
    {
        $$ = []*Pattern{$1.(*Pattern)}
    }
    | bindpatterns tokDestAltOp pattern
    {
        $$ = append($1.([]*Pattern), $3.(*Pattern))
    }

pattern
    : tokVariable
    {
        $$ = &Pattern{Name: $1}
    }
    | '[' arraypatterns ']'
    {
        $$ = &Pattern{Array: $2.([]*Pattern)}
    }
    | '{' objectpatterns '}'
    {
        $$ = &Pattern{Object: $2.([]*PatternObject)}
    }

arraypatterns
    : pattern
    {
        $$ = []*Pattern{$1.(*Pattern)}
    }
    | arraypatterns ',' pattern
    {
        $$ = append($1.([]*Pattern), $3.(*Pattern))
    }

objectpatterns
    : objectpattern
    {
        $$ = []*PatternObject{$1.(*PatternObject)}
    }
    | objectpatterns ',' objectpattern
    {
        $$ = append($1.([]*PatternObject), $3.(*PatternObject))
    }

objectpattern
    : objectkey ':' pattern
    {
        $$ = &PatternObject{Key: $1, Val: $3.(*Pattern)}
    }
    | tokString ':' pattern
    {
        $$ = &PatternObject{KeyString: $1, Val: $3.(*Pattern)}
    }
    | '(' query ')' ':' pattern
    {
        $$ = &PatternObject{Query: $2.(*Query), Val: $5.(*Pattern)}
    }
    | tokVariable
    {
        $$ = &PatternObject{KeyOnly: $1}
    }

term
    : '.'
    {
        $$ = &Term{Identity: true}
    }
    | tokRecurse
    {
        $$ = &Term{Recurse: true}
    }
    | tokIndex
    {
        $$ = &Term{Index: &Index{Name: $1}}
    }
    | '.' suffix
    {
        if $2.(*Suffix).Iter {
            $$ = &Term{Identity: true, SuffixList: []*Suffix{$2.(*Suffix)}}
        } else {
            $$ = &Term{Index: $2.(*Suffix).Index}
        }
    }
    | '.' tokString
    {
        $$ = &Term{Index: &Index{Str: $2}}
    }
    | tokNull
    {
        $$ = &Term{Null: true}
    }
    | tokTrue
    {
        $$ = &Term{True: true}
    }
    | tokFalse
    {
        $$ = &Term{False: true}
    }
    | tokIdentModuleIdent
    {
        $$ = &Term{Func: &Func{Name: $1}}
    }
    | tokIdentModuleIdent '(' args ')'
    {
        $$ = &Term{Func: &Func{Name: $1, Args: $3.([]*Query)}}
    }
    | tokVariableModuleVariable
    {
        $$ = &Term{Func: &Func{Name: $1}}
    }
    | tokNumber
    {
        $$ = &Term{Number: $1}
    }
    | tokFormat
    {
        $$ = &Term{Format: $1}
    }
    | tokFormat tokString
    {
        $$ = &Term{Format: $1, FormatStr: $2}
    }
    | tokString
    {
        $$ = &Term{Str: $1}
    }
    | '(' query ')'
    {
        $$ = &Term{Query: $2.(*Query)}
    }
    | '-' term
    {
        $$ = &Term{Unary: &Unary{OpSub, $2.(*Term)}}
    }
    | '+' term
    {
        $$ = &Term{Unary: &Unary{OpAdd, $2.(*Term)}}
    }
    | '{' object '}'
    {
        $$ = &Term{Object: &Object{$2.([]*ObjectKeyVal)}}
    }
    | '[' query ']'
    {
        $$ = &Term{Array: &Array{$2.(*Query)}}
    }
    | '[' ']'
    {
        $$ = &Term{Array: &Array{}}
    }
    | tokIf query tokThen query ifelifs ifelse tokEnd
    {
        $$ = &Term{If: &If{$2.(*Query), $4.(*Query), $5.([]*IfElif), $6.(*Query)}}
    }
    | tokTry query trycatch
    {
        $$ = &Term{Try: &Try{$2.(*Query), $3.(*Term)}}
    }
    | tokReduce term tokAs pattern '(' query ';' query ')'
    {
        $$ = &Term{Reduce: &Reduce{$2.(*Term), $4.(*Pattern), $6.(*Query), $8.(*Query)}}
    }
    | tokForeach term tokAs pattern '(' query ';' query ')'
    {
        $$ = &Term{Foreach: &Foreach{$2.(*Term), $4.(*Pattern), $6.(*Query), $8.(*Query), nil}}
    }
    | tokForeach term tokAs pattern '(' query ';' query ';' query ')'
    {
        $$ = &Term{Foreach: &Foreach{$2.(*Term), $4.(*Pattern), $6.(*Query), $8.(*Query), $10.(*Query)}}
    }
    | tokBreak tokVariable
    {
        $$ = &Term{Break: $2}
    }
    | term tokIndex
    {
        $1.(*Term).SuffixList = append($1.(*Term).SuffixList, &Suffix{Index: &Index{Name: $2}})
    }
    | term suffix
    {
        $1.(*Term).SuffixList = append($1.(*Term).SuffixList, $2.(*Suffix))
    }
    | term '?'
    {
        $1.(*Term).SuffixList = append($1.(*Term).SuffixList, &Suffix{Optional: true})
    }
    | term '.' suffix
    {
        $1.(*Term).SuffixList = append($1.(*Term).SuffixList, $3.(*Suffix))
    }
    | term '.' tokString
    {
        $1.(*Term).SuffixList = append($1.(*Term).SuffixList, &Suffix{Index: &Index{Str: $3}})
    }

tokIdentModuleIdent
    : tokIdent
    {
        $$ = $1
    }
    | tokModuleIdent
    {
        $$ = $1
    }

tokVariableModuleVariable
    : tokVariable
    {
        $$ = $1
    }
    | tokModuleVariable
    {
        $$ = $1
    }

suffix
    : '[' ']'
    {
        $$ = &Suffix{Iter: true}
    }
    | '[' query ']'
    {
        $$ = &Suffix{Index: &Index{Start: $2.(*Query)}}
    }
    | '[' query ':' ']'
    {
        $$ = &Suffix{Index: &Index{Start: $2.(*Query), IsSlice: true}}
    }
    | '[' ':' query ']'
    {
        $$ = &Suffix{Index: &Index{End: $3.(*Query)}}
    }
    | '[' query ':' query ']'
    {
        $$ = &Suffix{Index: &Index{Start: $2.(*Query), IsSlice: true, End: $4.(*Query)}}
    }

args
    : query
    {
        $$ = []*Query{$1.(*Query)}
    }
    | args ';' query
    {
        $$ = append($1.([]*Query), $3.(*Query))
    }

ifelifs
    :
    {
        $$ = []*IfElif(nil)
    }
    | tokElif query tokThen query ifelifs
    {
        $$ = append([]*IfElif{&IfElif{$2.(*Query), $4.(*Query)}}, $5.([]*IfElif)...)
    }

ifelse
    :
    {
        $$ = (*Query)(nil)
    }
    | tokElse query
    {
        $$ = $2
    }

trycatch
    : %prec tokEmptyCatch
    {
        $$ = (*Term)(nil)
    }
    | tokCatch term
    {
        $$ = $2
    }

object
    :
    {
        $$ = []*ObjectKeyVal(nil)
    }
    | objectkeyval
    {
        $$ = []*ObjectKeyVal{$1.(*ObjectKeyVal)}
    }
    | objectkeyval ',' object
    {
        $$ = append([]*ObjectKeyVal{$1.(*ObjectKeyVal)}, $3.([]*ObjectKeyVal)...)
    }

objectkeyval
    : objectkey ':' objectval
    {
        $$ = &ObjectKeyVal{Key: $1, Val: $3.(*ObjectVal)}
    }
    | tokString ':' objectval
    {
        $$ = &ObjectKeyVal{KeyString: $1, Val: $3.(*ObjectVal)}
    }
    | '(' query ')' ':' objectval
    {
        $$ = &ObjectKeyVal{Query: $2.(*Query), Val: $5.(*ObjectVal)}
    }
    | objectkey
    {
        $$ = &ObjectKeyVal{KeyOnly: &$1}
    }
    | tokString
    {
        $$ = &ObjectKeyVal{KeyOnlyString: $1}
    }

objectkey
    : tokIdent {}
    | tokVariable {}
    | tokKeyword {}

objectval
    : term
    {
        $$ = &ObjectVal{[]*Query{&Query{Term: $1.(*Term)}}}
    }
    | term '|' objectval
    {
        $$ = &ObjectVal{append([]*Query{&Query{Term: $1.(*Term)}}, $3.(*ObjectVal).Queries...)}
    }

constterm
    : constobject
    {
        $$ = &ConstTerm{Object: $1.(*ConstObject)}
    }
    | constarray
    {
        $$ = &ConstTerm{Array: $1.(*ConstArray)}
    }
    | tokNumber
    {
        $$ = &ConstTerm{Number: $1}
    }
    | tokString
    {
        $$ = &ConstTerm{Str: $1}
    }
    | tokNull
    {
        $$ = &ConstTerm{Null: true}
    }
    | tokTrue
    {
        $$ = &ConstTerm{True: true}
    }
    | tokFalse
    {
        $$ = &ConstTerm{False: true}
    }

constobject
    : '{' constobjectkeyvals '}'
    {
        $$ = &ConstObject{$2.([]*ConstObjectKeyVal)}
    }

constobjectkeyvals
    :
    {
        $$ = []*ConstObjectKeyVal(nil)
    }
    | constobjectkeyval
    {
        $$ = []*ConstObjectKeyVal{$1.(*ConstObjectKeyVal)}
    }
    | constobjectkeyval ',' constobjectkeyvals
    {
        $$ = append([]*ConstObjectKeyVal{$1.(*ConstObjectKeyVal)}, $3.([]*ConstObjectKeyVal)...)
    }

constobjectkeyval
    : tokIdent ':' constterm
    {
        $$ = &ConstObjectKeyVal{Key: $1, Val: $3.(*ConstTerm)}
    }
    | tokKeyword ':' constterm
    {
        $$ = &ConstObjectKeyVal{Key: $1, Val: $3.(*ConstTerm)}
    }
    | tokString ':' constterm
    {
        $$ = &ConstObjectKeyVal{KeyString: $1, Val: $3.(*ConstTerm)}
    }

constarray
    : '[' ']'
    {
        $$ = &ConstArray{}
    }
    | '[' constarrayelems ']'
    {
        $$ = &ConstArray{$2.([]*ConstTerm)}
    }

constarrayelems
    : constterm
    {
        $$ = []*ConstTerm{$1.(*ConstTerm)}
    }
    | constarrayelems ',' constterm
    {
        $$ = append($1.([]*ConstTerm), $3.(*ConstTerm))
    }

tokKeyword
    : tokOrOp {}
    | tokAndOp {}
    | tokModule {}
    | tokImport {}
    | tokInclude {}
    | tokDef {}
    | tokAs {}
    | tokLabel {}
    | tokBreak {}
    | tokNull {}
    | tokTrue {}
    | tokFalse {}
    | tokIf {}
    | tokThen {}
    | tokElif {}
    | tokElse {}
    | tokEnd {}
    | tokTry {}
    | tokCatch {}
    | tokReduce {}
    | tokForeach {}

%%
