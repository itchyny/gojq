%{
package gojq
%}

%union {
  query *Query
  term  *Term
}

%type<query> program query
%type<term> term

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

term
    : '.'
    {
        $$ = &Term{Identity: true}
    }

%%
