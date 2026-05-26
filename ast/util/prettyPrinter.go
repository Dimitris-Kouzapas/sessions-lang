package util

import (
    "fmt"
    "io"
    "strings"
)

type IndentedWriter interface {
    io.Writer
    io.StringWriter
    Print(args ...any)
    Println(args ...any)
    Printf(format string, args ...any)
    Set(string) *Stream
    Inc() *Stream
    Dec() *Stream
    fmt.Stringer
}

type Stream struct {
    indent int
    indentSymbol string
    builder strings.Builder
    newlineFlag bool
}

func NewStream() *Stream {
    return &Stream {
        indent: 0,
        indentSymbol: "\t",
        newlineFlag: true,
    }
}

func (s *Stream) Set(symbol string) *Stream {
    s.indentSymbol = symbol
    return s
}

func (s *Stream) Inc() *Stream {
    s.indent++
    return s
}

func (s *Stream) Dec() *Stream {
    if s.indent > 0 {
        s.indent--
    }
    return s
}

func (s *Stream) Reset() {
    s.indent = 0
    s.builder.Reset()
    s.newlineFlag = true
}

func (s *Stream) indentation() string {
    return strings.Repeat(s.indentSymbol, s.indent)
}

// implement io.Writer: handle multi-line input; indent at each new line start
func (s *Stream) Write(p []byte) (n int, err error) {
    for i := 0; i < len(p); i++ {
        if s.newlineFlag {
            s.builder.WriteString(s.indentation())
            s.newlineFlag = false
        }
        c := p[i]
        s.builder.WriteByte(c)
        if c == '\n' {
            s.newlineFlag = true
        }
    }
    return len(p), nil
}

// Efficient WriteString that preserves indentation and avoids []byte allocs.
func (s *Stream) WriteString(str string) (int, error) {
    written := 0
    for len(str) > 0 {
        // Emit indentation at the start of a new line
        if s.newlineFlag {
            s.builder.WriteString(s.indentation())
            s.newlineFlag = false
        }

        // Write up to and including the next newline (if any)
        if i := strings.IndexByte(str, '\n'); i >= 0 {
            s.builder.WriteString(str[:i+1])
            written += i + 1
            s.newlineFlag = true
            str = str[i+1:]
            continue
        }

        // No newline left; write the remainder
        s.builder.WriteString(str)
        written += len(str)
        break
    }
    return written, nil
}

func (s *Stream) Print(args ...any) {
    _, _ = s.WriteString(fmt.Sprint(args...))
}

func (s *Stream) Println(args ...any) {
    _, _ = s.WriteString(fmt.Sprintln(args...))
}

func (s *Stream) Printf(format string, args ...any) {
    _, _ = s.WriteString(fmt.Sprintf(format, args...))
}

func (s *Stream) String() string {
	 return s.builder.String()
}
