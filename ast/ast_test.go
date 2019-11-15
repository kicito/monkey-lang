package ast

import (
	"testing"

	"github.com/kicito/monkey/token"
)

func TestString(t *testing.T) {
	program := &Program{
		Statements: []Statement{
			&LetStatement{
				Token: token.Token{Type: token.LET, Literal: "let"},
				Name: &Identifier{
					Token: token.Token{Type: token.IDENT, Literal: "myVar"},
					Value: "myVar",
				},
				Value: &Identifier{
					Token: token.Token{Type: token.IDENT, Literal: "anotherVar"},
					Value: "anotherVar",
				},
			},
		},
	}

	if program.String() != "let myVar = anotherVar;" {
		t.Errorf("program.String() wrong, got=%q", program.String())
	}
}

func TestImport(t *testing.T) {
	program := &Program{
		Statements: []Statement{
			&ImportStatement{
				Token: token.Token{Type: token.IMPORT, Literal: "import"},
				LocalName: &Identifier{
					Token: token.Token{Type: token.IDENT, Literal: "A"},
					Value: "A",
				},
				Target: &StringLiteral{
					Token: token.Token{Type: token.STRING, Literal: "./A.mon"},
					Value: "./A.mon",
				},
			},
		},
	}

	if program.String() != "import A from ./A.mon" {
		t.Errorf("program.String() wrong, got=%q", program.String())
	}
}
