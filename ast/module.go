package ast

import (
    "strings"
    "sessions/ast/util"
)

type importModule struct {
    baseNode
    mod     string
    name    string
}

func newImportModule(mod string, name string, line int) *importModule {
    return &importModule {
        baseNode: baseNode{ line: line },
        mod: mod,
        name: name,
    }
}

func (im *importModule) prettyPrint(iw util.IndentedWriter) {
    iw.Print("import " + im.mod + " as " + im.name)
}

func (im *importModule) String() string {
    return "import " + im.mod + " as " + im.name
}

type module struct {
    baseNode
    name                    string
    imports                 []*importModule
    importMap               map[string]*importModule
    decls                   []declaration
    nameDecl                []declaration
    expressionMap           map[string]expression
    abstractionMap          map[string]*abstractionDeclaration
    typeDeclMap             map[string]*typeDeclaration
    valueMap                map[string]*valueDeclaration
    abstractionTypeAsgmMap  map[string]*abstractionTypeAssignment
    modType                 typedef
}

func newModule(imports []*importModule, line int) *module {
    return &module {
        baseNode: baseNode {
            line: line,
        },
        imports:                imports,
        decls:                  make([]declaration, 0),
        nameDecl:               make([]declaration, 0),
        expressionMap:          make(map[string]expression),
        abstractionMap:         make(map[string]*abstractionDeclaration),
        typeDeclMap:            make(map[string]*typeDeclaration),
        valueMap:               make(map[string]*valueDeclaration),
        abstractionTypeAsgmMap: make(map[string]*abstractionTypeAssignment),
    }
}

func (m *module) addDeclaration(decl declaration) {
    m.decls = append(m.decls, decl)
    switch d := decl.(type) {
        case *abstractionDeclaration:
            m.nameDecl = append(m.nameDecl, decl)
            m.expressionMap[d.getName()] = d.abstr
            m.abstractionMap[d.getName()] = d
        case *typeDeclaration:
            m.nameDecl = append(m.nameDecl, decl)
            m.typeDeclMap[d.getName()] = d
        case *valueDeclaration:
            m.nameDecl = append(m.nameDecl, decl)
            m.expressionMap[d.getName()] = d.value
            m.valueMap[d.getName()] = d
        case *abstractionTypeAssignment:
            m.abstractionTypeAsgmMap[d.getName()] = d
    }
}

func (m *module) getDecl(name string) declaration {
    if decl, ok := m.abstractionMap[name]; ok {
        return decl
    }
    return nil
}

func (m *module) getTypeDecl(name string) typedef {
    if tdecl, ok := m.typeDeclMap[name]; ok {
        return tdecl.tdef
    }

    return nil
}

func (m *module) getSession(name string) typedef {
    if decl, ok := m.abstractionTypeAsgmMap[name]; ok {
        return decl.abstrType
    }
    return nil
}

func (m *module) getAbstraction(name string) *abstraction {
    if abstDecl, ok := m.abstractionMap[name]; ok {
        return abstDecl.abstr
    }
    return nil
}

func (m *module) getValue(name string) expression {
    if decl, ok := m.valueMap[name]; ok {
        return decl.value
    }
    return nil
}

func (m *module) getImport(pack string) (string, bool) {
    if im, ok := m.importMap[pack]; ok {
        return im.mod, ok
    }
    return "", false
}

func (m *module) getType() typedef {
    if m.modType != nil {
        return m.modType
    }
    names := make([]string, 0)
    tdefs := make([]typedef, 0)
    for name := range m.abstractionMap {
        if tdef := m.getSession(name); tdef != nil {
            names = append(names, name)
            tdefs = append(tdefs, tdef)
        }
    }
    for name := range m.valueMap {
        if value := m.getValue(name); value != nil {
            names = append(names, name)
            tdefs = append(tdefs, value.getType())
        }
    }
    for name := range m.typeDeclMap {
        if tdef := m.getTypeDecl(name); tdef != nil {
            names = append(names, name)
            tdefs = append(tdefs, tdef)
        }
    }
    m.modType = newModuleType(names, tdefs, m.line)
    return m.modType
}

func (m *module) setFilename(filename string) {
    m.baseNode.setFilename(filename)
    for _, imp := range m.imports {
        imp.setFilename(filename)
    }
    for _, decl := range m.decls {
        decl.setFilename(filename)
    }
}

func (m *module) typeCheck(ctx *typeCheckContext, log util.ErrorLog) {
    var error bool = false
    importMap := make(map[string]*importModule, len(m.imports))
    for _, im := range m.imports {
        if im.mod == im.name { 
            j := strings.LastIndex(im.mod, ".")
            im.name = im.mod[j+1:]
        }
        if prev, ok := importMap[im.name]; ok {
            im.reportErrorf(
                log,
                "duplicate name definition (name=%q) for import module (pakage=%q); originally defined in line: %d",
                im.name,
                im.mod,
                prev.line,
            )
            error = true
        }   
        importMap[im.name] = im 
    }

    if error { return }

    m.importMap = importMap

    for i := range m.nameDecl {
        for j := i + 1; j < len(m.nameDecl); j++ {
            if m.nameDecl[i].getName() == m.nameDecl[j].getName() {
                m.nameDecl[j].reportErrorf(
                    log,
                    "duplicate definition of name: %q; originally defined in %s:%d",
                    m.nameDecl[j].getName(), m.nameDecl[i].file(), m.nameDecl[i].lineno(),
                )
            }
        }
    }
    for _, decl := range m.decls {
        decl.typeCheck(ctx, log)
    }
}

