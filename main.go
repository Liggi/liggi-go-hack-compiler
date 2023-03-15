package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"regexp"
)

// TODO:
// [x] - Rework tokeniser to have a "root" token
// [x] - Handle `subroutineDec`
// [x] - Handle `parameterList`
// [x] - Handle `subroutineBody`
// [x] - Refactor some of the "special cases" stuff, there's definitely a better way (I've commented below)
// [x] - Handle `return` statements
// [] - Handle `do` statements
// [] - Reconsider shallow functions like `handleKeyword` and `handleSymbol`

type Token struct {
	tokenType string
	token     string
	children  []*Token
}

type TokenContextStack struct {
	items []*Token
}

type Tokeniser struct {
	tokenBeingParsed  string
	currentTokenStack TokenContextStack
	tokens            []*Token
}

var keywords = []string{
	"class", "function", "void", "return", "do",
}

var symbols = []string{
	"{", "}", "(", ")", "[", "]", ".", ",", ";", "+", "-", "*", "/", "&", "|", "<", ">", "=", "~",
}

var whitespace = []string{
	" ", "\n", "\t",
}

var initialTokens = map[string]Token{
	"class": {
		tokenType: "class",
		token:     "",
		children: []*Token{
			{
				tokenType: "keyword",
				token:     "class",
			},
		},
	},
	"function": {
		tokenType: "subroutineDec",
		token:     "",
		children: []*Token{
			{
				tokenType: "keyword",
				token:     "function",
			},
		},
	},
	"parameterList": {
		tokenType: "parameterList",
		token:     "",
		children:  []*Token{},
	},
	"expressionList": {
		tokenType: "expressionList",
		token:     "",
		children:  []*Token{},
	},
	"subroutineBody": {
		tokenType: "subroutineBody",
		token:     "",
		children:  []*Token{},
	},
	"statements": {
		tokenType: "statements",
		token:     "",
		children:  []*Token{},
	},
	"return": {
		tokenType: "returnStatement",
		token:     "",
		children: []*Token{
			{
				tokenType: "keyword",
				token:     "return",
			},
		},
	},
	"do": {
		tokenType: "doStatement",
		token:     "",
		children: []*Token{
			{
				tokenType: "keyword",
				token:     "do",
			},
		},
	},
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("No file specified")
	}

	fileName := os.Args[1]

	file, err := os.Open(fileName)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanRunes)

	tokeniser := NewTokeniser()
	tokeniser.Tokenise(scanner)
	if err != nil {
		log.Fatal(err)
	}
}

func NewTokeniser() *Tokeniser {
	rootToken := &Token{
		tokenType: "root",
		children:  []*Token{},
	}

	// NOTE: Can probably what this is for
	initialTokenStack := TokenContextStack{
		items: []*Token{
			rootToken,
		},
	}

	return &Tokeniser{
		currentTokenStack: initialTokenStack,
		tokens: []*Token{
			rootToken,
		},
	}
}

func printTokenTree(token *Token, indent string, isLast bool) {
	fmt.Print(indent)

	if isLast {
		fmt.Print("└─")
		indent += "  "
	} else {
		fmt.Print("├─")
		indent += "│ "
	}

	fmt.Println(token.tokenType, token.token)

	numChildren := len(token.children)
	for i, child := range token.children {
		printTokenTree(child, indent, i == numChildren-1)
	}
}

func (t *Tokeniser) Tokenise(scanner *bufio.Scanner) string {
	for scanner.Scan() {
		char := scanner.Text()
		isTokenTerminator := (isSymbol(char) || isWhitespace(char)) && t.tokenBeingParsed != ""

		if isTokenTerminator {
			t.ProcessToken(t.tokenBeingParsed)
			t.tokenBeingParsed = ""
		}

		if isWhitespace(char) {
			continue
		}

		if isSymbol(char) {
			t.ProcessToken(char)
			t.tokenBeingParsed = ""
			continue
		}

		t.tokenBeingParsed += char
	}

	printTokenTree(t.tokens[0], " ", false)

	return ""
}

func (t *Tokeniser) shouldEndCurrentTokenGroup(token string) bool {
	tokenGroupEndingTokens := []string{
		"}", ";", ")", "]",
	}

	// Check if the token is in the `tokenGroupEndingTokens`
	for _, tokenGroupEndingToken := range tokenGroupEndingTokens {
		if token == tokenGroupEndingToken {
			return true
		}
	}

	return false
}

