package main
import (
    "fmt"
    "os"
    "sessions/ast"
    "sessions/ast/util"
)

func runFile(file string, cfg *util.Config) {
    program := ast.NewProgram(file)

    log := program.Parse()
    if log.HasErrors() {
        log.PrintErrors()
        return
    }

    elog, rlog := program.Validate(cfg)
    if elog.HasErrors() {
        elog.PrintErrors()
        return
    }

    if rlog.HasReports() {
        rlog.PrintReports()
    }

    if ok1, ok2 := cfg.Bool("prettyPrint"); ok1 && ok2 {
        stream := util.NewStream()
        program.PrettyPrint(stream)
        fmt.Fprintln(os.Stdout, stream)
    }

    if ok1, ok2 := cfg.Bool("no-exec"); !ok1 && ok2 {
        program.Execute()
    }
}

 /*********************
   main
  *********************/


func main() {
    defs := []util.FlagDef {
        {"verbose", []string{"v"}, false, "Enable verbose mode"},
        {"analysis", []string{"a"}, false, "Enable analysis mode"},
        {"no-exec", []string{"ne"}, false, "Disable execution mode"},
        {"prettyPrint", []string{"p"}, false, "Enable prettyPrint mode"},
        {"usage", []string{"u"}, false, "Print usage"},
    }

    cfg := util.ParseFlags(defs, os.Args[1:])
    if ok1, ok2 := cfg.Bool("usage"); ok1 && ok2 {
        fmt.Fprintln(os.Stdout, cfg.Usage())
    }
    files := cfg.Args()

    for _, file := range files {
        runFile(file, cfg)
    }
    return
}