func (m *module) expressionCheck(ctx *expressionCheckContext, log util.ErrorLog) typedef {
    for _, decl := range m.decls {
        decl.expressionCheck(ctx, log)
    }
    return m.getType()
}

func (m *module) projectionCheck(ctx *projectionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    for _, decl := range m.decls {
        decl.projectionCheck(ctx, elog, rlog)
    }
}

func (m *module) sessionCheck(ctx *sessionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    for _, decl := range m.decls {
        decl.sessionCheck(ctx, elog, rlog)
    }
}

func (m *module) evaluate(_ *evaluationContext) expression {
    return m
}

func (m *module) operation(ctx *evaluationContext, _ expression, operator string) expression {
    if val, ok := m.expressionMap[operator]; ok {
        switch val.(type) {
            case *abstraction:
                newCtx := newEvaluationContext(ctx.p, m)
                return val.evaluate(newCtx)
            default:
                return val
        } 
    }

    return newNothingf(m.line, "unknown import module: %q", operator)
}

func (m *module) prettyPrint(iw util.IndentedWriter) {
    for _, im := range m.imports {
        im.prettyPrint(iw)
        iw.Println("")
    }
    iw.Println()
    for _, d := range m.decls {
        d.prettyPrint(iw)
        iw.Println()
        iw.Println()
    }
}

func (m *module) String() string {
    s := ""
    for _, im := range m.imports {
        s += im.String() + "\n"
    }
    for _, d := range m.decls {
        s += d.String() + "\n"
    }
    return s
}

/******************************************************************************
 * module type
 ******************************************************************************/

type moduleType struct {
    baseNode
    names []string
    tdefs []typedef
    tdefMap map[string]typedef
}

func newModuleType(names []string, tdefs []typedef, line int) *moduleType {
    tdefMap := make(map[string]typedef)
    for i, name := range names {
        tdefMap[name] = tdefs[i]
    }
    return &moduleType {
        baseNode: baseNode {
            line: line,
        },
        names: names,
        tdefs: tdefs,
        tdefMap: tdefMap,
    }
}

func (rt *moduleType) setFilename(filename string) {
    rt.baseNode.setFilename(filename)
    for _, tdef := range rt.tdefs {
        tdef.setFilename(filename)
    }
}

func (rt *moduleType) subtypeOf(tdef typedef) bool { return rt.subtypeOf_(tdef, nil) }

func (rt *moduleType) subtypeOf_(tdef typedef, _ util.Set[typePair]) bool {
    switch mtype := tdef.getType().(type) {
        case *nothingType:
            return true
        case *moduleType:
            for i, name := range mtype.names {
                td1 := mtype.tdefs[i]
                td2, ok := rt.tdefMap[name]
                if !ok || !td2.subtypeOf(td1) {
                    return false
                }
            }
            return true
        default:
            return false
    }
}

func (rt *moduleType) join(tdef typedef) (typedef, bool) {
    switch rtype := tdef.getType().(type) {
        case *nothingType:
            return rt, true
        case *moduleType:
            names := make([]string, 0, len(rtype.names))
            tdefs  := make([]typedef, 0, len(rtype.names))
            for i, name := range rtype.names {
                td1 := rtype.tdefs[i]
                td2, ok := rt.tdefMap[name]
                if !ok {
                    return rt, false
                }
                max, ok := td2.join(td1)
                if !ok {
                    return rt, false
                }
                names = append(names, name)
                 tdefs = append(tdefs, max)
            }
            return newModuleType(names, tdefs, rt.line), true
        default:
            return tdef, false
    }
}

func (mt *moduleType) defaultValue() expression {
    return newNothingf(mt.line, "unexpected module type")
}

func (rt *moduleType) getType() typedef { return rt }

func (rt *moduleType) typeCheck(ctx *typeCheckContext, log util.ErrorLog) {
    for i := range rt.names {
        for j := i + 1; j < len(rt.names); j++ {
            if rt.names[i] == rt.names[j] {
                rt.reportErrorf(log, "duplicate definition of name: %q.", rt.names[j])
            }
        }
    }
    for _, tdef := range rt.tdefs {
        tdef.typeCheck(ctx, log)
    }
}

func (rt *moduleType) projectionCheck(ctx *projectionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    for _, tdef := range rt.tdefs {
        tdef.projectionCheck(ctx, elog, rlog)
    }
}

func (rt *moduleType) prettyPrint(iw util.IndentedWriter) {
    iw.Print("module { ")
    for i, name := range rt.names {
        iw.Print(name + " ")
        rt.tdefs[i].prettyPrint(iw)
        iw.Print("; ")
    }
    iw.Print(" }")
}

func (rt *moduleType) String() string {
    s := "module { "
    for i, name := range rt.names {
        s += name + " " + rt.tdefs[i].String() + "; "
    }
    return s + " }"
}