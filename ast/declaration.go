package ast

import (
    "fmt"
    "sessions/ast/util"
)

type declaration interface {
    setFilename(string)
    reportErrorf(util.ErrorLog, string, ...interface{})
    lineno() int
    getName() string
    file() string
    typeCheck(*typeCheckContext, util.ErrorLog)
    projectionCheck(*projectionCheckContext, util.ErrorLog, util.ReportLog)
    expressionCheck(*expressionCheckContext, util.ErrorLog)
    sessionCheck(*sessionCheckContext, util.ErrorLog, util.ReportLog)
    execute(*evaluationContext)
    prettyPrint(util.IndentedWriter)
    fmt.Stringer
}

type declarationImpl struct {
    baseNode
    name string
}

func (d *declarationImpl) getName() string                            { return d.name }
func (d *declarationImpl) setFilename(filename string)                { d.baseNode.setFilename(filename) }
func (_ *declarationImpl) expressionCheck(_ *expressionCheckContext, _ util.ErrorLog) {}
func (d *declarationImpl) projectionCheck(_ *projectionCheckContext, _ util.ErrorLog, _ util.ReportLog) {}
func (_ *declarationImpl) sessionCheck(_ *sessionCheckContext, _ util.ErrorLog, _ util.ReportLog) {}
func (_ *declarationImpl) execute(_ *evaluationContext)               {}
func (d *declarationImpl) prettyPrint(iw util.IndentedWriter)         { iw.Print(d.name) }
func (d *declarationImpl) String() string                             { return d.name }

/*******************************************************************************
 * type declaration
 ******************************************************************************/

type typeDeclaration struct {
    declarationImpl
    tdef typedef
}

func newTypeDeclaration(name string, tdef typedef, line int) *typeDeclaration {
    return &typeDeclaration{
        declarationImpl: declarationImpl{
            baseNode: baseNode{line: line},
            name:     name,
        },
        tdef: tdef,
    }
}

func (d *typeDeclaration) setFilename(filename string) {
    d.declarationImpl.setFilename(filename)
    d.tdef.setFilename(filename)
}

func (d *typeDeclaration) typeCheck(ctx *typeCheckContext, log util.ErrorLog) {
    d.tdef.typeCheck(ctx, log)
}

func (d *typeDeclaration) projectionCheck(ctx *projectionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    d.tdef.projectionCheck(ctx, elog, rlog)
}

func (d *typeDeclaration) prettyPrint(iw util.IndentedWriter) {
    iw.Print("type ")
    d.declarationImpl.prettyPrint(iw)
    iw.Print(" as ")
    d.tdef.prettyPrint(iw)
}

func (d *typeDeclaration) String() string {
    return "type " + d.declarationImpl.String() + " as " + d.tdef.String()
}

/*******************************************************************************
 * type assignment
 ******************************************************************************/

type abstractionTypeAssignment struct {
    declarationImpl
    abstrType typedef
}

func newAbstractionTypeAssignment(name string, abstrType typedef, line int) *abstractionTypeAssignment {
    return &abstractionTypeAssignment {
        declarationImpl: declarationImpl{
            baseNode: baseNode{line: line},
            name:     name,
        },
        abstrType: abstrType,
    }
}

func (d *abstractionTypeAssignment) setFilename(filename string) {
    d.declarationImpl.setFilename(filename)
    d.abstrType.setFilename(filename)
}

func (d *abstractionTypeAssignment) typeCheck(ctx *typeCheckContext, log util.ErrorLog) {
    d.abstrType.typeCheck(ctx, log)
}

func (d *abstractionTypeAssignment) projectionCheck(ctx *projectionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    d.abstrType.projectionCheck(ctx, elog, rlog)
}

func (d *abstractionTypeAssignment) expressionCheck(ctx *expressionCheckContext, log util.ErrorLog) {
    // If the assigned type is an abstraction type, ensure a process exists with that name.
    if la, ok := d.abstrType.getType().(*abstractionType); ok && la != nil {
        if ctx.getAbstraction(d.name) == nil {
            d.reportErrorf(log, "name %q associates a local type but does not associate a process.", d.name)
        }
    }
}

func (d *abstractionTypeAssignment) prettyPrint(iw util.IndentedWriter) {
    d.declarationImpl.prettyPrint(iw)
    iw.Print(": ")
    d.abstrType.prettyPrint(iw)
}

func (d *abstractionTypeAssignment) String() string {
    return d.declarationImpl.String() + ": " + d.abstrType.String()
}

