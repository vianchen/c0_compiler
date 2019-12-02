package analyzer

import (
	"c0_compiler/internal/parser"
	"c0_compiler/internal/token"
	"fmt"
)

type Parser = parser.Parser
type Token = token.Token

var globalLineCount = 0
var globalColumnCount = 0

// The only error this function will throw is NoMoreTokens so it's safe to check `err != nil` directly without
// specifying the kind of error.
func getNextToken() (res *Token, err error) {
	if !globalParser.HasNextToken() {
		res, err = nil, &Error{NoMoreTokens, globalLineCount, globalColumnCount}
		return
	}
	res, err = globalParser.NextToken(), nil
	globalLineCount, globalColumnCount = res.Line, res.Column
	return
}

func getCurrentPos() int {
	return globalParser.CurrentHead()
}

func resetHeadTo(pos int) {
	thatToken := globalParser.ResetHeadTo(pos)
	globalColumnCount, globalLineCount = thatToken.Column, thatToken.Line
}

// Print all the tokens the parser generated directly to stdout.
func BindToStdOut(parser *Parser) {
	for parser.HasNextToken() {
		fmt.Println(parser.NextToken())
	}
}
