package analyzer

import (
	"bufio"
	"c0_compiler/internal/token"
	"c0_compiler/internal/vm"
	"fmt"
)

var globalParser *Parser
var globalWriter *bufio.Writer
var globalSymbolTable, currentSymbolTable *symbolTable
var globalStart, currentFunction *functionInfo

func analyzePrimaryExpression() *Error {
	// <primary-expression> ::=
	//     '('<expression>')'
	//    | <identifier>
	//    | <integer-literal>
	//    | <function-call>

	next, err := getNextToken()
	if err != nil {
		return errorOf(IncompleteExpression)
	}

	// '('<expression>')'
	if next.Kind == token.LeftParenthesis {
		if err := analyzeExpression(); err != nil {
			return err
		}
		next, err = getNextToken()
		if err != nil {
			return errorOf(IncompleteExpression)
		}
		if next.Kind != token.RightParenthesis {
			return errorOf(IllegalExpression)
		}
	} else if next.Kind == token.Identifier { // <identifier> || <function-call>
		identifier := next.Value.(string)
		sb := currentSymbolTable.getSymbol(identifier)
		if sb == nil {
			return errorOf(UndefinedIdentifier)
		}
		if sb.isCallable {
			if err := analyzeFunctionCall(); err != nil {
				return err
			}
		} else {
			currentFunction.addAnInstructionWithOneOperand(vm.Ipush, sb.address)
			currentFunction.addAnInstructionWithNoOperands(vm.Iload)
		}
	} else if next.Kind == token.IntegerLiteral {
		// <integer-literal>
		currentFunction.addAnInstructionWithOneOperand(vm.Ipush, int(next.Value.(int64)))
	} else {
		_ = putBackAToken()
		return errorOf(IllegalExpression)
	}
	return nil
}

func analyzeUnaryExpression() *Error {
	// <unary-expression> ::= [<unary-operator>]<primary-expression>

	// [<unary-operator>]
	shouldBeNegated := false
	next, err := getNextToken()
	if err != nil {
		return errorOf(IncompleteExpression)
	}
	if next.IsAnUnaryOperator() {
		if next.Kind == token.MinusSign {
			shouldBeNegated = true
		}
	} else {
		_ = putBackAToken()
	}

	// <primary-expression>
	if err := analyzePrimaryExpression(); err != nil {
		return err
	}

	if shouldBeNegated {
		currentFunction.addAnInstructionWithNoOperands(vm.Ineg)
	}
	return nil
}

func analyzeMultiplicativeExpression() *Error {
	// <multiplicative-expression> ::= <unary-expression>{<multiplicative-operator><unary-expression>}

	// <unary-expression>
	if err := analyzeUnaryExpression(); err != nil {
		return err
	}

	// {<multiplicative-operator><unary-expression>}
	for {
		next, err := getNextToken()
		if err != nil || !next.IsAMultiplicativeOperator() {
			if err == nil {
				_ = putBackAToken()
			}
			return nil
		}
		if err := analyzeUnaryExpression(); err != nil {
			return err
		}
		if next.Kind == token.MultiplicationSign {
			currentFunction.addAnInstructionWithNoOperands(vm.Imul)
		} else {
			currentFunction.addAnInstructionWithNoOperands(vm.Idiv)
		}
	}
}

func analyzeAdditiveExpression() *Error {
	// <additive-expression> ::= <multiplicative-expression>{<additive-operator><multiplicative-expression>}

	// <multiplicative-expression>
	if err := analyzeMultiplicativeExpression(); err != nil {
		return err
	}

	// {<additive-operator><multiplicative-expression>}
	for {
		next, err := getNextToken()
		if err != nil || !next.IsAnAdditiveOperator() {
			if err == nil {
				_ = putBackAToken()
			}
			return nil
		}
		operator := next.Kind
		if err := analyzeMultiplicativeExpression(); err != nil {
			return err
		}

		if operator == token.PlusSign {
			currentFunction.addAnInstructionWithNoOperands(vm.Iadd)
		} else {
			currentFunction.addAnInstructionWithNoOperands(vm.Isub)
		}
	}
}

func analyzeExpression() *Error {
	// <expression> ::= <additive-expression>
	return analyzeAdditiveExpression()
}

