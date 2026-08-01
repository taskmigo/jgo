package gots

// This validator keeps class-context early errors explicit while the full class
// grammar is owned by Phase 3. It recognizes only method/static-block boundaries
// needed to avoid treating their bodies as unrelated sloppy statements.
func validateRecognizableClassEarlyErrors(tokens []Token) error {
	for index := 0; index < len(tokens); index++ {
		if tokens[index].Type != TokClass {
			continue
		}
		bodyStart := nextTokenOfType(tokens, index+1, TokLBrace)
		if bodyStart < 0 {
			continue
		}
		bodyEnd := matchingDelimiter(tokens, bodyStart, TokLBrace, TokRBrace)
		if bodyEnd < 0 {
			continue
		}
		if err := validateRecognizableClassBody(tokens, bodyStart+1, bodyEnd); err != nil {
			return err
		}
		index = bodyEnd
	}
	return nil
}

func validateRecognizableClassBody(tokens []Token, start, end int) error {
	memberStart := start
	for index := start; index < end; index++ {
		if tokens[index].Type != TokLBrace {
			continue
		}
		bodyEnd := matchingDelimiter(tokens, index, TokLBrace, TokRBrace)
		if bodyEnd < 0 || bodyEnd > end {
			return nil
		}
		header := tokens[memberStart:index]
		isStaticBlock := len(header) == 1 && header[0].Type == TokIdent && header[0].Literal == "static"
		isAsyncMethod := tokenSliceContainsLiteral(header, "async")
		isGeneratorMethod := tokenSliceContainsType(header, TokStar)
		if isStaticBlock {
			if err := validateRecognizableStaticBlock(tokens, index+1, bodyEnd); err != nil {
				return err
			}
		} else if isAsyncMethod || isGeneratorMethod {
			for bodyIndex := index + 1; bodyIndex < bodyEnd; bodyIndex++ {
				if bodyIndex+1 >= bodyEnd || tokens[bodyIndex+1].Type != TokColon {
					continue
				}
				if isAsyncMethod && tokens[bodyIndex].Type == TokIdent && tokens[bodyIndex].Literal == "await" {
					return classContextSyntaxError(tokens[bodyIndex], "await cannot be a label in an async method")
				}
				if isGeneratorMethod && tokens[bodyIndex].Type == TokIdent && tokens[bodyIndex].Literal == "yield" {
					return classContextSyntaxError(tokens[bodyIndex], "yield cannot be a label in a generator method")
				}
			}
		}
		index = bodyEnd
		memberStart = bodyEnd + 1
		if memberStart < end && tokens[memberStart].Type == TokSemi {
			memberStart++
		}
	}
	return nil
}

func validateRecognizableStaticBlock(tokens []Token, start, end int) error {
	localIterationDepth := 0
	for index := start; index < end; index++ {
		token := tokens[index]
		if token.Type == TokIdent && token.Literal == "await" {
			return classContextSyntaxError(token, "await is not allowed in a class static block")
		}
		if token.Type == TokClass && index+1 < end && tokens[index+1].Type == TokIdent && tokens[index+1].Literal == "await" {
			return classContextSyntaxError(tokens[index+1], "await is not allowed as a binding identifier in a class static block")
		}
		if token.Type == TokWhile || token.Type == TokFor {
			localIterationDepth++
		}
		if token.Type == TokIdent && token.Literal == "await" && index+1 < end && tokens[index+1].Type == TokColon {
			return classContextSyntaxError(token, "await cannot be a label in a class static block")
		}
		if (token.Type == TokBreak || token.Type == TokContinue) && localIterationDepth == 0 {
			return classContextSyntaxError(token, "control flow cannot cross a class static block boundary")
		}
	}
	return nil
}

func nextTokenOfType(tokens []Token, start int, tokenType TokenType) int {
	for index := start; index < len(tokens); index++ {
		if tokens[index].Type == tokenType {
			return index
		}
		if tokens[index].Type == TokSemi || tokens[index].Type == TokEOF {
			return -1
		}
	}
	return -1
}

func matchingDelimiter(tokens []Token, start int, opening, closing TokenType) int {
	depth := 0
	for index := start; index < len(tokens); index++ {
		switch tokens[index].Type {
		case opening:
			depth++
		case closing:
			depth--
			if depth == 0 {
				return index
			}
		}
	}
	return -1
}

func tokenSliceContainsLiteral(tokens []Token, literal string) bool {
	for _, token := range tokens {
		if token.Type == TokIdent && token.Literal == literal {
			return true
		}
	}
	return false
}

func tokenSliceContainsType(tokens []Token, tokenType TokenType) bool {
	for _, token := range tokens {
		if token.Type == tokenType {
			return true
		}
	}
	return false
}

func classContextSyntaxError(token Token, message string) error {
	return &SyntaxError{Span: token.Span, Token: token, Message: message}
}
