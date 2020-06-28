%{
package gojq
%}

%union {
  module *Module
  imports []*Import
  funcdefs []*FuncDef
  funcdef *FuncDef
  query *Query
  comma *Comma
  filter *Filter
  alt *Alt
  expr *Expr
  patterns []*Pattern
  pattern *Pattern
  objectpatterns []PatternObject
  objectpattern PatternObject
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
  tokens []string
  token string
  constterm *ConstTerm
  constobject *ConstObject
  constobjectkeyvals []ConstObjectKeyVal
  constobjectkeyval *ConstObjectKeyVal
  constarray *ConstArray
  constarrayelems []*ConstTerm
}

%type<module> module
%type<imports> imports
%type<funcdefs> funcdefs
%type<funcdef> funcdef
%type<tokens> funcdefargs
%type<query> modulebody query ifelse
%type<comma> comma
%type<filter> filter
%type<alt> alt
%type<expr> expr
%type<patterns> bindpatterns arraypatterns
%type<objectpatterns> objectpatterns
%type<objectpattern> objectpattern
%type<pattern> pattern
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
%type<constterm> constterm
%type<constobject> moduleheader constobject metaopt
%type<constobjectkeyvals> constobjectkeyvals
%type<constobjectkeyval> constobjectkeyval
%type<constarray> constarray
%type<constarrayelems> constarrayelems
%type<token> tokIdentVariable
%token<operator> tokAltOp tokUpdateOp tokDestAltOp tokOrOp tokAndOp tokCompareOp
%token<token> tokModule tokImport tokInclude tokDef tokAs tokLabel tokBreak
%token<token> tokIdent tokVariable tokIndex tokNumber tokString tokFormat tokInvalid
%token<token> tokIf tokThen tokElif tokElse tokEnd
%token<token> tokTry tokCatch tokReduce tokForeach
%token tokRecurse

%right '|'
%left ','
%right tokAltOp
%nonassoc tokUpdateOp
%left tokOrOp
%left tokAndOp
%nonassoc tokCompareOp
%left '+' '-'
%left '*' '/' '%'

%%

module
    : moduleheader imports funcdefs modulebody
    {
        yylex.(*lexer).result = &Module{$1, $2, $3, $4}
    }

moduleheader
    :
    {
    }
    | tokModule constobject ';'
    {
        $$ = $2;
    }

imports
    :
    {
    }
    | tokImport tokString tokAs tokIdentVariable metaopt ';' imports
    {
        $$ = append([]*Import{&Import{ImportPath: $2, ImportAlias: $4, Meta: $5}}, $7...)
    }
    | tokInclude tokString metaopt ';' imports
    {
        $$ = append([]*Import{&Import{IncludePath: $2, Meta: $3}}, $5...)
    }

metaopt
    :
    {
    }
    | constobject
    {
        $$ = $1
    }

funcdefs
    :
    {
    }
    | funcdef funcdefs
    {
        $$ = append([]*FuncDef{$1}, $2...)
    }

funcdef
    : tokDef tokIdent ':' query ';'
    {
        $$ = &FuncDef{Name: $2, Body: $4}
    }
    | tokDef tokIdent '(' funcdefargs ')' ':' query ';'
    {
        $$ = &FuncDef{$2, $4, $7}
    }

funcdefargs
    : tokIdentVariable
    {
        $$ = []string{$1}
    }
    | funcdefargs ';' tokIdent
    {
        $$ = append($1, $3)
    }
    | funcdefargs ';' tokVariable
    {
        $$ = append($1, $3)
    }

tokIdentVariable
    : tokIdent
    {
        $$ = $1
    }
    | tokVariable
    {
        $$ = $1
    }

