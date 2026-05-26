package util

import (
	"bytes"
	"flag"
	"fmt"
)

type FlagDef struct {
    Name        string
    Aliases     []string
    Default     interface{}
    Description string
}


type Config struct {
	boolPtr   map[string]*bool
	strPtr    map[string]*string
	intPtr    map[string]*int
	args      []string
	usageText string
}

func ParseFlags(defs []FlagDef, args []string) *Config {
    fs := flag.NewFlagSet("sesspi", flag.ContinueOnError)

    cfg := &Config{
        boolPtr: map[string]*bool{},
        strPtr:  map[string]*string{},
        intPtr:  map[string]*int{},
    }

    for _, d := range defs {
        names := append([]string{d.Name}, d.Aliases...) // primary + aliases

        switch v := d.Default.(type) {
        case bool:
            p := new(bool)
            *p = v
            for i, name := range names {
                desc := d.Description
                if i > 0 {
                    desc += fmt.Sprintf(" (alias of -%s)", d.Name)
                }
                fs.BoolVar(p, name, v, desc)
                cfg.boolPtr[name] = p
            }

        case string:
            p := new(string)
            *p = v
            for i, name := range names {
                desc := d.Description
                if i > 0 { desc += fmt.Sprintf(" (alias of -%s)", d.Name) }
                fs.StringVar(p, name, v, desc)
                cfg.strPtr[name] = p
            }

        case int:
            p := new(int)
            *p = v
            for i, name := range names {
                desc := d.Description
                if i > 0 { desc += fmt.Sprintf(" (alias of -%s)", d.Name) }
                fs.IntVar(p, name, v, desc)
                cfg.intPtr[name] = p
            }

        default:
            panic(fmt.Errorf("unsupported flag type for %q", d.Name))
        }
    }

    _ = fs.Parse(args)
    cfg.args = fs.Args()

    var buf bytes.Buffer
    fs.SetOutput(&buf)
    fs.PrintDefaults()
    cfg.usageText = buf.String()

    return cfg
}

// --- Accessors ---
func (c *Config) Usage() string  { return c.usageText }
func (c *Config) Args() []string { return c.args }

func (c *Config) Bool(name string) (bool, bool) {
	p, ok := c.boolPtr[name]
	if !ok { return false, false }
	return *p, true
}
func (c *Config) String(name string) (string, bool) {
	p, ok := c.strPtr[name]
	if !ok { return "", false }
	return *p, true
}
func (c *Config) Int(name string) (int, bool) {
	p, ok := c.intPtr[name]
	if !ok { return 0, false }
	return *p, true
}
func (c *Config) TypeOf(name string) (string, bool) {
	if _, ok := c.boolPtr[name]; ok { return "bool", true }
	if _, ok := c.strPtr[name]; ok { return "string", true }
	if _, ok := c.intPtr[name]; ok { return "int", true }
	return "", false
}