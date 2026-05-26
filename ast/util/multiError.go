package util

import "strings"

type multiError struct {
	errs []error
	sep string
}

func newMultiError(sep string) *multiError {
	return &multiError{sep: sep}
}

func (m *multiError) Add(err error) {
	if err != nil {
		m.errs = append(m.errs, err)
	}
}

func (m *multiError) Unwrap() []error {
	return m.errs
}

func (m *multiError) Error() string {
	if len(m.errs) == 0 {
		return ""
	}
	msgs := make([]string, 0, len(m.errs))
	for _, e := range m.errs {
		if e != nil {
			msgs = append(msgs, e.Error())
		}
	}
	return strings.Join(msgs, m.sep)
}

func JoinSep(sep string, errs ...error) error {
	if len(errs) == 0 {
		return nil
	}
	m := newMultiError(sep)
	for _, e := range errs {
		m.Add(e)
	}
	if m.errs == nil {
		return nil
	}
	return m
}