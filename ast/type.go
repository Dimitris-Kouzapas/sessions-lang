package ast

import (
    "fmt"
    "sessions/ast/util"
)

type typedef interface {
    setFilename(string)
    reportErrorf(util.ErrorLog, string, ...interface{})
    getType() typedef
    subtypeOf(typedef) bool
    subtypeOf_(typedef, util.Set[typePair]) bool
    join(typedef) (typedef, bool)
    typeCheck(*typeCheckContext, util.ErrorLog)
    projectionCheck(*projectionCheckContext, util.ErrorLog, util.ReportLog)
    defaultValue() expression
    prettyPrint(util.IndentedWriter)
    fmt.Stringer
}

/******************************************************************************
 * subtyping
 ******************************************************************************/

type typePair struct {
    a typedef
    b typedef
}

/******************************************************************************
 * nothing type
 ******************************************************************************/

type nothingType struct {
    baseNode
}

func newNothingType() *nothingType {
    return &nothingType {
        baseNode: baseNode {
            line: 0,
        },
    }
}

func (n *nothingType) getType() typedef { return n }
func (_ *nothingType) typeCheck(_ *typeCheckContext, _ util.ErrorLog) {}
func (_ *nothingType) projectionCheck(_ *projectionCheckContext, _ util.ErrorLog, _ util.ReportLog) {}
func (n *nothingType) subtypeOf(tdef typedef) bool { return n.subtypeOf_(tdef, nil) }
func (_ *nothingType) subtypeOf_(_ typedef, _ util.Set[typePair]) bool { return true }
func (n *nothingType) join(tdef typedef) (typedef, bool) { return tdef, true }
// func (n *nothingType) meet(tdef typedef) (typedef, bool) { return n, true }
func (n *nothingType) defaultValue() expression { return newNothingf(n.line, "nothing expression") }
func (n *nothingType) prettyPrint(iw util.IndentedWriter) { iw.Print(n.String()) }
func (_ *nothingType) String() string { return "nothing" }

/******************************************************************************
 * primitive types
 ******************************************************************************/

/******************
 * primitive type
 ******************/

type primitiveType struct {
    baseNode
    typeString string
}

func (_ *primitiveType) typeCheck(_ *typeCheckContext, _ util.ErrorLog) {}
func (_ *primitiveType) projectionCheck(_ *projectionCheckContext, _ util.ErrorLog, _ util.ReportLog) {}
func (pt *primitiveType) prettyPrint(iw util.IndentedWriter) { iw.Print(pt.String()) }
func (pt *primitiveType) String() string { return pt.typeString }

/******************
 * boolean type
 ******************/

type boolType struct {
    primitiveType
}

func newBoolType(line int) *boolType {
    return &boolType {
        primitiveType: primitiveType {
            typeString: "bool",
            baseNode: baseNode {
                line: line,
            },
        },
    }
}

func (bt *boolType) getType() typedef               { return bt }
func (bt *boolType) subtypeOf(tdef typedef) bool    { return bt.subtypeOf_(tdef, nil) }

func (_ *boolType) subtypeOf_(tdef typedef, _ util.Set[typePair]) bool {
    switch tdef.getType().(type) {
        case *nothingType, *boolType:
            return true
        default:
            return false
    }
}

func (bt *boolType) join(tdef typedef) (typedef, bool) {
    switch tdef.getType().(type) {
        case *nothingType, *boolType:
            return bt, true
        default:
            return tdef, false
    }
}

func (bt *boolType) defaultValue() expression {
    return newFalseExpr(bt.line)
}

/******************
 * integer type
 ******************/

type intType struct {
    primitiveType
}

func newIntType(line int) *intType {
    return &intType {
        primitiveType: primitiveType {
            typeString: "int",
            baseNode: baseNode {
                line: line,
            },
        },
    }
}

func (it *intType) getType() typedef { return it }
func (it *intType) subtypeOf(tdef typedef) bool { return it.subtypeOf_(tdef, nil) }

func (_ *intType) subtypeOf_(tdef typedef, _ util.Set[typePair]) bool {
    switch tdef.getType().(type) {
        //case *anyTC, *eq, *num, *ord, *intType, *floatType:
        case *nothingType, *intType, *floatType:
            return true
        default:
            return false
    }
}

func (it *intType) join(tdef typedef) (typedef, bool) {
    switch tdef.getType().(type) {
        case *nothingType, *intType:
            return it, true
        case *floatType:
            return tdef, true
        default:
            return tdef, false
    }
}

