%{
package gojq
%}

%union {
  query *Query
  comma *Comma
  filter *Filter
  alt *Alt
  expr *Expr
  logic *Logic
  andexpr *AndExpr
  compare *Compare
  arith *Arith
  factor *Factor
  term  *Term
  suffix *Suffix
  args  []*Query
  ifelifs []IfElif
  object []ObjectKeyVal
  objectkeyval *ObjectKeyVal
  objectval *ObjectVal
  operator Operator
  token string
}

%type<query> program query ifelse
%type<comma> comma
%type<filter> filter
%type<alt> alt
%type<expr> expr
%type<logic> logic
%type<andexpr> andexpr
%type<compare> compare
%type<arith> arith
%type<factor> factor
%type<term> term trycatch
%type<suffix> suffix
%type<args> args
%type<ifelifs> ifelifs
%type<object> object
%type<objectkeyval> objectkeyval
%type<objectval> objectval
%token<operator> tokAltOp tokOrOp tokAndOp tokCompareOp
%token<token> tokIdent tokVariable tokIndex tokNumber tokInvalid
%token<token> tokIf tokThen tokElif tokElse tokEnd
%token<token> tokTry tokCatch
%token tokRecurse

%right '|'
%left ','
%right tokAltOp
%left tokOrOp
%left tokAndOp
%nonassoc tokCompareOp
%left '+' '-'
%left '*' '/' '%'

%%

program
    : query
    {
        $$ = $1
        yylex.(*lexer).result = $$
    }

query
    : comma
    {
        $$ = &Query{Commas: []*Comma{$1}}
    }
    | query '|' query
    {
        $1.Commas = append($1.Commas, $3.Commas...)
    }

comma
    : filter
    {
        $$ = &Comma{Filters: []*Filter{$1}}
    }
    | comma ',' filter
    {
        $1.Filters = append($1.Filters, $3)
    }

filter
    : alt
    {
        $$ = &Filter{Alt: $1}
    }

alt
    : expr
    {
        $$ = &Alt{Left: $1}
    }
    | alt tokAltOp expr
    {
        $1.Right = append($1.Right, AltRight{$2, $3})
    }

expr
    : logic
    {
        $$ = &Expr{Logic: $1}
    }

logic
    : andexpr
    {
        $$ = &Logic{Left: $1}
    }
    | logic tokOrOp andexpr
    {
        $1.Right = append($1.Right, LogicRight{$2, $3})
    }

andexpr
    : compare
    {
        $$ = &AndExpr{Left: $1}
    }
    | andexpr tokAndOp compare
    {
        $1.Right = append($1.Right, AndExprRight{$2, $3})
    }

compare
    : arith
    {
        $$ = &Compare{Left: $1}
    }
    | arith tokCompareOp arith
    {
        $$ = &Compare{Left: $1, Right: &CompareRight{$2, $3}}
    }

arith
    : factor
    {
        $$ = &Arith{Left: $1}
    }
    | arith '+' factor
    {
        $1.Right = append($1.Right, ArithRight{OpAdd, $3})
    }
    | arith '-' factor
    {
        $1.Right = append($1.Right, ArithRight{OpSub, $3})
    }

factor
    : term
    {
        $$ = &Factor{Left: $1}
    }
    | factor '*' term
    {
        $1.Right = append($1.Right, FactorRight{OpMul, $3})
    }
    | factor '/' term
    {
        $1.Right = append($1.Right, FactorRight{OpDiv, $3})
    }
    | factor '%' term
    {
        $1.Right = append($1.Right, FactorRight{OpMod, $3})
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
        if $2.Iter {
            $$ = &Term{Identity: true, SuffixList: []*Suffix{$2}}
        } else {
            $$ = &Term{Index: $2.SuffixIndex.toIndex()}
        }
    }
    | tokIdent
    {
        switch $1 {
        case "null":
            $$ = &Term{Null: true}
        case "true":
            $$ = &Term{True: true}
        case "false":
            $$ = &Term{False: true}
        default:
            $$ = &Term{Func: &Func{Name: $1}}
        }
    }
    | tokIdent '(' args ')'
    {
        $$ = &Term{Func: &Func{Name: $1, Args: $3}}
    }
    | tokVariable
    {
        $$ = &Term{Func: &Func{Name: $1}}
    }
    | tokNumber
    {
        $$ = &Term{Number: $1}
    }
    | '(' query ')'
    {
        $$ = &Term{Query: $2}
    }
    | '-' term
    {
        $$ = &Term{Unary: &Unary{OpSub, $2}}
    }
    | '+' term
    {
        $$ = &Term{Unary: &Unary{OpAdd, $2}}
    }
    | '{' object '}'
    {
        $$ = &Term{Object: &Object{$2}}
    }
    | '[' query ']'
    {
        $$ = &Term{Array: &Array{$2}}
    }
    | '[' ']'
    {
        $$ = &Term{Array: &Array{}}
    }
    | tokIf query tokThen query ifelifs ifelse tokEnd
    {
        $$ = &Term{If: &If{$2, $4, $5, $6}}
    }
    | tokTry query trycatch
    {
        $$ = &Term{Try: &Try{$2, $3}}
    }
    | term tokIndex
    {
        $1.SuffixList = append($1.SuffixList, &Suffix{Index: &Index{Name: $2}})
    }
    | term suffix
    {
        $1.SuffixList = append($1.SuffixList, $2)
    }
    | term '?'
    {
        $1.SuffixList = append($1.SuffixList, &Suffix{Optional: true})
    }
    | term '.' suffix
    {
        $1.SuffixList = append($1.SuffixList, $3)
    }

suffix
    : '[' ']'
    {
        $$ = &Suffix{Iter: true}
    }
    | '[' query ']'
    {
        $$ = &Suffix{SuffixIndex: &SuffixIndex{Start: $2}}
    }
    | '[' query ':' ']'
    {
        $$ = &Suffix{SuffixIndex: &SuffixIndex{Start: $2, IsSlice: true}}
    }
    | '[' ':' query ']'
    {
        $$ = &Suffix{SuffixIndex: &SuffixIndex{End: $3}}
    }
    | '[' query ':' query ']'
    {
        $$ = &Suffix{SuffixIndex: &SuffixIndex{Start: $2, IsSlice: true, End: $4}}
    }

args
    : query
    {
        $$ = []*Query{$1}
    }
    | args ';' query
    {
        $$ = append($1, $3)
    }

ifelifs
    :
    {
    }
    | tokElif query tokThen query ifelifs
    {
        $$ = append([]IfElif{IfElif{$2, $4}}, $5...)
    }

ifelse
    :
    {
    }
    | tokElse query
    {
        $$ = $2
    }

trycatch
    :
    {
    }
    | tokCatch term
    {
        $$ = $2
    }

object
    :
    {
    }
    | objectkeyval
    {
        $$ = []ObjectKeyVal{*$1}
    }
    | objectkeyval ',' object
    {
        $$ = append([]ObjectKeyVal{*$1}, $3...)
    }

objectkeyval
    : tokIdent ':' objectval
    {
        $$ = &ObjectKeyVal{Key: $1, Val: $3}
    }
    | '(' query ')' ':' objectval
    {
        $$ = &ObjectKeyVal{Query: $2, Val: $5}
    }
    | tokIdent
    {
        $$ = &ObjectKeyVal{KeyOnly: &$1}
    }

objectval
    : term
    {
        $$ = &ObjectVal{[]*Alt{$1.toFilter().Alt}}
    }
    | term '|' objectval
    {
        $$ = &ObjectVal{append([]*Alt{$1.toFilter().Alt}, $3.Alts...)}
    }

%%