func analyzeInitDeclarator(isConstant bool, declaredType int) *Error {
	// <init-declarator> ::= <identifier>[<initializer>]

	// <identifier>
	next, err := getNextToken()
	if err != nil {
		return errorOf(IncompleteVariableDeclaration)
	}
	if next.Kind != token.Identifier {
		return errorOf(InvalidDeclaration)
	}
	identifier := next.Value.(string)

	if isConstant {
		if err := currentSymbolTable.addAConstant(identifier, declaredType); err != nil {
			return err
		}
	} else {
		if err := currentSymbolTable.addAVariable(identifier, declaredType); err != nil {
			return err
		}
	}

	// <initializer> ::= '='<expression>
	next, err = getNextToken()
	if err != nil {
		return nil
	}
	if next.Kind != token.AssignmentSign {
		_ = putBackAToken()
		return nil
	}

	address := currentSymbolTable.getAddressOf(identifier)
	currentFunction.addAnInstructionWithOneOperand(vm.Ipush, address)
	if err := analyzeExpression(); err != nil {
		return err
	}
	currentFunction.addAnInstructionWithNoOperands(vm.Istore)
	return nil
}

func analyzeDeclaratorList(isConstant bool, declaredType int) *Error {
	// <init-declarator-list> ::= <init-declarator>{','<init-declarator>}
	if err := analyzeInitDeclarator(isConstant, declaredType); err != nil {
		return err
	}
	for {
		next, err := getNextToken()
		if !(err == nil && next.Kind == token.Comma) {
			if err == nil {
				_ = putBackAToken()
			}
			return nil
		}
		if err := analyzeInitDeclarator(isConstant, declaredType); err != nil {
			return err
		}
	}
}

func analyzeVariableDeclaration() *Error {
	// <variable-declaration> ::= [<const-qualifier>]<type-specifier><init-declarator-list>';'

	// [<const-qualifier>]
	next, err := getNextToken()
	if err != nil {
		return errorOf(IncompleteVariableDeclaration)
	}
	isAConstant := false
	if next.Kind == token.Const {
		isAConstant = true
		next, err = getNextToken()
		if err != nil {
			return errorOf(IncompleteVariableDeclaration)
		}
	}

	// <type-specifier>
	if next.Kind != token.Int {
		return errorOf(IncompleteVariableDeclaration)
	}
	kind := tokenKindToType(next)

	// <init-declarator-list>
	if err := analyzeDeclaratorList(isAConstant, kind); err != nil {
		return err
	}

	// ;
	if next, err = getNextToken(); err != nil || next.Kind != token.Semicolon {
		return errorOf(IncompleteVariableDeclaration)
	}
	return nil
}

func analyzeVariableDeclarations() *Error {
	for {
		next, err := getNextToken()
		_ = putBackAToken()
		if err != nil || (next.Kind != token.Int && next.Kind != token.Const) {
			return nil
		}
		if err := analyzeVariableDeclaration(); err != nil {
			return err
		}
	}
}

func analyzeReturnStatement() *Error {
	// <return-statement> ::= 'return' [<expression>] ';'
	if next, err := getNextToken(); err != nil || next.Kind != token.Return {
		if err == nil {
			_ = putBackAToken()
		}
		return errorOf(InvalidStatement)
	}
	_ = analyzeExpression()
	if next, err := getNextToken(); err != nil || next.Kind != token.Semicolon {
		if err == nil {
			_ = putBackAToken()
		}
		return errorOf(InvalidStatement)
	}
	return nil
}

func analyzeJumpStatement() *Error {
	// <jump-statement> ::= <return-statement>
	return analyzeReturnStatement()
}

func analyzePrintable() *Error {
	// <printable> ::= <expression>
	return analyzeExpression()
}

func analyzePrintableList() *Error {
	// <printable-list>  ::= <printable> {',' <printable>}
	if err := analyzePrintable(); err != nil {
		return err
	}
	for {
		if next, err := getNextToken(); err != nil || next.Kind != token.Comma {
			if err == nil {
				_ = putBackAToken()
			}
			return nil
		}
		if err := analyzePrintable(); err != nil {
			return err
		}
	}
}