func (it *intType) defaultValue() expression {
    return newIntExpr("0", it.line)
}

/******************
 * float type
 ******************/

type floatType struct {
    primitiveType
}

func newFloatType(line int) *floatType {
    return &floatType {
        primitiveType: primitiveType {
            typeString: "float",
            baseNode: baseNode {
                line: line,
            },
        },
    }
}

func (ft *floatType) getType() typedef            { return ft }
func (ft *floatType) subtypeOf(tdef typedef) bool { return ft.subtypeOf_(tdef, nil) }

func (_ *floatType) subtypeOf_(tdef typedef, _ util.Set[typePair]) bool {
    switch tdef.getType().(type) {
        case *nothingType, *floatType:
            return true
        default:
            return false
    }
}

func (ft *floatType) join(tdef typedef) (typedef, bool) {
    switch tdef.getType().(type) {
        case *nothingType, *intType, *floatType:
            return ft, true
        default:
            return tdef, false
    }
}
 
 func (ft *floatType) defaultValue() expression {
    return newFloatExpr("0", ft.line)
 }

/******************
 * string type
 ******************/

type stringType struct {
    primitiveType
}

func newStringType(line int) *stringType {
    return &stringType {
        primitiveType: primitiveType {
            typeString: "string",
            baseNode: baseNode {
                line: line,
            },
        },
    }
}

func (st *stringType) getType() typedef            { return st }
func (st *stringType) subtypeOf(tdef typedef) bool { return st.subtypeOf_(tdef, nil) }

func (_ *stringType) subtypeOf_(tdef typedef, _ util.Set[typePair]) bool {
    switch tdef.getType().(type) {
        case *nothingType, *stringType:
            return true
        default:
            return false
    }
}

func (st *stringType) join(tdef typedef) (typedef, bool) {
    switch tdef.getType().(type) {
        case *nothingType, *stringType:
            return st, true
        default:
            return tdef, false
    }
}

func (st *stringType) defaultValue() expression {
    return newStringExpr("", st.line)
}

/******************************************************************************
 * list type
 ******************************************************************************/

type listType struct {
    baseNode
    tdef typedef
}

func newListType(tdef typedef, line int) *listType {
    return &listType {
        baseNode: baseNode {
            line: line,
        },
        tdef: tdef,
    }
}

func (lt *listType) setFilename(filename string) {
    lt.baseNode.setFilename(filename)
    lt.tdef.setFilename(filename)
}

func (lt *listType) getType() typedef { return lt }
func (lt *listType) subtypeOf(tdef typedef) bool { return lt.subtypeOf_(tdef, nil) }

func (lt *listType) subtypeOf_(tdef typedef, _ util.Set[typePair]) bool {
    switch at := tdef.getType().(type) {
        case *nothingType:
            return true
        case *listType:
            return lt.tdef.subtypeOf(at.tdef)
        default:
            return false
    }
}

func (lt *listType) join(tdef typedef) (typedef, bool) {
    switch at := tdef.getType().(type) {
        case *nothingType:
            return lt, true
        case *listType:
            td, ok := lt.tdef.join(at.tdef)
            if ok == true {
                return newListType(td, lt.line), true
            } else {
                return lt, false
            }
        default:
            return lt, false
    }
}

func (lt *listType) typeCheck(ctx *typeCheckContext, log util.ErrorLog) {
    lt.tdef.typeCheck(ctx, log)
}

func (lt *listType) projectionCheck(ctx *projectionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    lt.tdef.projectionCheck(ctx, elog, rlog)
}

func (lt *listType) prettyPrint(iw util.IndentedWriter) {
    iw.Print("[")
    lt.tdef.prettyPrint(iw)
    iw.Print("]")
}

func (lt *listType) String() string {
    return "[" + lt.tdef.String() + "]"
}

func (lt *listType) defaultValue() expression {
    return newListExpr(make([]expression, 0), lt.line)
}

/******************************************************************************
 * label type
 ******************************************************************************/

type labelType struct {
    baseNode
    label string
}

func newLabelType(label string, line int) *labelType {
    return &labelType {
        baseNode: baseNode {
            line: line,
        },
        label: label,
    }
}

