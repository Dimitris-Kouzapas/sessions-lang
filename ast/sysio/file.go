package sysio

import (
	"fmt"
	"os"
	"io"
	"bufio"
	"sync"
	"errors"
)


/**********************************************************************************
 * file
 **********************************************************************************/

type File struct {
	fp 		*os.File
	bufr 	*bufio.Reader
	bufw 	*bufio.Writer
	rd    	bool
	wr    	bool

	mu    	sync.Mutex
}

func NewFile(fp *os.File, rd, wr bool) *File {
	return &File{
		fp:  fp,
		rd: rd,
		wr: wr,
		// bufr/bufw are lazy
	}
}


func (f *File) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if !f.wr {
		return 0, errors.New("file not writable")
	}
	return f.fp.Write(p)
}

func (f *File) WriteString(s string) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if !f.wr {
		return 0, errors.New("file not writable")
	}
	if f.bufw == nil {
		f.bufw = bufio.NewWriter(f.fp)
	}
	return f.bufw.WriteString(s)
}

func (f *File) Writef(format string, args ...interface{}) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.wr {
		return 0, errors.New("file not writable")
	}

	return fmt.Fprintf(f.fp, format, args...)
}


func (f *File) Read(p []byte) (int, error) {
	if !f.rd {
		return 0, errors.New("file not readable")
	}
	return f.fp.Read(p)
}

func (f *File) ReadString(delim byte) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	if !f.rd {
		return "", errors.New("file not readable")
	}

	if f.bufr == nil {
		f.bufr = bufio.NewReader(f.fp)
	}
	return f.bufr.ReadString(delim)
}

// ReadLine reads up to '\n'. If EOF arrives after some bytes, returns them.
func (f *File) ReadLine() (string, error) {
    s, err := f.ReadString('\n')
    if err == io.EOF && len(s) > 0 {
        return s, nil
    }
    return s, err
}

func (f *File) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	var err error
	if f.bufw != nil {
		if e := f.bufw.Flush(); e != nil && err == nil {
			err = e
		}
	}
	if f.fp != nil {
		if e := f.fp.Close(); e != nil && err == nil {
			err = e
		}
		f.fp = nil
	}
	return err
}


