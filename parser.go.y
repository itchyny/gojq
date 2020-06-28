%{
package gojq
%}

%union {
  query *Query
  importstmt *Import
  funcdef *FuncDef
  patterns []*Pattern
  pattern *Pattern
  objectpatterns []PatternObject
  objectpattern PatternObject
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

%type<query> program program1 program2 query ifelse
%type<importstmt> import
%type<funcdef> funcdef
%type<tokens> funcdefargs
%type<patterns> bindpatterns arraypatterns
%type<objectpatterns> objectpatterns
%type<objectpattern> objectpattern
%type<pattern> pattern
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
%type<token> tokIdentVariable tokIdentModuleIdent tokVariableModuleVariable tokKeyword objectkey
%token<operator> tokAltOp tokUpdateOp tokDestAltOp tokOrOp tokAndOp tokCompareOp
%token<token> tokModule tokImport tokInclude tokDef tokAs tokLabel tokBreak
%token<token> tokNull tokTrue tokFalse
%token<token> tokIdent tokVariable tokModuleIdent tokModuleVariable
%token<token> tokIndex tokNumber tokString tokFormat tokInvalid
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

program
    : moduleheader program1
    {
        $2.Meta = $1
        yylex.(*lexer).result = $2
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

program1
    : import program1
    {
        $2.Imports = append([]*Import{$1}, $2.Imports...)
        $$ = $2
    }
    | program2
    {
        $$ = $1
    }

import
    : tokImport tokString tokAs tokIdentVariable metaopt ';'
    {
        $$ = &Import{ImportPath: $2, ImportAlias: $4, Meta: $5}
    }
    | tokInclude tokString metaopt ';'
    {
        $$ = &Import{IncludePath: $2, Meta: $3}
    }

metaopt
    :
    {
        $$ = nil
    }
    | constobject
    {
        $$ = $1
    }

program2
    :
    {
        $$ = &Query{Term: &Term{Identity: true}}
    }
    | funcdef program2
    {
        $2.FuncDefs = append([]*FuncDef{$1}, $2.FuncDefs...)
        $$ = $2
    }
    | query
    {
        $$ = $1
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
    : tokIdent {}
    | tokVariable {}

query
    : funcdef query
    {
        $2.FuncDefs = append([]*FuncDef{$1}, $2.FuncDefs...)
        $$ = $2
    }
    | query '|' query
    {
        $$ = &Query{Left: $1, Op: OpPipe, Right: $3}
    }
    | term tokAs bindpatterns '|' query
    {
        $$ = &Query{Term: $1, Bind: &Bind{$3, $5}}
    }
    | tokLabel tokVariable '|' query
    {
        $$ = &Query{Label: &Label{$2, $4}}
    }
    | query ',' query
    {
        $$ = &Query{Left: $1, Op: OpComma, Right: $3}
    }
    | query tokAltOp query
    {
        $$ = &Query{Left: $1, Op: $2, Right: $3}
    }
    | query tokUpdateOp query
    {
        $$ = &Query{Left: $1, Op: $2, Right: $3}
    }
    | query tokOrOp query
    {
        $$ = &Query{Left: $1, Op: OpOr, Right: $3}
    }
    | query tokAndOp query
    {
        $$ = &Query{Left: $1, Op: OpAnd, Right: $3}
    }
    | query tokCompareOp query
    {
        $$ = &Query{Left: $1, Op: $2, Right: $3}
    }
    | query '+' query
    {
        $$ = &Query{Left: $1, Op: OpAdd, Right: $3}
    }
    | query '-' query
    {
        $$ = &Query{Left: $1, Op: OpSub, Right: $3}
    }
    | query '*' query
    {
        $$ = &Query{Left: $1, Op: OpMul, Right: $3}
    }
    | query '/' query
    {
        $$ = &Query{Left: $1, Op: OpDiv, Right: $3}
    }
    | query '%' query
    {
        $$ = &Query{Left: $1, Op: OpMod, Right: $3}
    }
    | term
    {
        $$ = &Query{Term: $1}
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
    : objectkey ':' pattern
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
        $$ = &Term{Func: &Func{Name: $1, Args: $3}}
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
        $$ = nil
    }
    | tokElif query tokThen query ifelifs
    {
        $$ = append([]IfElif{IfElif{$2, $4}}, $5...)
    }

ifelse
    :
    {
        $$ = nil
    }
    | tokElse query
    {
        $$ = $2
    }

trycatch
    :
    {
        $$ = nil
    }
    | tokCatch term
    {
        $$ = $2
    }

object
    :
    {
        $$ = nil
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
    : objectkey ':' objectval
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
        $$ = &ObjectVal{[]*Query{&Query{Term: $1}}}
    }
    | term '|' objectval
    {
        $$ = &ObjectVal{append([]*Query{&Query{Term: $1}}, $3.Queries...)}
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
        $$ = &ConstObject{$2}
    }

constobjectkeyvals
    :
    {
        $$ = nil
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
    | tokKeyword ':' constterm
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