func (_ *labelType) typeCheck(_ *typeCheckContext, _ util.ErrorLog) {}
func (_ *labelType) projectionCheck(_ *projectionCheckContext, _ util.ErrorLog, _ util.ReportLog) {}
func (lt *labelType) subtypeOf(tdef typedef) bool { return lt.subtypeOf_(tdef, nil) }

func (lt *labelType) subtypeOf_(tdef typedef, _ util.Set[typePair]) bool {
    switch ltype := tdef.getType().(type) {
        case *labelType:
            return lt.label == ltype.label
        default:
            return false
    }
}

func (_ *labelType) join(_ typedef) (typedef, bool)         { return nil, false }

func (lt *labelType) defaultValue() expression              { return newLabelExpr("", lt.line) }

func (lt *labelType) getType() typedef                      { return lt }
func (lt *labelType) prettyPrint(iw util.IndentedWriter)    { iw.Print(lt.String()) }
func (lt *labelType) String() string                        { return lt.label }

/******************************************************************************
 * record type
 ******************************************************************************/

type recordType struct {
    baseNode
    labels []string
    tdefs []typedef
    tdefMap map[string]typedef
}

func newRecordType(labels []string, tdefs []typedef, line int) *recordType {
    tdefMap := make(map[string]typedef)
    for i, label := range labels {
        tdefMap[label] = tdefs[i]
    }
    return &recordType {
        baseNode: baseNode {
            line: line,
        },
        labels: labels,
        tdefs: tdefs,
        tdefMap: tdefMap,
    }
}

func (rt *recordType) setFilename(filename string) {
    rt.baseNode.setFilename(filename)
    for _, tdef := range rt.tdefs {
        tdef.setFilename(filename)
    }
}

func (rt *recordType) subtypeOf(tdef typedef) bool { return rt.subtypeOf_(tdef, nil) }

//TODO: add more operators to recordType
func (rt *recordType) subtypeOf_(tdef typedef, _ util.Set[typePair]) bool {
    switch rtype := tdef.getType().(type) {
        case *nothingType:
            return true
        case *recordType:
            for i, lab := range rtype.labels {
                td1 := rtype.tdefs[i]
                td2, ok := rt.tdefMap[lab]
                if !ok || !td2.subtypeOf(td1) {
                    return false
                }
            }
            return true
        default:
            return false
    }
}

func (rt *recordType) join(tdef typedef) (typedef, bool) {
    switch rtype := tdef.getType().(type) {
        case *nothingType:
            return rt, true
        case *recordType:
            labels := make([]string, 0, len(rtype.labels))
            tdefs  := make([]typedef, 0, len(rtype.labels))
            for i, lab := range rtype.labels {
                td1 := rtype.tdefs[i]
                td2, ok := rt.tdefMap[lab]
                if !ok {
                    return rt, false
                }
                max, ok := td2.join(td1)
                if !ok {
                    return rt, false
                }
                labels = append(labels, lab)
                tdefs = append(tdefs, max)
            }
            return newRecordType(labels, tdefs, rt.line), true
        default:
            return tdef, false
    }
}

func (rt *recordType) defaultValue() expression {
    expressions := make([]expression, len(rt.tdefs))
    for i, tdef := range rt.tdefs {
        expressions[i] = tdef.defaultValue()
    }
    return newRecordExpr(rt.labels, expressions, rt.line)
}

func (rt *recordType) getType() typedef { return rt }

func (rt *recordType) typeCheck(ctx *typeCheckContext, log util.ErrorLog) {
    for i := range rt.labels {
        for j := i + 1; j < len(rt.labels); j++ {
            if rt.labels[i] == rt.labels[j] {
                rt.reportErrorf(log, "duplicate definition of record label: %q.", rt.labels[j])
            }
        }
    }
    for _, tdef := range rt.tdefs {
        tdef.typeCheck(ctx, log)
    }
}

func (rt *recordType) projectionCheck(ctx *projectionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    for _, tdef := range rt.tdefs {
        tdef.projectionCheck(ctx, elog, rlog)
    }
}

func (rt *recordType) prettyPrint(iw util.IndentedWriter) {
    iw.Print("record { ")
    for i, label := range rt.labels {
        iw.Print(label + " ")
        rt.tdefs[i].prettyPrint(iw)
        iw.Print("; ")
    }
    iw.Print(" }")
}

func (rt *recordType) String() string {
    s := "record { "
    for i, label := range rt.labels {
        s += label + " " + rt.tdefs[i].String() + "; "
    }
    return s + " }"
}

