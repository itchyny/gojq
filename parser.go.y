%{
package gojq
%}

%union {
  query *Query
  term  *Term
  args  []*Query
  token string
}

%type<query> program query
%type<term> term
%type<args> args
%token <token> tokIdent tokIndex tokNumber tokInvalid
%token tokRecurse

%right '|'

%%

program
    : query
    {
        $$ = $1
        yylex.(*lexer).result = $$
    }

query
    : term
    {
        $$ = $1.toQuery()
    }
    | query '|' query
    {
        $1.Commas = append($1.Commas, $3.Commas...)
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
    | '[' query ']'
    {
        $$ = &Term{Array: &Array{$2}}
    }
    | '[' ']'
    {
        $$ = &Term{Array: &Array{}}
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

%%
