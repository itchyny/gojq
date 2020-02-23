include "5";
import "6" as bar;
def f: [., . * 2, h, (bar::i|.+1), bar::g];
def g: { foo: . };