/******************************************************************************
 * name type
 ******************************************************************************/
type nameType struct {
    baseNode
    name string
    tdef typedef
}

func newNameType(name string, line int) *nameType {
    return &nameType {
        baseNode: baseNode {
            line: line,
        },
        name: name,
    }
}

func (nt *nameType) getType() typedef { return nt.tdef }

func (nt *nameType) typeCheck(ctx *typeCheckContext, log util.ErrorLog) {
    if nt.tdef = ctx.getTypeDecl(nt.name); nt.tdef == nil {
        nt.reportErrorf(log, "undefined type: %q.", nt.name)
    }
}

func (_ *nameType) projectionCheck(_ *projectionCheckContext, _ util.ErrorLog, _ util.ReportLog) {}

func (nt *nameType) subtypeOf(tdef typedef) bool {
    visited := util.NewHashSet[typePair]()
    return nt.subtypeOf_(tdef, visited)
}

func (nt *nameType) subtypeOf_(tdef typedef, visited util.Set[typePair]) bool {
    if nt.tdef == nil { return false }
    return nt.tdef.subtypeOf_(tdef, visited)
}

func (nt *nameType) join(tdef typedef) (typedef, bool) {
    if nt.tdef == nil { return tdef, false }
    return nt.tdef.join(tdef)
}

func (nt *nameType) defaultValue() expression {
    if nt.tdef == nil { return newNothingf(nt.line, "unknown type: %q", nt.name) }
    return nt.tdef.defaultValue()
}

func (nt *nameType) prettyPrint(iw util.IndentedWriter) { iw.Print(nt.String()) }
func (nt *nameType) String() string                     { return nt.name }

/******************************************************************************
 * access type
 ******************************************************************************/
type accessType struct {
    nameType
    access string
}

func newAccessType(access string, name string, line int) *accessType {
    return &accessType {
        nameType: nameType {
            baseNode: baseNode{line: line},
            name: name,
        },
        access: access,
    }
}

func (at *accessType) typeCheck(ctx *typeCheckContext, log util.ErrorLog) {
    tdef := ctx.getTypeDecl(at.access)
    if tdef == nil {
        at.reportErrorf(log, "undefined module: %q.", at.access)
        return
    }
    switch m := tdef.(type) {
        case *moduleType:
            td, ok := m.tdefMap[at.name]
            if !ok {
                at.reportErrorf(log, "module %q; unknown type: %q.", at.access, at.name)
                return
            }
            at.tdef = td
        default:
            at.reportErrorf(log, "expecting module type but found: %q.", tdef.String())
    }
}

func (at *accessType) prettyPrint(iw util.IndentedWriter) { iw.Print(at.String()) }
func (at *accessType) String() string                     { return at.access + "." + at.nameType.String() }

/******************************************************************************
 * participantType type
 ******************************************************************************/

type participantType struct {
    baseNode
    participant string
}

func newParticipantType(participant string, line int) *participantType {
    return &participantType {
        baseNode: baseNode {
            line: line,
        },
        participant: participant,
    }
}

func (pt *participantType) getType() typedef { return pt }
func (_ *participantType) typeCheck(_ *typeCheckContext, log util.ErrorLog) {}
func (_ *participantType) projectionCheck(_ *projectionCheckContext, _ util.ErrorLog, _ util.ReportLog) {}
func (pt *participantType) subtypeOf(tdef typedef) bool { return pt.subtypeOf_(tdef, nil) }

func (pt *participantType) subtypeOf_(tdef typedef, _ util.Set[typePair]) bool {
    switch ptype := tdef.getType().(type) {
        case *nothingType:
            return true
        case *participantType:
            return pt.participant == ptype.participant
        default:
            return false
    }
}

func (pt *participantType) join(tdef typedef) (typedef, bool) {
    switch ptype := tdef.getType().(type) {
        case *nothingType:
            return pt, true
        case *participantType:
            return pt, pt.participant == ptype.participant
        default:
            return tdef, false
    }
}

func (pt *participantType) defaultValue() expression {
    return newParticipantExpr(pt.participant, pt.line)
}

func (pt *participantType) prettyPrint(iw util.IndentedWriter) { iw.Print(pt.String()) }
func (pt *participantType) String() string { return pt.participant }


/******************************************************************************
 * types and typeclasses
 *****************************************************************************/

// TODO Possibly even add polymorphism
type typeClass interface {
    contains(typedef) bool
    fmt.Stringer
}

