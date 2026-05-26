package ast

import (
    antlr "github.com/antlr4-go/antlr/v4"
    "sessions/ast/parser"
    "sessions/ast/util"
)

/******************************************************************************
 * Custom syntax error Listener
 ******************************************************************************/

type customErrorListener struct {
    *antlr.DefaultErrorListener
    util.BasicErrorLog
}

func newCustomErrorListener() *customErrorListener {
    return &customErrorListener{
        DefaultErrorListener: &antlr.DefaultErrorListener{},
        BasicErrorLog:        util.BasicErrorLog{},
    }
}

func (ce *customErrorListener) SyntaxError(_ antlr.Recognizer, _ interface{}, line, column int, msg string, _ antlr.RecognitionException) {
    customError := util.NewSyntaxError(msg, line, column)
    ce.Add(customError)
}

/******************************************************************************
 * Parser
 ******************************************************************************/

func parse(file string, log *customErrorListener) *module {
    input, err := antlr.NewFileStream(file)

    if err != nil {
        log.Add(util.NewSystemError(err.Error()))
        return nil
    }

    lexer := parser.NewsessionsLexer(input)
    lexer.RemoveErrorListeners()
    lexer.AddErrorListener(log)

    stream := antlr.NewCommonTokenStream(lexer, 0)

    p := parser.NewsessionsParser(stream)
    p.RemoveErrorListeners()
    p.AddErrorListener(log)
    tree := p.Module()

    if log.HasErrors() {
        return nil
    }
    return new(sessionsVisitor).VisitModule(tree.(*parser.ModuleContext))
}