func analyzeIOStatement() *Error {
	// <scan-statement>  ::= 'scan' '(' <identifier> ')' ';'
	// <print-statement> ::= 'print' '(' [<printable-list>] ')' ';'
	next, err := getNextToken()
	if err != nil {
		return errorOf(InvalidStatement)
	}
	if next.Kind == token.Scan {
		if next, err := getNextToken(); err != nil || next.Kind != token.LeftParenthesis {
			return errorOf(InvalidStatement)
		}
		next, err = getNextToken()
		if err != nil || next.Kind != token.Identifier {
			return errorOf(InvalidStatement)
		}
		identifier := next.Value.(string)
		if next, err := getNextToken(); err != nil || next.Kind != token.RightParenthesis {
			return errorOf(InvalidStatement)
		}
		currentFunction.addAnInstructionWithOneOperand(vm.Iload, currentSymbolTable.getAddressOf(identifier))
		currentFunction.addAnInstructionWithNoOperands(vm.Iscan)
		currentFunction.addAnInstructionWithNoOperands(vm.Istore)
	} else if next.Kind == token.Print {
		if next, err := getNextToken(); err != nil || next.Kind != token.LeftParenthesis {
			return errorOf(InvalidStatement)
		}
		if err := analyzePrintableList(); err != nil {
			return err
		}
		// TODO: generate expression
		if next, err := getNextToken(); err != nil || next.Kind != token.RightParenthesis {
			return errorOf(InvalidStatement)
		}
	} else {
		_ = putBackAToken()
		return errorOf(InvalidStatement)
	}
	if next, err := getNextToken(); err != nil || next.Kind != token.Semicolon {
		return errorOf(InvalidStatement)
	}
	return nil
}

func analyzeLoopStatement() *Error {
	// <loop-statement> ::= 'while' '(' <condition> ')' <statement>
	// TODO: generate instructions
	next, err := getNextToken()
	if err != nil || next.Kind != token.While {
		if err == nil {
			_ = putBackAToken()
		}
		return errorOf(InvalidStatement)
	}
	next, err = getNextToken()
	if err != nil || next.Kind != token.LeftParenthesis {
		return errorOf(InvalidStatement)
	}
	if err := analyzeCondition(); err != nil {
		return err
	}
	next, err = getNextToken()
	if err != nil || next.Kind != token.RightParenthesis {
		return errorOf(InvalidStatement)
	}
	return nil
}

func analyzeCondition() *Error {
	// <condition> ::= <expression>[<relational-operator><expression>]

	if err := analyzeExpression(); err != nil {
		return err
	}
	next, err := getNextToken()
	if err != nil {
		return nil
	}
	if !next.IsARelationalOperator() {
		_ = putBackAToken()
		return nil
	}
	if err := analyzeExpression(); err != nil {
		return err
	}
	currentFunction.addAnInstructionWithNoOperands(vm.Icmp)
	// TODO: jump according to comparison result
	return nil
}

func analyzeConditionStatement() *Error {
	// <condition-statement> ::=  'if' '(' <condition> ')' <statement> ['else' <statement>]
	// TODO: generate instructions

	if next, err := getNextToken(); err != nil || next.Kind != token.If {
		if err == nil {
			_ = putBackAToken()
		}
		return errorOf(InvalidStatement)
	}
	if next, err := getNextToken(); err != nil || next.Kind != token.LeftParenthesis {
		return errorOf(InvalidStatement)
	}
	if err := analyzeCondition(); err != nil {
		return err
	}
	if next, err := getNextToken(); err != nil || next.Kind != token.RightParenthesis {
		return errorOf(InvalidStatement)
	}
	if err := analyzeStatement(); err != nil {
		return err
	}
	if next, err := getNextToken(); err != nil || next.Kind != token.Else {
		// Only when a token is actually read is there the need to put back a token
		if err == nil {
			_ = putBackAToken()
		}
		return nil
	}
	if err := analyzeStatement(); err != nil {
		return err
	}

	return nil
}

func analyzeAssignmentExpression() *Error {
	// <assignment-expression> ::= <identifier><assignment-operator><expression>
	next, err := getNextToken()
	if err != nil {
		return errorOf(IncompleteExpression)
	}
	if next.Kind != token.Identifier {
		_ = putBackAToken()
		return errorOf(IncompleteExpression)
	}
	identifier := next.Value.(string)
	address := currentSymbolTable.getAddressOf(identifier)
	if next, err := getNextToken(); err != nil || next.Kind != token.AssignmentSign {
		if err == nil {
			_, _ = putBackAToken(), putBackAToken()
		}
		return errorOf(IncompleteExpression)
	}
	currentFunction.addAnInstructionWithOneOperand(vm.Ipush, address)
	if err := analyzeExpression(); err != nil {
		return err
	}
	currentFunction.addAnInstructionWithNoOperands(vm.Istore)
	return nil
}

