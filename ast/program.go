package ast

import (
    "fmt"
    "strings"
    "sessions/ast/util"
)

type program struct {
    file        string
    modules     []*module
    moduleMap   map[string]*module
    mainModule  *module
    suffix      string
}

func NewProgram(file string) *program {
    return &program{
        file:    file,
        moduleMap: make(map[string]*module),
        suffix:  ".sess",
    }
}

func (p *program) Parse() util.ErrorLog {
    log := newCustomErrorListener()

    if !strings.HasSuffix(p.file, p.suffix) {
        msg := fmt.Sprintf("file %q should have suffix %q.", p.file, p.suffix)
        return log.Add(util.NewSystemError(msg))
    }

    moduleName := strings.TrimSuffix(p.file, p.suffix)
    p.mainModule = p.parseModule(moduleName, log)

    return log
}

func (p *program) parseModule(moduleName string, log *customErrorListener) *module {
    if m, ok := p.moduleMap[moduleName]; ok {
        return m
    }
    filename := moduleName + p.suffix
    m := parse(filename, log)
    if m == nil {
        return nil
    }

    m.name = moduleName
    m.setFilename(filename)

    p.modules = append(p.modules, m)
    p.moduleMap[moduleName] = m

    for _, im := range m.imports {
        imp := strings.ReplaceAll(im.mod, ".", "\\")
        p.parseModule(imp, log)
    }

    return m
}

func (p *program) getModule(name string) *module {
    imp := strings.ReplaceAll(name, ".", "\\")
    if mod, ok := p.moduleMap[imp]; ok {
        return mod
    }
    return nil
}

func (p *program) Validate(config *util.Config) (elog util.ErrorLog, rlog util.ReportLog) {
    elog = util.NewErrorLog()
    rlog = util.NewReportLog()
    // mod := p.mainModule
    for _, mod := range p.modules {
        ctx := newTypeCheckContext(p, mod)
        mod.typeCheck(ctx, elog)
        if elog.HasErrors() {
            return
        }
    }
    for _, mod := range p.modules {
        ctx := newProjectionCheckContext(config)
        mod.projectionCheck(ctx, elog, rlog)
        if elog.HasErrors() {
            return
        }
    }
    for _, mod := range p.modules {
        ctx := newExpressionCheckContext(p, mod)
        mod.expressionCheck(ctx, elog)
        if elog.HasErrors() {
            return
        }
    }
    for _, mod := range p.modules {
        ctx := newSessionCheckContext(config)
        mod.sessionCheck(ctx, elog, rlog)
        if elog.HasErrors() {
            return
        }
    }
    return
}

func (p *program) Execute() {
    ctx := newEvaluationContext(p, p.mainModule)
    if main := p.mainModule.getDecl("main"); main != nil {
        main.execute(ctx)
    }
    ctx.wait()
}

func (p *program) PrettyPrint(iw util.IndentedWriter) {
    for _, mod := range p.modules {
        mod.prettyPrint(iw)
        iw.Println("")
    }
}