// Zero-sized types (no fields).
type anyTC struct{}
type eqTC  struct{}
type numTC struct{}
type ordTC struct{}

func (anyTC) contains(_ typedef) bool { return true }
func (anyTC) String() string          { return "Any" }

func (eqTC) contains(tdef typedef) bool {
    switch tdef.getType().(type) {
        case *boolType, *intType, *floatType, *stringType, *listType:
            return true
        default:
            return false
    }
}
func (eqTC) String() string { return "Eq" }

func (numTC) contains(tdef typedef) bool {
    switch tdef.getType().(type) {
        case *intType, *floatType, *stringType, *listType:
            return true
        default:
            return false
    }
}
func (numTC) String() string { return "Num" }

func (ordTC) contains(tdef typedef) bool {
    switch tdef.getType().(type) {
        case *intType, *floatType, *stringType:
            return true
        default:
            return false
    }
}
func (ordTC) String() string { return "Ord" }

// Singletons (no allocations; zero values).
var (
    anytc typeClass = anyTC{}
    eq  typeClass = eqTC{}
    num typeClass = numTC{}
    ord typeClass = ordTC{}
)

/******************************************************************************
* localAbstraction
*******************************************************************************/

type localAbstraction struct {
    baseNode
    parameters []*participantType
    loc local
}

func newLocalAbstraction(parameters []*participantType, loc local, line int) *localAbstraction {
    return &localAbstraction {
        baseNode: baseNode {
            line: line,
        },
        parameters: parameters,
        loc: loc,
    }
}

func (la *localAbstraction) setFilename(filename string) {
    la.baseNode.setFilename(filename)
    for _, p := range la.parameters {
        p.setFilename(filename)
    }
    la.loc.setFilename(filename)
}

func (la *localAbstraction) getType() typedef { return la }
func (la *localAbstraction) typeCheck(ctx *typeCheckContext, log util.ErrorLog) {
    la.loc.typeCheck(ctx, log)
}

func (la *localAbstraction) projectionCheck(ctx *projectionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    ctx.push()
    defer ctx.pop()
    for _, p := range la.parameters {
        if ctx.addParticipant(p) == false {
            p.reportErrorf(elog, "duplicated participant definition: %q.", p.String())
        }
    }
    for _, p := range la.parameters {
        p.projectionCheck(ctx, elog, rlog)
    }
    la.loc.projectionCheck(ctx, elog, rlog)
}

func (la *localAbstraction) subtypeOf(tdef typedef) bool {
    visited := util.NewHashSet[typePair]()
    return la.subtypeOf_(tdef, visited)
}

func (la *localAbstraction) subtypeOf_(tdef typedef, visited util.Set[typePair]) bool {
    tPair := typePair{a: la, b: tdef}

    if visited.Contains(tPair) == true {
        return true
    }

    visited.Add(tPair)

    switch locAbstr := tdef.getType().(type) {
        case *localAbstraction:
            if (len(la.parameters) != len(locAbstr.parameters)) {
                // error no subtype
                return false
            }
            labstr := locAbstr.substitute(la.parameters)
            return la.loc.subtypeOf(labstr.loc, visited)
        default:
            return false
    }
}

func (la *localAbstraction) join(tdef typedef) (typedef, bool) {
    r, ok := tdef.getType().(*localAbstraction)
    if !ok || r == nil {
        return la, false
    }
    if len(la.parameters) != len(r.parameters) {
        return la, false
    }
    for i, p := range la.parameters {
        if !p.subtypeOf(r.parameters[i]) {
            return la, false
        }
    }

    loc, result := la.loc.join(r.loc)
    return newLocalAbstraction(la.parameters, loc, la.line), result
}

func (la *localAbstraction) substitute(ptypes []*participantType) *localAbstraction {
    substitution := make(map[string]string)
    parameters := make([]*participantType, len(la.parameters))
    for i, p := range la.parameters {
        if i < len(ptypes) {
            substitution[p.participant] = ptypes[i].participant
            parameters[i] = ptypes[i]
        } else {
            substitution[p.participant] = p.participant
            parameters[i] = p
        }
    }
    loc := la.loc.substitute(substitution)
    return newLocalAbstraction(parameters, loc, la.line)
}

func (la *localAbstraction) apply(arguments []*participantExpr) local {
    if (len(la.parameters) != len(arguments)) {
        // error no join
        return la.loc
    }
    substitution := make(map[string]string)
    for i := range la.parameters {
        substitution[la.parameters[i].participant] = arguments[i].id
    }
    return la.loc.substitute(substitution)
}