/*******************************************************************************
 * abstraction declaration
 ******************************************************************************/

type abstractionDeclaration struct {
    declarationImpl
    abstr    *abstraction
    locAbstr *abstractionType
}

func newAbstractionDeclaration(name string, abstr *abstraction, line int) *abstractionDeclaration {
    return &abstractionDeclaration{
        declarationImpl: declarationImpl{
            baseNode: baseNode{line: line},
            name:     name,
        },
        abstr:    abstr,
        locAbstr: nil,
    }
}

func (ad *abstractionDeclaration) setFilename(filename string) {
    // Keep consistent with other declarations
    ad.declarationImpl.setFilename(filename)
    ad.abstr.setFilename(filename)
}

func (ad *abstractionDeclaration) typeCheck(ctx *typeCheckContext, log util.ErrorLog) {
    ad.abstr.typeCheck(ctx, log)
}

func (ad *abstractionDeclaration) projectionCheck(ctx *projectionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    ad.abstr.projectionCheck(ctx, elog, rlog)
}

func (ad *abstractionDeclaration) expressionCheck(ctx *expressionCheckContext, log util.ErrorLog) {
    if session := ctx.getSession(ad.name); session == nil {
        ad.reportErrorf(log, "process declaration %q associates no local type.", ad.name)
    } else {
        locAbstr, ok := session.getType().(*abstractionType)
        if ok  == false {
            ad.reportErrorf(log, "expecting local abstraction type; instead found type %v.", session.String())
        } else {
            ad.locAbstr = locAbstr
        }
    }
    locAbstr := ad.abstr.expressionCheck(ctx, log).(*abstractionType)
    if ad.locAbstr != nil {
        if locAbstr.subtypeOf(ad.locAbstr) == false {
            locAbstr2 := ad.locAbstr.substitute(locAbstr)
            stream1 := util.NewStream().Inc().Inc()
            stream2 := util.NewStream().Inc().Inc()
            locAbstr2.prettyPrint(stream1)
            locAbstr.prettyPrint(stream2)
            ad.reportErrorf(log, "abstraction definition %q: expecting type:\n%v\n\tbut found:\n%v.", ad.name, stream1.String(), stream2.String())
        }
    }
}

func (ad *abstractionDeclaration) sessionCheck(ctx *sessionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    ad.abstr.sessionCheck(ctx, elog, rlog)
}


func (ad *abstractionDeclaration) execute(ctx *evaluationContext) {
    value := ad.abstr.evaluate(ctx).(*runtimeAbstraction)
    ctx.add()
    value.spawn(nil, nil, ctx)
}

func (ad *abstractionDeclaration) prettyPrint(iw util.IndentedWriter) {
    iw.Print("val ")
    ad.declarationImpl.prettyPrint(iw)
    iw.Print(" as ")
    ad.abstr.prettyPrint(iw)
}

func (ad *abstractionDeclaration) String() string {
    return "val " + ad.declarationImpl.String() + " as " + ad.abstr.String()
}

/*******************************************************************************
 * value declaration
 ******************************************************************************/

type valueDeclaration struct {
    declarationImpl
    value expression
}

func newValueDeclaration(name string, value expression, line int) *valueDeclaration {
    return &valueDeclaration {
        declarationImpl: declarationImpl{
            baseNode: baseNode{line: line},
            name:     name,
        },
        value: value,
    }
}

func (vd *valueDeclaration) setFilename(filename string) {
    vd.declarationImpl.setFilename(filename)
    vd.value.setFilename(filename)
}

func (vd *valueDeclaration) typeCheck(ctx *typeCheckContext, log util.ErrorLog) {
    vd.value.typeCheck(ctx, log)
}

func (vd *valueDeclaration) projectionCheck(ctx *projectionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    vd.value.projectionCheck(ctx, elog, rlog)
}

func (vd *valueDeclaration) expressionCheck(ctx *expressionCheckContext, log util.ErrorLog) {
    vd.value.expressionCheck(ctx, log)
}

func (vd *valueDeclaration) sessionCheck(ctx *sessionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    vd.value.sessionCheck(ctx, elog, rlog)
}

func (vd *valueDeclaration) prettyPrint(iw util.IndentedWriter) {
    iw.Print("val ")
    vd.declarationImpl.prettyPrint(iw)
    iw.Print(" as ")
    vd.value.prettyPrint(iw)
}

func (vd *valueDeclaration) String() string {
    return "val " + vd.declarationImpl.String() + " as " + vd.value.String()
}