func (t *Tokeniser) shouldOpenNewTokenGroup(token string) bool {
	tokenGroupOpeningTokens := []string{
		"{", "(", "[",
	}

	// Check if the token is in the `tokenGroupEndingTokens`
	for _, tokenGroupEndingToken := range tokenGroupOpeningTokens {
		if token == tokenGroupEndingToken {
			return true
		}
	}

	return false
}

func (t *Tokeniser) CheckCurrentContext(contextType ...string) bool {
	var matchesContextTypes bool

	for _, contextType := range contextType {
		if t.currentTokenStack.Peek().tokenType == contextType {
			matchesContextTypes = true
		}
	}

	return matchesContextTypes
}

func (t *Tokeniser) PopContextStack(count ...int) {
	if len(count) == 0 {
		t.currentTokenStack.Pop()
		return
	}

	t.currentTokenStack.Pop(count[0])
}

func (t *Tokeniser) endCurrentTokenGroup(token string) {
	if token == "}" {
		// Encountering a "}" while in a "statements" block means you should close the "statements"
		// block, add the symbol, then close the "subroutineBody" and the "subroutineDec"
		if t.CheckCurrentContext("statements") {
			t.PopContextStack()
			t.handleSymbol(token)
			t.PopContextStack(2)
			return
		}

		// The normative case here is that encountering a "}" closes the current block
		t.handleSymbol(token)
		t.PopContextStack()
		return
	}

	if token == ";" {
		// Encountering a ";" signifies the end of a statement
		t.handleSymbol(token)
		t.PopContextStack()
		return
	}

	if token == ")" {
		if t.CheckCurrentContext("expression") {
			t.PopContextStack()
			t.handleSymbol(token)

			// Are we still in a `term`? If so, the `term` was the `expression` we just closed, so we can close the `term` too
			// Alternatively, if we've hit the end of our list of `expression`s, then it's time to also close the `expressionList`
			if t.CheckCurrentContext("term", "expressionList") {
				t.PopContextStack()
			}

			return
		}

		// The normative case here is that encountering a ")" closes the current block
		t.PopContextStack()
		t.handleSymbol(token)

		return
	}

	if token == "]" {
		//
	}
}

func GetInitialToken(tokenType string) *Token {
	initial := initialTokens[tokenType]
	return &initial
}

func (t *Tokeniser) openNewTokenGroup(token string) {
	// Same as above, just needs tidying up so it's not such a mess

	currentTokenContext := t.currentTokenStack.Peek()

	if token == "{" && currentTokenContext.tokenType == "subroutineDec" {
		subroutineBodyToken := GetInitialToken("subroutineBody")
		t.AddToken(subroutineBodyToken)
		t.UpdateCurrentContext(subroutineBodyToken)

		t.AddToken(&Token{
			tokenType: "symbol",
			token:     token,
		})

		statementsToken := GetInitialToken("statements")
		t.AddToken(statementsToken)
		t.UpdateCurrentContext(statementsToken)
		return
	}

	if token == "(" && currentTokenContext.tokenType == "subroutineDec" {
		t.AddToken(&Token{
			tokenType: "symbol",
			token:     token,
		})

		parameterListToken := GetInitialToken("parameterList")
		t.AddToken(parameterListToken)
		t.UpdateCurrentContext(parameterListToken)
		return
	}

	if token == "(" && currentTokenContext.tokenType == "doStatement" {
		t.AddToken(&Token{
			tokenType: "symbol",
			token:     token,
		})

		expressionListToken := GetInitialToken("expressionList")
		t.AddToken(expressionListToken)
		t.UpdateCurrentContext(expressionListToken)
		return
	}

	if token == "(" {
		if t.CheckCurrentContext("expressionList", "expression", "term") {
			var expressionToken *Token

			if !t.CheckCurrentContext("expression") {
				expressionToken = &Token{
					tokenType: "expression",
					token:     "",
				}

				t.AddToken(expressionToken)
				t.UpdateCurrentContext(expressionToken)
			}

			termToken := &Token{
				tokenType: "term",
				token:     "",
			}

			t.AddToken(termToken)
			t.UpdateCurrentContext(termToken)

			t.AddToken(&Token{
				tokenType: "symbol",
				token:     token,
			})

			expressionToken = &Token{
				tokenType: "expression",
				token:     "",
			}

			t.AddToken(expressionToken)
			t.UpdateCurrentContext(expressionToken)

			return
		}
	}

	t.AddToken(&Token{
		tokenType: "symbol",
		token:     token,
	})
	return
}

