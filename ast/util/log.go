package util

import (
    "fmt"
    "os"
)

type ErrorLog interface {
    Add(e error) ErrorLog
    HasErrors() bool
    Iterator() func() (error, bool)
    PrintErrors()
    Errors() error
}

/*******************************************************************************
 *  ErrorLog implementation
 ******************************************************************************/

type BasicErrorLog struct {
    log []error
}

func NewErrorLog() *BasicErrorLog {
    return &BasicErrorLog{}
}

func (b *BasicErrorLog) Add(e error) ErrorLog {
    b.log = append(b.log, e)
    return b
}

func (b *BasicErrorLog) HasErrors() bool    { return len(b.log) != 0 }

func (b *BasicErrorLog) Iterator() func() (error, bool) {
    size := len(b.log)
    index := 0
    return  func() (error, bool) {
                if index < size {
                    elem := b.log[index]
                    index++
                    return elem, true
                }
                return nil, false
            }
}

func (b *BasicErrorLog) PrintErrors() {
    for _, e := range b.log {
        fmt.Fprintln(os.Stderr, e.Error())
    }
}

func (b *BasicErrorLog) Errors() error {
    return JoinSep("\n", b.log...)
}

/*******************************************************************************
 *  ReportLog
 ******************************************************************************/

type ReportLog interface {
    Report(r fmt.Stringer) ReportLog
    HasReports() bool
    Iterator() func() (fmt.Stringer, bool)
    PrintReports()
}

/*******************************************************************************
 *  ReportLog implementation
 *******************************************************************************/

type BasicReportLog struct {
    log []fmt.Stringer
}

func NewReportLog() *BasicReportLog {
    return &BasicReportLog{}
}

func (b *BasicReportLog) Report(s fmt.Stringer) ReportLog {
    b.log = append(b.log, s)
    return b
}

func (b *BasicReportLog) HasReports() bool    { return len(b.log) != 0 }

func (b *BasicReportLog) Iterator() func() (fmt.Stringer, bool) {
    size := len(b.log)
    index := 0
    return  func() (fmt.Stringer, bool) {
                if index < size {
                    elem := b.log[index]
                    index++
                    return elem, true
                }
                return nil, false
            }
}

func (b *BasicReportLog) PrintReports() {
    for _, s := range b.log {
        fmt.Fprintln(os.Stdout, s.String())
    }
}

/*******************************************************************************
 *  report implementation
 *******************************************************************************/

type basicReport struct {
    msg         string
    reportType  string
    file        string
    line        int
}

func NewSessionAnalysisReport(msg string, file string, line int) *basicReport {
    return &basicReport {
        msg: msg,
        reportType: "Session Analysis",
        file: file,
        line: line,
    }
}

func (r *basicReport) String() string {
    if r.file == "" {
        return fmt.Sprintf("%d: %s report.\n\t%s", r.line, r.reportType, r.msg)
    }
    return fmt.Sprintf("%s:%d: %s report.\n\t%s", r.file, r.line, r.reportType, r.msg)
}

/*******************************************************************************
 * Implementations of the error interface
 ******************************************************************************/

/*******************************************************************************
 * basic Error
 ******************************************************************************/

type basicError struct {
    err         string
    errorType   string
    file        string
    line        int
}

func (b *basicError) Error() string {
    if b.file == "" {
        return fmt.Sprintf("%d: %s error.\n\t%s", b.line, b.errorType, b.err)
    }
    return fmt.Sprintf("%s:%d: %s error.\n\t%s", b.file, b.line, b.errorType, b.err)
}

/*******************************************************************************
 * Custom Syntax Error
 ******************************************************************************/

type syntaxError struct {
    basicError
    column  int
    file    string
}

func NewSyntaxError(err string, file string, line int, column int) *syntaxError {
    return &syntaxError {
        basicError: basicError{
            err: err,
            errorType: "Syntax",
            file: file,
            line: line,
        },
        column: column,
    }
}

/*******************************************************************************
 * type check Error
 ******************************************************************************/

type typeError struct {
    basicError
}

func NewTypeError(err string, file string, line int) *typeError {
    return &typeError {
        basicError: basicError{
            err: err,
            errorType: "Typecheck",
            file: file,
            line: line,
        },
    }
}

/*******************************************************************************
 * system Error
 ******************************************************************************/

type systemError struct {
    basicError
}

func NewSystemError(err string) *systemError {
    return &systemError {
        basicError: basicError{
            err: err,
            errorType: "System",
            file: "",
            line: 0,
        },
    }
}

func (s *systemError) Error() string {
    return fmt.Sprintf("System error.\n\t%v", s.err)
}


/*******************************************************************************
 * runtime Error
 ******************************************************************************/

type runtimeError struct {
    basicError
}

func NewRuntimeError(err string, file string, line int) *runtimeError {
    return &runtimeError {
        basicError: basicError {
            err: err,
            errorType: "Runtime",
            file: file,
            line: line,
        },
    }
}