modulebody
    :
    {
        $$ = (&Term{Identity: true}).toQuery()
    }
    | query
    {
        $$ = $1
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
    | imports query
    {
        $2.Imports = $1
        $$ = $2
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
    | funcdef filter
    {
        $2.FuncDefs = append([]*FuncDef{$1}, $2.FuncDefs...)
        $$ = $2
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
    | logic tokUpdateOp alt
    {
        $$ = &Expr{Logic: $1, UpdateOp: $2, Update: $3}
    }
    | logic tokAs bindpatterns '|' query
    {
        $$ = &Expr{Logic: $1, Bind: &Bind{$3, $5}}
    }
    | tokLabel tokVariable '|' query
    {
        $$ = &Expr{Label: &Label{$2, $4}}
    }

bindpatterns
    : pattern
    {
        $$ = []*Pattern{$1}
    }
    | bindpatterns tokDestAltOp pattern
    {
        $$ = append($1, $3)
    }

pattern
    : tokVariable
    {
        $$ = &Pattern{Name: $1}
    }
    | '[' arraypatterns ']'
    {
        $$ = &Pattern{Array: $2}
    }
    | '{' objectpatterns '}'
    {
        $$ = &Pattern{Object: $2}
    }

arraypatterns
    : pattern
    {
        $$ = []*Pattern{$1}
    }
    | arraypatterns ',' pattern
    {
        $$ = append($1, $3)
    }

objectpatterns
    : objectpattern
    {
        $$ = []PatternObject{$1}
    }
    | objectpatterns ',' objectpattern
    {
        $$ = append($1, $3)
    }

objectpattern
    : tokIdent ':' pattern
    {
        $$ = PatternObject{Key: $1, Val: $3}
    }
    | tokVariable ':' pattern
    {
        $$ = PatternObject{Key: $1, Val: $3}
    }
    | tokString ':' pattern
    {
        $$ = PatternObject{KeyString: $1, Val: $3}
    }
    | '(' query ')' ':' pattern
    {
        $$ = PatternObject{Query: $2, Val: $5}
    }
    | tokVariable
    {
        $$ = PatternObject{KeyOnly: $1}
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
    | '.' tokString
    {
        $$ = &Term{Index: &Index{Str: $2}}
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
    | tokReduce term tokAs pattern '(' query ';' query ')'
    {
        $$ = &Term{Reduce: &Reduce{$2, $4, $6, $8}}
    }
    | tokForeach term tokAs pattern '(' query ';' query ')'
    {
        $$ = &Term{Foreach: &Foreach{$2, $4, $6, $8, nil}}
    }
    | tokForeach term tokAs pattern '(' query ';' query ';' query ')'
    {
        $$ = &Term{Foreach: &Foreach{$2, $4, $6, $8, $10}}
    }
    | tokBreak tokVariable
    {
        $$ = &Term{Break: $2}
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
    | term '.' tokString
    {
        $1.SuffixList = append($1.SuffixList, &Suffix{Index: &Index{Str: $3}})
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
    | tokString ':' objectval
    {
        $$ = &ObjectKeyVal{KeyString: $1, Val: $3}
    }
    | '(' query ')' ':' objectval
    {
        $$ = &ObjectKeyVal{Query: $2, Val: $5}
    }
    | tokIdent
    {
        $$ = &ObjectKeyVal{KeyOnly: &$1}
    }
    | tokVariable
    {
        $$ = &ObjectKeyVal{KeyOnly: &$1}
    }
    | tokString
    {
        $$ = &ObjectKeyVal{KeyOnlyString: $1}
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

constterm
    : constobject
    {
        $$ = &ConstTerm{Object: $1}
    }
    | constarray
    {
        $$ = &ConstTerm{Array: $1}
    }
    | tokNumber
    {
        $$ = &ConstTerm{Number: $1}
    }
    | tokString
    {
        $$ = &ConstTerm{Str: $1}
    }

constobject
    : '{' constobjectkeyvals '}'
    {
        $$ = &ConstObject{$2}
    }

constobjectkeyvals
    :
    {
    }
    | constobjectkeyval
    {
        $$ = []ConstObjectKeyVal{*$1}
    }
    | constobjectkeyval ',' constobjectkeyvals
    {
        $$ = append([]ConstObjectKeyVal{*$1}, $3...)
    }

constobjectkeyval
    : tokIdent ':' constterm
    {
        $$ = &ConstObjectKeyVal{Key: $1, Val: $3}
    }
    | tokString ':' constterm
    {
        $$ = &ConstObjectKeyVal{KeyString: $1, Val: $3}
    }

constarray
    : '[' ']'
    {
        $$ = &ConstArray{}
    }
    | '[' constarrayelems ']'
    {
        $$ = &ConstArray{$2}
    }

constarrayelems
    : constterm
    {
        $$ = []*ConstTerm{$1}
    }
    | constarrayelems ',' constterm
    {
        $$ = append($1, $3)
    }

%%