func analyzeExpressionList() *Error {
	// <expression-list> ::= <expression>{','<expression>}
	if err := analyzeExpression(); err != nil {
		return err
	}
	for {
		if next, err := getNextToken(); err != nil || next.Kind != token.Comma {
			if err == nil {
				_ = putBackAToken()
			}
			return nil
		}
		if err := analyzeExpression(); err != nil {
			return err
		}
	}
}

func analyzeFunctionCall() *Error {
	// <identifier> '(' [<expression-list>] ')'
	next, err := getNextToken()
	if err != nil {
		return errorOf(IncompleteFunctionCall)
	}
	if next.Kind != token.Identifier {
		_ = putBackAToken()
		return errorOf(IncompleteFunctionCall)
	}
	identifier := next.Value.(string)
	sb := globalSymbolTable.getSymbol(identifier)
	if sb == nil {
		return errorOf(UndefinedIdentifier)
	}
	if next, err := getNextToken(); err != nil || next.Kind != token.LeftParenthesis {
		return errorOf(IncompleteFunctionCall)
	}
	_ = analyzeExpressionList()
	if next, err := getNextToken(); err != nil || next.Kind != token.RightParenthesis {
		return errorOf(IncompleteFunctionCall)
	}
	currentFunction.addAnInstructionWithOneOperand(vm.AnalyzerCall, sb.address)
	return nil
}

func analyzeStatement() *Error {
	// <statement> ::=
	//		'{' <statement-seq> '}'
	// 		|<condition-statement>
	// 		|<loop-statement>
	// 		|<jump-statement>
	// 		|<print-statement>
	// 		|<scan-statement>
	// 		|<assignment-expression>';'
	// 		|<function-call>';'
	// 		|';'

	// '{' <statement-seq> '}'
	next, err := getNextToken()
	if err == nil && next.Kind == token.LeftBracket {
		if err := analyzeStatementSeq(); err != nil {
			return err
		}
		if next, err := getNextToken(); err != nil || next.Kind != token.RightBracket {
			return errorOf(InvalidStatement)
		}
		return nil
	}
	_ = putBackAToken()

	// <condition-statement>
	if err := analyzeConditionStatement(); err == nil {
		return nil
	}

	// <loop-statement>
	if err := analyzeLoopStatement(); err == nil {
		return nil
	}

	// <jump-statement>
	if err := analyzeJumpStatement(); err == nil {
		return nil
	}

	// <print-statement> | <scan-statement>
	if err := analyzeIOStatement(); err == nil {
		return nil
	}

	// <assignment-expression>';'
	if err := analyzeAssignmentExpression(); err == nil {
		if next, err := getNextToken(); err != nil || next.Kind != token.Semicolon {
			return errorOf(InvalidStatement)
		}
		return nil
	}

	// <function-call>';'
	if err := analyzeFunctionCall(); err == nil {
		if next, err := getNextToken(); err != nil || next.Kind != token.Semicolon {
			return errorOf(InvalidStatement)
		}
		return nil
	}

	// ';'
	if next, err := getNextToken(); err != nil || next.Kind != token.Semicolon {
		if err == nil {
			_ = putBackAToken()
		}
		return errorOf(InvalidStatement)
	}

	return nil
}

func analyzeStatementSeq() *Error {
	// {<statement>}
	for {
		if err := analyzeStatement(); err != nil {
			return nil
		}
	}
}

func analyzeCompoundStatement() *Error {
	// '{' {<variable-declaration>} <statement-seq> '}'
	if next, err := getNextToken(); err != nil || next.Kind != token.LeftBracket {
		if err == nil {
			_ = putBackAToken()
		}
		return errorOf(InvalidStatement)
	}
	if err := analyzeVariableDeclarations(); err != nil {
		return err
	}
	if err := analyzeStatementSeq(); err != nil {
		return err
	}
	if next, err := getNextToken(); err != nil || next.Kind != token.RightBracket {
		return errorOf(InvalidStatement)
	}
	return nil
}