func (la *localAbstraction) defaultValue() expression {
    return newNothingf(la.line, "unexpected local abstraction type")
}

func (la *localAbstraction) prettyPrint(iw util.IndentedWriter) {
    iw.Print("local")
    for _, participant := range la.parameters {
        iw.Print(" ")
        participant.prettyPrint(iw)
    }
    iw.Print(". ")
    iw.Inc()
    iw.Println("")
    la.loc.prettyPrint(iw)
    iw.Dec()
    return
}

func (la *localAbstraction) String() string {
    s := "local"
    for _, participant := range la.parameters {
        s += " " + participant.String()
    }
    return s + ". " + la.loc.String()
}

/******************************************************************************
* projection
*******************************************************************************/

type projection struct {
    baseNode
    ptype       *participantType
    conf         typedef
    locAbstr    *localAbstraction
}

func newProjection(conf typedef, ptype *participantType, line int) *projection {
    return &projection {
        baseNode: baseNode {
            line: line,
        },
        ptype: ptype,
        conf: conf,
    }
}

func (p *projection) getType() typedef { return p.locAbstr }

func (p *projection) setFilename(filename string) {
    p.baseNode.setFilename(filename)
    p.ptype.setFilename(filename)
    p.conf.setFilename(filename)
}

func (p *projection) typeCheck(ctx *typeCheckContext, log util.ErrorLog) {
    p.conf.typeCheck(ctx, log)
    conf := p.conf.getType()
    if conf == nil {
        return
    }
    _, ok := conf.(globalConfig)
    if !ok {
        p.reportErrorf(log, "expecting global type; instead found %q", conf.String())
        return
    }
}

func (p *projection) projectionCheck(ctx *projectionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    p.conf.projectionCheck(ctx, elog, rlog)
    // TODO: ensure p.ptype is one of gconfig.participants()
    gconfig := p.conf.getType().(globalConfig)
    p.locAbstr = gconfig.project(p.ptype)
}

func (p *projection) subtypeOf(tdef typedef) bool {
    visited := util.NewHashSet[typePair]()
    return p.subtypeOf_(tdef, visited)
}

func (p *projection) subtypeOf_(tdef typedef, visited util.Set[typePair]) bool {
    if p.locAbstr == nil { return false }
    return p.locAbstr.subtypeOf_(tdef, visited)
}

func (p *projection) join(tdef typedef) (typedef, bool) {
    if p.locAbstr == nil { return p, false }
    return p.locAbstr.join(tdef)
}

func (p *projection) apply(arguments []*participantExpr) local {
    return p.locAbstr.apply(arguments)
}

func (p *projection) defaultValue() expression {
    if p.locAbstr == nil {
        return newNothingf(p.line, "unprojected type: %q", p.String())
    }
    return p.locAbstr.defaultValue()
}

func (p *projection) prettyPrint(iw util.IndentedWriter) {
    p.conf.prettyPrint(iw)
    iw.Print("(")
    p.ptype.prettyPrint(iw)
    iw.Print(")")
}

func (p *projection) String() string {
    s := p.conf.String() + "("
    s += p.ptype.String()
    return s + ")"
}

/*****************************************************************************
 * abstraction type
 *****************************************************************************/

type abstractionType struct {
    baseNode
    initAbstr   *localAbstraction
    tdef        typedef

    la          *localAbstraction
}

func newAbstractionType(initAbstr *localAbstraction, tdef typedef, line int) *abstractionType {
    return &abstractionType {
        baseNode:   baseNode{ line: line }, 
        initAbstr:  initAbstr,
        tdef:       tdef,
    }
}

func (at *abstractionType) locAbstr() *localAbstraction {
    if at.tdef == nil || at.la != nil {
        return at.la
    }
    switch locAbstr := at.tdef.getType().(type) {
        case *localAbstraction:
            at.la = locAbstr
    }
    return at.la
}

func (at *abstractionType) getType() typedef { return at }

func (at *abstractionType) setFilename(filename string) {
    at.baseNode.setFilename(filename)
    at.tdef.setFilename(filename)
    at.initAbstr.setFilename(filename)
}