func (t *Tokeniser) ProcessToken(token string) {
	tokenType := t.GetTokenType(token)

	if t.shouldEndCurrentTokenGroup(token) {
		t.endCurrentTokenGroup(token)
		return
	}

	if t.shouldOpenNewTokenGroup(token) {
		t.openNewTokenGroup(token)
		return
	}

	if tokenType == "symbol" {
		t.handleSymbol(token)
		return
	}

	if tokenType == "keyword" {
		// Could probably extract all this logic into `handleKeyword`
		if token == "class" {
			t.handleClass(token)
			return
		}

		if token == "function" {
			t.handleFunction(token)
			return
		}

		if token == "return" {
			t.handleReturnStatement(token)
			return
		}

		if token == "do" {
			t.handleDoStatement(token)
			return
		}

		t.handleKeyword(token)
		return
	}

	if tokenType == "integerConstant" || tokenType == "identifier" {
		// If we're in an "expressionList", this must be our first single term `expression`
		if t.CheckCurrentContext("expressionList") {
			expressionToken := &Token{
				tokenType: "expression",
				token:     "",
			}

			t.AddToken(expressionToken)
			t.UpdateCurrentContext(expressionToken)
		}

		// If we're in an "expression", and we encounter an "integerConstant" or an "identifier", it means
		// we have start defining terms, so we open an initial `term`
		if t.CheckCurrentContext("expression") {
			termToken := &Token{
				tokenType: "term",
				token:     "",
			}

			t.AddToken(termToken)
			t.UpdateCurrentContext(termToken)
		}

		if tokenType == "integerConstant" {
			t.AddToken(&Token{
				tokenType: "integerConstant",
				token:     token,
			})
		}

		if tokenType == "identifier" {
			t.AddToken(&Token{
				tokenType: "identifier",
				token:     token,
			})
		}

		// If a term contains an `integerConstant` or an `identifier`, that's all it contains
		if t.CheckCurrentContext("term") {
			t.PopContextStack()
		}

		return
	}

	return
}

// These four functions are all doing the same thing
func (t *Tokeniser) handleReturnStatement(token string) {
	returnToken := GetInitialToken("return")

	t.AddToken(returnToken)
	t.UpdateCurrentContext(returnToken)
}

func (t *Tokeniser) handleDoStatement(token string) {
	returnToken := GetInitialToken("do")

	t.AddToken(returnToken)
	t.UpdateCurrentContext(returnToken)
}

func (t *Tokeniser) handleClass(token string) {
	classToken := GetInitialToken("class")

	t.AddToken(classToken)
	t.UpdateCurrentContext(classToken)
}

func (t *Tokeniser) handleFunction(token string) {
	functionToken := GetInitialToken("function")

	t.AddToken(functionToken)
	t.UpdateCurrentContext(functionToken)
}

func (t *Tokeniser) handleKeyword(token string) {
	t.AddToken(&Token{
		tokenType: "keyword",
		token:     token,
	})
}

func (t *Tokeniser) handleSymbol(token string) {
	// This logic feels weird here
	if (token == "+" || token == "*" || token == "/") && t.currentTokenStack.Peek().tokenType == "term" {
		t.currentTokenStack.Pop()
	}

	t.AddToken(&Token{
		tokenType: "symbol",
		token:     token,
	})
}

func (t *Tokeniser) AddToken(token *Token) {
	currentTokenContext := t.currentTokenStack.Peek()
	currentTokenContext.children = append(currentTokenContext.children, token)
}

func (t *Tokeniser) UpdateCurrentContext(token *Token) {
	t.currentTokenStack.Push(token)
}

func (t *Tokeniser) GetTokenType(token string) string {
	for _, keyword := range keywords {
		if token == keyword {
			return "keyword"
		}
	}

	if isSymbol(token) {
		return "symbol"
	}

	if matched, err := regexp.MatchString(`^[a-zA-Z_]\w*$`, token); err == nil && matched {
		return "identifier"
	}

	if matched, err := regexp.MatchString(`^\d*$`, token); err == nil && matched {
		return "integerConstant"
	}

	return ""
}

func isSymbol(r string) bool {
	if r == "" {
		return false
	}

	for _, symbol := range symbols {
		if r == symbol {
			return true
		}
	}

	return false
}

func isWhitespace(r string) bool {
	for _, whitespace := range whitespace {
		if r == whitespace {
			return true
		}
	}

	return false
}

func (tcs *TokenContextStack) Push(token *Token) {
	tcs.items = append(tcs.items, token)
}

func (tcs *TokenContextStack) Pop(count ...int) {
	var c int

	if len(count) == 0 {
		c = 1
	} else {
		c = count[0]
	}

	tcs.items = tcs.items[:len(tcs.items)-c]
}

func (tcs *TokenContextStack) Peek() *Token {
	return tcs.items[len(tcs.items)-1]
}
