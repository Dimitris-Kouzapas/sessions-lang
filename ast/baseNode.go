package ast

import (
    "fmt"
    "sessions/ast/util"
)

type baseNode struct {
    filename string
    line     int
}

func (b *baseNode) init(line int) {
    b.line = line
    b.filename = ""
}

func (b *baseNode) file() string { return b.filename }
func (b *baseNode) lineno() int  { return b.line }
func (b *baseNode) setFilename(filename string) {
    b.filename = filename
}

func (b *baseNode) reportError(err string, log util.ErrorLog) {
    log.Add(util.NewTypeError(err, b.filename, b.line))
}

func (b *baseNode) reportErrorf(log util.ErrorLog, format string, args ...interface{}) {
    msg := fmt.Sprintf(format, args...)
    log.Add(util.NewTypeError(msg, b.filename, b.line))
}

func (b *baseNode) reportf(log util.ReportLog, format string, args ...interface{}) {
    msg := fmt.Sprintf(format, args...)
    log.Report(util.NewSessionAnalysisReport(msg, b.filename, b.line))
}

func (b *baseNode) runtimeErrorf(format string, args ...interface{}) error {
    msg := fmt.Sprintf(format, args...)
    return util.NewRuntimeError(msg, b.filename, b.line)
}