func (at *abstractionType) typeCheck(ctx *typeCheckContext, log util.ErrorLog) {
    at.tdef.typeCheck(ctx, log)
    at.initAbstr.typeCheck(ctx, log)

    la := at.tdef.getType()
    if la == nil {
        return
    }
    ok := false
    _, ok = la.(*localAbstraction)
    if !ok {
        at.tdef.reportErrorf(log, "expecting local abstraction; instead found %q", la.String())
    }
}

func (at *abstractionType) projectionCheck(ctx *projectionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    at.tdef.projectionCheck(ctx, elog, rlog)
    at.initAbstr.projectionCheck(ctx, elog, rlog)
} 

func (at *abstractionType) subtypeOf(tdef typedef) bool {
    visited := util.NewHashSet[typePair]()
    return at.subtypeOf_(tdef, visited)
}

func (at *abstractionType) subtypeOf_(tdef typedef, visited util.Set[typePair]) bool {
    tPair := typePair{a: at, b: tdef}

    if visited.Contains(tPair) == true {
        return true
    }

    visited.Add(tPair)

    switch abstrType := tdef.getType().(type) {
        case *abstractionType:
            //b := at.la.subtypeOf_(abstrType.la, visited)
            b := at.tdef.subtypeOf_(abstrType.tdef, visited)
            if !b { return false }
            return at.initAbstr.subtypeOf_(abstrType.initAbstr, visited)
        default:
            return false
    }
}

func (at *abstractionType) join(tdef typedef) (typedef, bool) {
    abstrType, ok := tdef.getType().(*abstractionType)
    if !ok || abstrType == nil {
        return at, false
    }
    //la, ok := at.la.join(abstrType.la)
    la, ok := at.tdef.join(abstrType.tdef)
    if !ok {
        return at, false
    }
    initAbstr, ok := at.initAbstr.join(abstrType.initAbstr)
    if !ok {
        return at, false
    }

    return newAbstractionType(initAbstr.(*localAbstraction), la.(*localAbstraction), at.line), true
}

func (at *abstractionType) substitute(abstrType *abstractionType) *abstractionType {
    ptypes := abstrType.locAbstr().parameters
    iptypes := abstrType.initAbstr.parameters
    la := at.locAbstr().substitute(ptypes)
    initAbstr := at.initAbstr.substitute(iptypes)
    return newAbstractionType(initAbstr, la, at.line)
}

func (at *abstractionType) defaultValue() expression {
    return newNothingf(at.line, "unexpected abstraction type")
}

func (at *abstractionType) prettyPrint(iw util.IndentedWriter) {
    iw.Inc()
    iw.Print("init")
    for _, p := range at.initAbstr.parameters {
        iw.Print(" ")
        p.prettyPrint(iw)
    }
    iw.Print(". ")
    iw.Println("")
    iw.Inc()
    at.initAbstr.loc.prettyPrint(iw)
    iw.Println("")
    iw.Dec()
    iw.Print("| ")
    at.tdef.prettyPrint(iw)
    iw.Dec()
}

func (at *abstractionType) String() string {
    s := "init"
    for _, p := range at.initAbstr.parameters {
        s += " " + p.String()
    }
    s += ". " + at.initAbstr.loc.String()
    return s + " | " + at.tdef.String() 
}

/**********************************************************************************
 * ioType
 **********************************************************************************/

 type ioType struct {
    baseNode
 }

 func newioType(line int) *ioType {
    return &ioType {
        baseNode{line: line},
    }
 }

func (iot *ioType) getType() typedef { return iot }
func (iot *ioType) subtypeOf(tdef typedef) bool { return iot.subtypeOf_(tdef, nil) }
func (iot *ioType) subtypeOf_(tdef typedef, _ util.Set[typePair]) bool {
    switch tdef.(type) {
        case *nothingType, *ioType:
            return true
        default:
            return false
    }
}

func (iot *ioType) join(tdef typedef) (typedef, bool) {
    switch tdef.(type) {
        case *nothingType, *ioType:
            return iot, true
        default:
            return tdef, false
    }
}

func (iot *ioType) typeCheck(_ *typeCheckContext, _ util.ErrorLog) {}
func (iot *ioType) projectionCheck(_ *projectionCheckContext, _ util.ErrorLog, _ util.ReportLog) {}


func (iot *ioType) defaultValue() expression {
    return newEmptyHandle(iot.line)
}

func (iot *ioType) prettyPrint(iw util.IndentedWriter) { iw.Print( iot.String() ) }
func (_ *ioType) String() string { return "io" }