func analyzeParameterDeclaration() *Error {
	// [<const-qualifier>]<type-specifier><identifier>
	next, err := getNextToken()
	if err != nil {
		return errorOf(InvalidDeclaration)
	}
	isConst := false
	if next.Kind == token.Const {
		isConst = true
		next, err = getNextToken()
		if err != nil {
			return errorOf(InvalidDeclaration)
		}
	}
	if !next.IsATypeSpecifier() {
		return errorOf(InvalidDeclaration)
	}
	kind := tokenKindToType(next)
	next, err = getNextToken()
	if err != nil || next.Kind != token.Identifier {
		return errorOf(InvalidDeclaration)
	}
	identifier := next.Value.(string)

	if isConst {
		_ = currentSymbolTable.addAVariable(identifier, kind)
	} else {
		_ = currentSymbolTable.addAConstant(identifier, kind)
	}
	*currentFunction.parameters = append(*currentFunction.parameters, identifier)
	return nil
}

func analyzeParameterDeclarationList() *Error {
	// <parameter-declaration-list> ::= <parameter-declaration>{','<parameter-declaration>}

	// <parameter-declaration>
	if err := analyzeParameterDeclaration(); err != nil {
		return err
	}

	// {','<parameter-declaration>}
	for {
		next, err := getNextToken()
		if err != nil {
			return nil
		}
		if next.Kind != token.Comma {
			_ = putBackAToken()
			return nil
		}
		if err := analyzeParameterDeclaration(); err != nil {
			return err
		}
	}
}

func analyzeParameterClause() *Error {
	// <parameter-clause> ::= '(' [<parameter-declaration-list>] ')'
	next, err := getNextToken()
	if err != nil || next.Kind != token.LeftParenthesis {
		if err == nil {
			_ = putBackAToken()
		}
		return errorOf(InvalidDeclaration)
	}

	next, err = getNextToken()
	if err != nil {
		return errorOf(InvalidStatement)
	}
	if next.Kind == token.RightParenthesis {
		return nil
	} else {
		_ = putBackAToken()
	}

	if err := analyzeParameterDeclarationList(); err != nil {
		return err
	}
	next, err = getNextToken()
	if err != nil || next.Kind != token.RightParenthesis {
		return errorOf(IncompleteExpression)
	}
	return nil
}

func analyzeFunctionDefinition() *Error {
	// <function-definition> ::= <type-specifier><identifier><parameter-clause><compound-statement>

	currentFunction = initFunctionInfo()
	currentSymbolTable = createChildSymbolTableFor(currentSymbolTable, currentFunction)

	next, err := getNextToken()
	if err != nil || !next.IsATypeSpecifier() {
		if err == nil {
			_ = putBackAToken()
		}
		return errorOf(InvalidDeclaration)
	}
	kind := next.Kind
	next, err = getNextToken()
	if err != nil || next.Kind != token.Identifier {
		_ = putBackAToken()
		return errorOf(InvalidDeclaration)
	}
	identifier := next.Value.(string)
	if err := addAFunction(identifier, kind); err != nil {
		return err
	}
	if err := analyzeParameterClause(); err != nil {
		return err
	}
	if err := analyzeCompoundStatement(); err != nil {
		return err
	}

	if funSymbol := globalSymbolTable.getSymbol(identifier); funSymbol != nil {
		// Have to do the assignment this way thanks to all the trivia of golang
		funSymbol.appendix = currentFunction
		globalSymbolTable.symbols[identifier] = *funSymbol
	}

	currentFunction = globalStart
	currentSymbolTable = globalSymbolTable
	return nil
}

func analyzeFunctionDefinitions() *Error {
	for {
		next, err := getNextToken()
		_ = putBackAToken()
		if err != nil || !next.IsATypeSpecifier() {
			return nil
		}
		if err := analyzeFunctionDefinition(); err != nil {
			return err
		}
	}
}

func Run(parser *Parser, writer *bufio.Writer, shouldCompileToBinary bool) {
	globalParser, globalWriter = parser, writer
	globalStart = initFunctionInfo()
	globalSymbolTable = initSymbolTable(nil, globalStart)
	currentSymbolTable = globalSymbolTable
	currentFunction = globalStart

	if err := analyzeVariableDeclarations(); err != nil {
		reportFatalError(err)
	}
	if err := analyzeFunctionDefinitions(); err != nil {
		reportFatalError(err)
	}

	symbolOfFn := globalSymbolTable.symbols["fn"]
	appendix := symbolOfFn.appendix.(*functionInfo)
	for _, el := range *appendix.instructions {
		fmt.Println(el)
	}
}
