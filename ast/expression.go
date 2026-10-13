package ast

import (
    "fmt"
    "math"
    "strconv"
    "os"
    "sync"

    "sessions/ast/util"
)

type expression interface {
    lineno() int
    setFilename(string)
    reportErrorf(util.ErrorLog, string, ...interface{})
    runtimeErrorf(string, ...interface{}) error

    getType() typedef
    typeCheck(*typeCheckContext, util.ErrorLog)
    expressionCheck(*expressionCheckContext, util.ErrorLog) typedef
    projectionCheck(*projectionCheckContext, util.ErrorLog, util.ReportLog)
    sessionCheck(*sessionCheckContext, util.ErrorLog, util.ReportLog)

    evaluate(*evaluationContext) expression
    operation(*evaluationContext, expression, string) expression
    prettyPrint(util.IndentedWriter)
    fmt.Stringer
}

/******************************************************************************
 * sequential expression interface
 *****************************************************************************/

type seqExpr interface {
    expression
    localType(*linearContext) *linearContext
    removeVariable(*expressionCheckContext)
}

/******************************************************************************
 * send expression
 *****************************************************************************/

type sendExpr struct {
    baseNode
    lexpr expression
    rexpr expression
    ptype *participantType
    tdef typedef
}

func newSendExpr(lexpr expression, rexpr expression, line int) *sendExpr {
    return &sendExpr {
        baseNode: baseNode{line: line},
        lexpr: lexpr,
        rexpr: rexpr,
    }
}

func (se *sendExpr) setFilename(filename string) {
    se.baseNode.setFilename(filename)
    se.lexpr.setFilename(filename)
    se.rexpr.setFilename(filename)
}

func (se *sendExpr) getType() typedef { return se.lexpr.getType() }

func (se *sendExpr) typeCheck(ctx *typeCheckContext, log util.ErrorLog) {
    se.lexpr.typeCheck(ctx, log)
    se.rexpr.typeCheck(ctx, log)
}

func (se *sendExpr) projectionCheck(ctx *projectionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    se.lexpr.projectionCheck(ctx, elog, rlog)
    se.rexpr.projectionCheck(ctx, elog, rlog)
}

func (se *sendExpr) expressionCheck(ctx *expressionCheckContext, log util.ErrorLog) typedef {
    // syntactically lexpr is always of participant type.
    se.ptype = se.lexpr.expressionCheck(ctx, log).(*participantType)

    if tdef := se.rexpr.expressionCheck(ctx, log); tdef != nil {
        se.tdef = tdef
    } else {
        se.tdef = newNothingType()
    }

    return se.ptype
}

func (se *sendExpr) sessionCheck(ctx *sessionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    se.lexpr.sessionCheck(ctx, elog, rlog)
    se.rexpr.sessionCheck(ctx, elog, rlog)
}

func (se *sendExpr) evaluate(ctx *evaluationContext) expression {
    v1 := se.lexpr.evaluate(ctx)
    v2 := se.rexpr.evaluate(ctx)
    v1.operation(ctx, v2, "send")
    return v1
}

func (_ *sendExpr) operation(_ *evaluationContext, _ expression, _ string) expression { return nil }

func (se *sendExpr) localType(lin *linearContext) *linearContext {
    lin = lin.newSendLocal(se.ptype, se.tdef, se.line)
    if expr, ok := se.lexpr.(seqExpr); ok {
        lin = expr.localType(lin)
    }
    return lin
}

func (se *sendExpr) removeVariable(ctx *expressionCheckContext) {
    if expr, ok := se.rexpr.(seqExpr); ok {
        expr.removeVariable(ctx)
    }
}

func (se *sendExpr) prettyPrint(iw util.IndentedWriter) {
    se.lexpr.prettyPrint(iw)
    iw.Print(" <- (")
    se.rexpr.prettyPrint(iw)
    iw.Print(")")
}

func (se *sendExpr) String() string {
    return se.lexpr.String() + " <- (" + se.rexpr.String() + ")"
}

/******************************************************************************
 * receive expression
 *****************************************************************************/

type receiveExpr struct {
    baseNode
    variable *variableExpr
    expr expression
    ptype *participantType
    tdef typedef
}

func newReceiveExpr(variable *variableExpr, expr expression, line int) *receiveExpr {
    return &receiveExpr{
        baseNode: baseNode{line: line},
        variable: variable,
        expr: expr,
    }
}

func (re *receiveExpr) setFilename(filename string) {
    re.baseNode.setFilename(filename)
    re.variable.setFilename(filename)
    re.expr.setFilename(filename)
}

func (re *receiveExpr) getType() typedef { return re.expr.getType() }

func (re *receiveExpr) typeCheck(ctx *typeCheckContext, log util.ErrorLog) {
    re.variable.typeCheck(ctx, log)
    re.expr.typeCheck(ctx, log)
}

func (re *receiveExpr) projectionCheck(ctx *projectionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    re.variable.projectionCheck(ctx, elog, rlog)
    re.expr.projectionCheck(ctx, elog, rlog)
}

func (re *receiveExpr) expressionCheck(ctx *expressionCheckContext, log util.ErrorLog) typedef {
    // syntactically expr is always of participant type.
    re.ptype = re.expr.expressionCheck(ctx, log).(*participantType)
    re.tdef = re.variable.addVariable(ctx, log)
    return re.ptype
}

func (re *receiveExpr) sessionCheck(ctx *sessionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    re.variable.sessionCheck(ctx, elog, rlog)
    re.expr.sessionCheck(ctx, elog, rlog)
}

func (re *receiveExpr) evaluate(ctx *evaluationContext) expression {
    v1 := re.expr.evaluate(ctx)
    v2 := v1.operation(ctx, nil, "receive")
    // in the case where the channel/broker is closed, receive operation will return nil
    if v2 == nil {
        v2 = re.tdef.defaultValue()
    }
    ctx.addValue(re.variable, v2)
    return v1
}

func (_ *receiveExpr) operation(_ *evaluationContext, _ expression, _ string) expression { return nil }

func (re *receiveExpr) localType(lin *linearContext) *linearContext {
    lin = lin.newReceiveLocal(re.ptype, re.tdef, re.line)
    if expr, ok := re.expr.(seqExpr); ok {
        lin = expr.localType(lin)
    }
    return lin
}

func (re *receiveExpr) removeVariable(ctx *expressionCheckContext) {
    if expr, ok := re.expr.(seqExpr); ok {
        expr.removeVariable(ctx)
    }
    re.variable.removeVariable(ctx)
}

func (re *receiveExpr) prettyPrint(iw util.IndentedWriter) {
    re.variable.prettyPrint(iw)
    iw.Print(" <- (")
    re.expr.prettyPrint(iw)
    iw.Print(")")
}

func (re *receiveExpr) String() string {
    return re.variable.String() + " <- (" + re.expr.String() + ")"
}

/******************************************************************************
 * output expression
 *****************************************************************************/

type outExpr struct {
    baseNode
    ioc expression
    exprs []expression
}

func newOutExpr(ioc expression, exprs []expression, line int) *outExpr {
    return &outExpr {
        baseNode: baseNode{line: line},
        ioc: ioc,
        exprs: exprs,
    }
}

func (o *outExpr) setFilename(filename string) {
    o.baseNode.setFilename(filename)
    if o.ioc != nil {
        o.ioc.setFilename(filename)
    }
    for _, expr := range o.exprs {
        expr.setFilename(filename)
    }
}

func (o *outExpr) getType() typedef { return o.exprs[len(o.exprs) - 1].getType() }

func (o *outExpr) typeCheck(ctx *typeCheckContext, log util.ErrorLog) {
    if o.ioc != nil {
        o.ioc.typeCheck(ctx, log)
    }
    for _, expr := range o.exprs {
        expr.typeCheck(ctx, log)
    }
}

func (o *outExpr) projectionCheck(ctx *projectionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    if o.ioc != nil {
        o.ioc.projectionCheck(ctx, elog, rlog)
    }
    for _, expr := range o.exprs {
        expr.projectionCheck(ctx, elog, rlog)
    }
}

func (o *outExpr) expressionCheck(ctx *expressionCheckContext, log util.ErrorLog) typedef {
    errorFlag := false
    if o.ioc != nil {
        tdef := o.ioc.expressionCheck(ctx, log)
        if tdef != nil {
            switch tdef.(type) {
                case *recordType, *ioType:
                default:
                    o.ioc.reportErrorf(
                        log,
                        "expecting io configuration; instead found type %q.",
                        tdef.String(),
                    )
                    errorFlag = true
            }
        } else {
            errorFlag = true
        }
    }
    var tdef typedef
    for _, expr := range o.exprs {
        tdef = expr.expressionCheck(ctx, log)
        if tdef == nil {
            errorFlag = true
        }
    }
    if errorFlag {
        return nil
    }
    return tdef
}

func (o *outExpr) sessionCheck(ctx *sessionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    if o.ioc != nil {
        o.ioc.sessionCheck(ctx, elog, rlog)
    }
    for _, expr := range o.exprs {
        expr.sessionCheck(ctx, elog, rlog)
    }
}

func (o *outExpr) evaluate(ctx *evaluationContext) expression {
    var h handle
    if o.ioc != nil {
        config := o.ioc.evaluate(ctx)
        var ok bool
        h, ok = config.(handle)
        if !ok {
            err := o.runtimeErrorf("unexpected io config value: %q", config)
            fmt.Fprintf(os.Stderr, "%s\n", err.Error())
            h = eHandle
        }
    } else {
        h = stdoutHandle
    }
    var v expression
    for _, expr := range o.exprs {
        v = expr.evaluate(ctx)
        err := h.output(v)
        if err != nil {
            err = o.runtimeErrorf("%s", err)
            fmt.Fprintf(os.Stderr, "%s\n", err)
        }
    }
    
    return v
}

func (_ *outExpr) operation(_ *evaluationContext, _ expression, _ string) expression { return nil }
func (_ *outExpr) localType(lin *linearContext) *linearContext { return lin }
func (_ *outExpr) removeVariable(_ *expressionCheckContext)    {}

func (o *outExpr) prettyPrint(iw util.IndentedWriter) {
    iw.Print("out")
    if o.ioc != nil {
        iw.Print("[")
        o.ioc.prettyPrint(iw)
        iw.Print("]")
    }
    for _, expr := range o.exprs {
        iw.Print(" <- ")
        expr.prettyPrint(iw)
    }
}

func (o *outExpr) String() string {
    s := "out"
    if o.ioc != nil {
        s += "[" + o.ioc.String() + "]"
    }
    for _, expr := range o.exprs {
        s += " <- " + expr.String()
    }
    return s
}

/******************************************************************************
 * let expression
 *****************************************************************************/

type letExpr struct {
    baseNode
    variable *variableExpr
    expr expression
}

func newLetExpr(variable *variableExpr, expr expression, line int) *letExpr {
    return &letExpr {
        baseNode: baseNode{line: line},
        variable: variable,
        expr: expr,
    }
}

func (le *letExpr) setFilename(filename string) {
    le.baseNode.setFilename(filename)
    le.variable.setFilename(filename)
    le.expr.setFilename(filename)
}

func (le *letExpr) getType() typedef { return le.variable.getType() }

func (le *letExpr) typeCheck(ctx *typeCheckContext, log util.ErrorLog) {
    le.variable.typeCheck(ctx, log)
    le.expr.typeCheck(ctx, log)
}

func (le *letExpr) projectionCheck(ctx *projectionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    le.variable.projectionCheck(ctx, elog, rlog)
    le.expr.projectionCheck(ctx, elog, rlog)
}

func (le *letExpr) expressionCheck(ctx *expressionCheckContext, log util.ErrorLog) typedef {
    tdef := le.expr.expressionCheck(ctx, log)
    vtdef := le.variable.setType(tdef)
    if tdef != nil {
        if !tdef.subtypeOf(vtdef) {
            le.reportErrorf(log, "expecting type %q; instead found type %q.", vtdef.String(), tdef.String())
        }
        le.variable.addVariable(ctx, log)
    }
    return vtdef
}

func (le *letExpr) sessionCheck(ctx *sessionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    le.variable.sessionCheck(ctx, elog, rlog)
    le.expr.sessionCheck(ctx, elog, rlog)
}

func (le *letExpr) evaluate(ctx *evaluationContext) expression {
    v := le.expr.evaluate(ctx)
    ctx.addValue(le.variable, v)
    return v
}

func (_ *letExpr) operation(_ *evaluationContext, _ expression, _ string) expression { return nil }
func (_ *letExpr) localType(lin *linearContext) *linearContext { return lin }
func (le *letExpr) removeVariable(ctx *expressionCheckContext) { le.variable.removeVariable(ctx) }

func (le *letExpr) prettyPrint(iw util.IndentedWriter) {
    //le.variable.prettyPrint(iw)
    //Minor hack!
    iw.Print("let " + le.variable.id + " as ")
    le.expr.prettyPrint(iw)
}

func (le *letExpr) String() string {
    //Minor hack!
    return "let " + le.variable.id + " as " + le.expr.String()
}

/******************************************************************************
 * input expression
 *****************************************************************************/

type inpExpr struct {
    baseNode
    ioc         expression
    variable    *variableExpr
}

func newInpExpr(ioc expression, variable *variableExpr, line int) *inpExpr {
    return &inpExpr {
        baseNode: baseNode{line: line},
        ioc: ioc,
        variable: variable,
    }
}

func (ie *inpExpr) setFilename(filename string) {
    ie.baseNode.setFilename(filename)
    if ie.ioc != nil {
        ie.ioc.setFilename(filename)
    }
    ie.variable.setFilename(filename)
}

func (ie *inpExpr) getType() typedef { return ie.variable.getType() }

func (ie *inpExpr) typeCheck(ctx *typeCheckContext, log util.ErrorLog) {
    if ie.ioc != nil {
        ie.ioc.typeCheck(ctx, log)
    }
    ie.variable.typeCheck(ctx, log)
}

func (ie *inpExpr) projectionCheck(ctx *projectionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    if ie.ioc != nil {
        ie.ioc.projectionCheck(ctx, elog, rlog)
    }
    ie.variable.projectionCheck(ctx, elog, rlog)
}

func (ie *inpExpr) expressionCheck(ctx *expressionCheckContext, log util.ErrorLog) typedef {
    errorFlag := false
    if ie.ioc != nil {
        tdef := ie.ioc.expressionCheck(ctx, log)
        if tdef != nil {
            switch tdef.(type) {
                case *recordType, *ioType:
                default:
                    ie.ioc.reportErrorf(
                        log,
                        "expecting io configuration; instead found type %q.",
                        tdef.String(),
                    )
                    errorFlag = true
            }
        } else {
            errorFlag = true
        }
    }
    tdef := ie.variable.addVariable(ctx, log)
    if tdef == nil {
        return nil
    }

    _, ok := tdef.(*ioType)
    if !ok && !valueType(tdef) {
        ie.reportErrorf(log, "expecting primitive or io type; instead found type %q.", ie.variable.tdef.String())
        errorFlag = true
    }

    if errorFlag {
        return nil
    }
    return tdef
}

func (ie *inpExpr) sessionCheck(ctx *sessionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    if ie.ioc != nil {
        ie.ioc.sessionCheck(ctx, elog, rlog)
    }
    ie.variable.sessionCheck(ctx, elog, rlog)
}

func (ie *inpExpr) evaluate(ctx *evaluationContext) expression {
    var h handle
    if ie.ioc != nil {
        config := ie.ioc.evaluate(ctx)
        var ok bool
        h, ok = config.(handle)
        if !ok {
            err := ie.runtimeErrorf("unexpected io config value: %q", config)
            fmt.Fprintf(os.Stderr, "%s\n", err.Error())
            h = eHandle
        }
    } else {
        h = stdinHandle
    }
    v, err := h.input(ie.variable.tdef)
    if err != nil {
        err = ie.runtimeErrorf("%s", err.Error())
        fmt.Fprintf(os.Stderr, "%s\n", err)
    }
    ctx.addValue(ie.variable, v)
    return v
}

func (_ *inpExpr) operation(_ *evaluationContext, _ expression, _ string) expression { return nil }
func (_ *inpExpr) localType(lin *linearContext) *linearContext { return lin }
func (ie *inpExpr) removeVariable(ctx *expressionCheckContext) { ie.variable.removeVariable(ctx) }

func (ie *inpExpr) prettyPrint(iw util.IndentedWriter) {
    ie.variable.prettyPrint(iw)
    iw.Print(" <- inp")
    if ie.ioc != nil {
        iw.Print("[")
        ie.ioc.prettyPrint(iw)
        iw.Print("]")
    }
}

func (ie *inpExpr) String() string {
    s := ie.variable.String() + " <- inp"
    if ie.ioc != nil {
        s += "[" + ie.ioc.String() + "]"
    }
    return s
}

/******************************************************************************
 * close expression
 *****************************************************************************/

type closeExpr struct {
    baseNode
    expr expression
}

func newCloseExpr(expr expression, line int) *closeExpr {
    return &closeExpr {
        baseNode: baseNode{line: line},
        expr: expr,
    }
}

func (ce *closeExpr) setFilename(filename string) {
    ce.baseNode.setFilename(filename)
    ce.expr.setFilename(filename)
}

func (ce *closeExpr) getType() typedef { return ce.expr.getType() }

func (ce *closeExpr) typeCheck(ctx *typeCheckContext, log util.ErrorLog) {
    ce.expr.typeCheck(ctx, log)
}

func (ce *closeExpr) projectionCheck(ctx *projectionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    ce.expr.projectionCheck(ctx, elog, rlog)
}

func (ce *closeExpr) expressionCheck(ctx *expressionCheckContext, log util.ErrorLog) typedef {
    tdef := ce.expr.expressionCheck(ctx, log)
    if tdef != nil {
        switch tdef.getType().(type) {
            case *reqChannelType, *recChannelType:
            default:
                ce.reportErrorf(log, "expecting channel type; instead found type %q.", tdef.String())
                return nil
        }
    }
    return tdef
}

func (ce *closeExpr) sessionCheck(ctx *sessionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    ce.expr.sessionCheck(ctx, elog, rlog)
}

func (ce *closeExpr) evaluate(ctx *evaluationContext) expression {
    v := ce.expr.evaluate(ctx)
    br := v.(sharedChannel)
    br.close()
    return br
}

func (_ *closeExpr) operation(_ *evaluationContext, _ expression, _ string) expression { return nil }
func (_ *closeExpr) localType(lin *linearContext) *linearContext { return lin }
func (_ *closeExpr) removeVariable(_ *expressionCheckContext) { }

func (ce *closeExpr) prettyPrint(iw util.IndentedWriter) {
    ce.expr.prettyPrint(iw)
    iw.Print(" <- .close")
}

func (ce *closeExpr) String() string {
    return ce.expr.String() + " <- .close "
}

/******************************************************************************
* participant expression
******************************************************************************/

type participantExpr struct {
    baseNode
    id string
}

func newParticipantExpr(id string, line int) *participantExpr {
    return &participantExpr {
        baseNode: baseNode{line: line},
        id: id,
    }
}

func (p *participantExpr) pType() *participantType {
    return p.getType().(*participantType)
}

func (p *participantExpr) getType() typedef {
    return newParticipantType(p.id, p.line)
}

func (_ *participantExpr) typeCheck(_ *typeCheckContext, _ util.ErrorLog) {}
func (_ *participantExpr) projectionCheck(_ *projectionCheckContext, _ util.ErrorLog, _ util.ReportLog) {}
func (p *participantExpr) expressionCheck(ctx *expressionCheckContext, log util.ErrorLog) typedef { 
    if ctx.getParticipant(p) == nil {
        p.reportErrorf(log, "participant %q is undefined.", p.id)
    }
    return p.getType()
}

func (_ *participantExpr) sessionCheck(_ *sessionCheckContext, _ util.ErrorLog, _ util.ReportLog) {}


func (p *participantExpr) evaluate(ctx *evaluationContext) expression {
    return p
}

func (p *participantExpr) operation(ctx *evaluationContext, value expression, operator string) expression {
    switch operator {
        case "send":
            ch := ctx.getSendParticipantChannel(p)
            //ch <- value
            _ = ch.Send(value)
        case "receive":
            ch := ctx.getReceiveParticipantChannel(p)
            //return <- ch
            var v expression
            v, ok := ch.Receive()
            if !ok {
                // !ok means the channel/broker is closed -> return nil and let the receiveProcess call the defaultValue operation
                return nil
            }
            return v
    }
    return p
}

func (p *participantExpr) prettyPrint(iw util.IndentedWriter) { iw.Print(p.String()) }
func (p *participantExpr) String() string                     { return p.id }

/******************************************************************************
 * variable expression
 ******************************************************************************/

type variableExpr struct {
    baseNode
    id string
    tdef typedef
}

func newVariableExpr(id string, tdef typedef, line int) *variableExpr {
    return &variableExpr {
        baseNode: baseNode{line: line},
        id: id,
        tdef: tdef,
    }
}

func (ve *variableExpr) setFilename(filename string) {
    ve.baseNode.setFilename(filename)
    if ve.tdef != nil {
        ve.tdef.setFilename(filename)
    }
}

func (ve *variableExpr) getType() typedef { return ve.tdef }

func (ve *variableExpr) setType(tdef typedef) typedef {
    if ve.tdef == nil {
        ve.tdef = tdef
    }
    return ve.tdef
}

func (ve *variableExpr) typeCheck(ctx *typeCheckContext, log util.ErrorLog) {
    if ve.tdef != nil {
        ve.tdef.typeCheck(ctx, log)
    }
}

func (ve *variableExpr) projectionCheck(ctx *projectionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    if ve.tdef != nil {
        ve.tdef.projectionCheck(ctx, elog, rlog)
    }
}

func (ve *variableExpr) addVariable(ctx *expressionCheckContext, log util.ErrorLog) typedef {
    tdef := ve.tdef.getType()
    if !ctx.addName(ve.id, tdef) {
        ve.reportErrorf(log, "duplicate definition of name: %v.", ve.id)
    }

    return tdef
}

func (ve *variableExpr) removeVariable(ctx *expressionCheckContext) {
    if ve.tdef != nil {
        ctx.removeName(ve.id)
    }
}

func (ve *variableExpr) expressionCheck(ctx *expressionCheckContext, log util.ErrorLog) typedef {
    if vtype := ctx.getVariableType(ve.id); vtype != nil {
        return vtype.getType()
    }
    ve.reportErrorf(log, "name %v is undefined.", ve.id)
    return nil
}

func (_ *variableExpr) sessionCheck(_ *sessionCheckContext, _ util.ErrorLog, _ util.ReportLog) {}
func (ve *variableExpr) evaluate(ctx *evaluationContext) expression { return ctx.getValue(ve) }
func (_ *variableExpr) operation(_ *evaluationContext, _ expression, _ string) expression { return nil }
func (ve *variableExpr) prettyPrint(iw util.IndentedWriter)         { iw.Print(ve.String()) }

func (ve *variableExpr) String() string {
    s := ve.id
    if ve.tdef != nil {
        s += " " + ve.tdef.String()
        return s
    }
    return s
}

/******************************************************************************
* evaluation operators
******************************************************************************/

func operationI(a int, b int, operator string, line int) expression {
    var i int = 0
    var c bool = true
    var op bool = false

    switch operator {
        case "+":
            i = a + b
        case "-":
            i = a - b
        case "*":
            i = a * b
        case "/":
            if b == 0 {
                return newNothing("division by zero", line)
            }
            i = a / b
        case "%":
            i = a % b
        case "^":
            i = 1
            for j := 0; j < b; j ++ {
                i *= a
            }
        case "<":
            c = a < b
            op = true
        case "<=":
            c = a <= b
            op = true
        case ">":
            c = a > b
            op = true
        case ">=":
            c = a >= b
            op = true
        case "==":
            c = a == b
            op = true
        case "!=":
            c = a != b
            op = true
    }

    if op == false {
        return newIntExpr(strconv.Itoa(i), line)
    }
    if c == true {
        return newTrueExpr(line)
    }
        return newFalseExpr(line)
}

func operationF(a float64, b float64, operator string, line int) expression {
    var i float64 = 0
    var c bool = true
    var op bool = false
    switch operator {
        case "+":
            i = a + b
        case "-":
            i = a - b
        case "*":
            i = a * b
        case "/":
            if b == 0 {
                return newNothing("division by zero", line)
            }
            i = a / b
        case "^":
            i = math.Pow(a, b)
        case "<":
            c = a < b
            op = true
        case "<=":
            c = a <= b
            op = true
        case ">":
            c = a > b
            op = true
        case ">=":
            c = a >= b
            op = true
        case "==":
            c = a == b
            op = true
        case "!=":
            c = a != b
            op = true
    }

    if op == false {
        return newFloatExpr(fmt.Sprintf("%v", i), line)
    }
    if c == true {
        return newTrueExpr(line)
    }
    return newFalseExpr(line)
}

func operationB(v1 expression, v2 expression, operator string, line int) expression {
    v := true
    eval :=
        func (value expression) bool {
            switch value.(type) {
                case *trueExpr: return true
                case *falseExpr: return false
            }
            return false
        }
    a := eval(v1)
    b := eval(v2)
    switch operator {
        case "==":        
            v = a == b
        case "!=":
            v = a != b
        case "&&":
            v = a && b
        case "||":
            v = a || b
    }
    if v == true {
        return newTrueExpr(line)
    }
    return newFalseExpr(line)
}

func operationS(a string, b string, operator string, line int) expression {
    v := true
    s := ""
    boolOp := true
    switch operator {
        case "==":
            v = a == b
        case "!=":
            v = a != b
        case "<":
            v = a < b
        case "<=":
            v = a <= b
        case ">":
            v = a > b
        case ">=":
            v = a >= b
        case "+":
            s = a + b
            boolOp = false
    }
    if boolOp == false {
        return newStringExpr(s, line)
    }
    if v == true {
        return newTrueExpr(line)
    }
    return newFalseExpr(line)
}

/******************************************************************************
* integer expression
******************************************************************************/

type intExpr struct {
    baseNode
    integer int
}

func newIntExpr(integer string, line int) *intExpr {
    n, _ := strconv.Atoi(integer)
    return &intExpr {
        baseNode: baseNode{line: line},
        integer: n,
    }
}

func (i *intExpr) getType() typedef { return newIntType(i.line) }
func (_ *intExpr) typeCheck(_ *typeCheckContext, _ util.ErrorLog) { }
func (_ *intExpr) projectionCheck(_ *projectionCheckContext, _ util.ErrorLog, _ util.ReportLog) {}
func (i *intExpr) expressionCheck(_ *expressionCheckContext, _ util.ErrorLog) typedef {
    return i.getType()
}

func (_ *intExpr) sessionCheck(_ *sessionCheckContext, _ util.ErrorLog, _ util.ReportLog) {}
func (i *intExpr) evaluate(_ *evaluationContext) expression { return i }

func (i *intExpr) operation(_ *evaluationContext, value expression, operator string) expression {
    if value == nil {
        switch operator {
            case "+":
                return &intExpr{baseNode: baseNode{line: i.line}, integer: i.integer}
            case "-":
                return &intExpr{baseNode: baseNode{line: i.line}, integer: -i.integer}
        }
    }
    switch v := value.(type) {
        case *intExpr:
            return operationI(i.integer, v.integer, operator, i.line)
        case *floatExpr:
            return operationF(float64(i.integer), v.floating, operator, i.line)
        case *nothing:
            return value
    }
    return newNothingf(i.line, "unknown integer operator: %q", operator)
}

func (i *intExpr) prettyPrint(iw util.IndentedWriter) { iw.Print(i.String()) }
func (i *intExpr) String() string                     { return strconv.Itoa(i.integer) }

/******************************************************************************
* float expression
******************************************************************************/

type floatExpr struct {
    baseNode
    floating float64
}

func newFloatExpr(floating string, line int) *floatExpr {
    fl, _ := strconv.ParseFloat(floating, 64)
    return &floatExpr {
        baseNode: baseNode{line: line},
        floating: fl,
    }
}

func (f *floatExpr) getType() typedef { return newFloatType(f.line) }
func (_ *floatExpr) typeCheck(_ *typeCheckContext, _ util.ErrorLog) { }
func (_ *floatExpr) projectionCheck(_ *projectionCheckContext, _ util.ErrorLog, _ util.ReportLog) {}
func (f *floatExpr) expressionCheck(_ *expressionCheckContext, _ util.ErrorLog) typedef {
    return f.getType()
}

func (_ *floatExpr) sessionCheck(_ *sessionCheckContext, _ util.ErrorLog, _ util.ReportLog) {}
func (f *floatExpr) evaluate(_ *evaluationContext) expression { return f }

func (f *floatExpr) operation(_ *evaluationContext, value expression, operator string) expression {
    if value == nil {
        switch operator {
            case "+":
              return &floatExpr{baseNode: baseNode{line: f.line}, floating: f.floating}
            case "-":
              return &floatExpr{baseNode: baseNode{line: f.line}, floating: -f.floating}
        }
    }
    switch v := value.(type) {
        case *intExpr:
            return operationF(f.floating, float64(v.integer), operator, f.line)
        case *floatExpr:
            return operationF(f.floating, v.floating, operator, f.line)
        case *nothing:
            return value
    }
    return newNothingf(f.line, "unknown float operator: %q", operator)
}

func (f *floatExpr) prettyPrint(iw util.IndentedWriter) { iw.Print(f.String()) }
func (f *floatExpr) String() string { return fmt.Sprintf("%f", f.floating) }

/******************************************************************************
* true
*******************************************************************************/

type trueExpr struct {
    baseNode
}

func newTrueExpr(line int) *trueExpr {
     return &trueExpr{
        baseNode: baseNode{line: line},
    }
}

func (t *trueExpr) getType() typedef { return newBoolType(t.line) }
func (_ *trueExpr) typeCheck(_ *typeCheckContext, _ util.ErrorLog) {}
func (_ *trueExpr) projectionCheck(_ *projectionCheckContext, _ util.ErrorLog, _ util.ReportLog) {}
func (t *trueExpr) expressionCheck(_ *expressionCheckContext, _ util.ErrorLog) typedef {
    return t.getType()
}

func (_ *trueExpr) sessionCheck(_ *sessionCheckContext, _ util.ErrorLog, _ util.ReportLog) {}
func (t *trueExpr) evaluate(_ *evaluationContext) expression { return t }

func (t *trueExpr) operation(_ *evaluationContext, value expression, operator string) expression {
    switch value.(type) {
        case *nothing:
            return value
        default:
            return operationB(t, value, operator, t.line)
    }
}

func (t *trueExpr) prettyPrint(iw util.IndentedWriter)  { iw.Print(t.String()) }
func (_ *trueExpr) String() string                      { return "true" }

/******************************************************************************
* false
*******************************************************************************/

type falseExpr struct {
    baseNode
}

func newFalseExpr(line int) *falseExpr {
    return &falseExpr{
        baseNode: baseNode{line: line}, 
    }
}

func (f *falseExpr) getType() typedef { return newBoolType(f.line) }
func (_ *falseExpr) typeCheck(_ *typeCheckContext, _ util.ErrorLog) { }
func (_ *falseExpr) projectionCheck(_ *projectionCheckContext, _ util.ErrorLog, _ util.ReportLog) {}
func (f *falseExpr) expressionCheck(_ *expressionCheckContext, _ util.ErrorLog) typedef {
    return f.getType()
}

func (_ *falseExpr) sessionCheck(_ *sessionCheckContext, _ util.ErrorLog, _ util.ReportLog) {}
func (f *falseExpr) evaluate(_ *evaluationContext) expression { return f }

func (f *falseExpr) operation(_ *evaluationContext, value expression, operator string) expression {
    switch value.(type) {
        case *nothing:
            return value
        default:
            return operationB(f, value, operator, f.line)
    }
}

func (f *falseExpr) prettyPrint(iw util.IndentedWriter) { iw.Print(f.String()) }
func (_ *falseExpr) String() string                     { return "false" }

/******************************************************************************
* string
*******************************************************************************/

type stringExpr struct {
    baseNode
    stringVal string
}

func newStringExpr(stringVal string, line int) *stringExpr {
    return &stringExpr {
        baseNode: baseNode{line: line},
        stringVal: stringVal,
    }
}

func (s *stringExpr) getType() typedef { return newStringType(s.line) }
func (_ *stringExpr) typeCheck(_ *typeCheckContext, _ util.ErrorLog) {}
func (_ *stringExpr) projectionCheck(_ *projectionCheckContext, _ util.ErrorLog, _ util.ReportLog) {}
func (s *stringExpr) expressionCheck(_ *expressionCheckContext, _ util.ErrorLog) typedef {
    return s.getType()
}

func (_ *stringExpr) sessionCheck(_ *sessionCheckContext, _ util.ErrorLog, _ util.ReportLog) {}
func (s *stringExpr) evaluate(_ *evaluationContext) expression { return s }

func (s *stringExpr) operation(_ *evaluationContext, value expression, operator string) expression {
    switch v := value.(type) {
        case *stringExpr:
            return operationS(s.stringVal, v.stringVal, operator, s.line)
        case *intExpr:
            runes := []rune(s.stringVal)
            pos := v.integer
            if pos == 0 && len(runes) == 0 {
                return s
            }
            if pos < 0 || pos >= len(runes) {
                return newNothingf(v.line, "index out of range (index=%d)", pos)
            }
            return newStringExpr(string(runes[pos]), s.line)
        case *nothing:
            return value
    }
    return newNothingf(s.line, "unknown operation: %q", operator)
}

func (s *stringExpr) prettyPrint(iw util.IndentedWriter) { iw.Print(s.String()) }
func (s *stringExpr) String() string { return "\"" + s.stringVal + "\"" }

/******************************************************************************
* nothing
******************************************************************************/

type nothing struct {
    baseNode
    msg string
}

func newNothing(msg string, line int) *nothing {
    return &nothing {
        baseNode:   baseNode{line: line},
        msg:        msg,
    }
}

func newNothingf(line int, format string, a ...interface{}) *nothing {
    return newNothing(fmt.Sprintf(format, a...), line)
}

func (_ *nothing) getType() typedef { return newNothingType() }
func (_ *nothing) typeCheck(_ *typeCheckContext, _ util.ErrorLog) {}
func (_ *nothing) projectionCheck(_ *projectionCheckContext, _ util.ErrorLog, _ util.ReportLog) {}
func (n *nothing) expressionCheck(_ *expressionCheckContext, _ util.ErrorLog) typedef {
    return n.getType()
}

func (_ *nothing) sessionCheck(_ *sessionCheckContext, _ util.ErrorLog, _ util.ReportLog) {}
func (n *nothing) evaluate(_ *evaluationContext) expression     { return n }
func (n *nothing) operation(_ *evaluationContext, _ expression, _ string) expression  { return n }
func (n *nothing) prettyPrint(iw util.IndentedWriter)           { iw.Print(n.String()) }

func (n *nothing) String() string {
    msg := ""
    if n.msg != "" {
        msg = fmt.Sprintf(" {%v, line: %v}", n.msg, n.line)
    }
    return "nothing" + msg
}

/******************************************************************************
* binary expression (super struct)
******************************************************************************/

type binaryExpr struct {
    baseNode
    lexpr expression
    rexpr expression
    operator string
}

func newBinaryExpr(operand1 expression, operand2 expression, operator string, line int) *binaryExpr {
    return &binaryExpr {
        baseNode: baseNode{line: line},
        lexpr: operand1,
        rexpr: operand2,
        operator: operator,
    }
}

func (be *binaryExpr) setFilename(filename string) {
    be.baseNode.setFilename(filename)
    be.lexpr.setFilename(filename)
    be.rexpr.setFilename(filename)
}

func (be *binaryExpr) typeCheck(ctx *typeCheckContext, log util.ErrorLog) {
    be.lexpr.typeCheck(ctx, log)
    be.rexpr.typeCheck(ctx, log)
}

func (be *binaryExpr) projectionCheck(ctx *projectionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    be.lexpr.projectionCheck(ctx, elog, rlog)
    be.rexpr.projectionCheck(ctx, elog, rlog)
}

func (be *binaryExpr) sessionCheck(ctx *sessionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    be.lexpr.sessionCheck(ctx, elog, rlog)
    be.rexpr.sessionCheck(ctx, elog, rlog)
}

func (be *binaryExpr) evaluate(ctx *evaluationContext) expression {
    v1 := be.lexpr.evaluate(ctx)
    v2 := be.rexpr.evaluate(ctx)
    return v1.operation(ctx, v2, be.operator)
}

func (_ *binaryExpr) operation(_ *evaluationContext, _ expression, _ string) expression { return nil }
func (be *binaryExpr) prettyPrint(iw util.IndentedWriter) {
    be.lexpr.prettyPrint(iw)
    iw.Print(" " + be.operator + " ")
    be.rexpr.prettyPrint(iw)
}

func (be *binaryExpr) String() string {
    return be.lexpr.String() + " " + be.operator + " " + be.rexpr.String()
}

/******************************************************************************
* logical expression
******************************************************************************/

type logicalExpr struct {
    *binaryExpr
}

func newLogicalExpr(lexpr expression, rexpr expression, operator string, line int) *logicalExpr {
    return &logicalExpr {
        binaryExpr: newBinaryExpr(lexpr, rexpr, operator, line),
    }
}

func (le *logicalExpr) getType() typedef { return newBoolType(le.line) }

func (le *logicalExpr) expressionCheck(ctx *expressionCheckContext, log util.ErrorLog) typedef {
    tdef1 := le.lexpr.expressionCheck(ctx, log)
    tdef2 := le.rexpr.expressionCheck(ctx, log)
    ok1, ok2 := true, true

    if tdef1 != nil && !tdef1.subtypeOf(newBoolType(le.line)) {
        ok1 = false
        le.reportErrorf(
            log,
            "operand %q (type=%s) is not of boolean type.",
            le.lexpr.String(), tdef1.String(),
        )
    }

    if tdef2 != nil && !tdef2.subtypeOf(newBoolType(le.line)) {
        ok2 = false
        le.reportErrorf(
            log, "operand %q (type=%s) is not of boolean type.",
            le.rexpr.String(), tdef2.String(),
        )
    }

    if tdef1 == nil || tdef2 == nil || !ok1 || !ok2 {
        return nil
    }

    return newBoolType(le.line)
}

// implement short circuit
func (le *logicalExpr) evaluate(ctx *evaluationContext) expression {
    switch v := le.lexpr.evaluate(ctx).(type) {
        case *trueExpr:
            if le.operator == "||" {
                return v
            } 
        case *falseExpr:
            if le.operator == "&&" {
                return v
            }
        default:
            return newNothingf(le.line, "non-boolean value at runtime: %q", v.String())
    }
    return le.rexpr.evaluate(ctx)
}

/******************************************************************************
* eqaulity Expr
******************************************************************************/

type equalityExpr struct {
    *binaryExpr
}

func newEqualityExpr(lexpr expression, rexpr expression, operator string, line int) *equalityExpr {
    return &equalityExpr {
        binaryExpr: newBinaryExpr(lexpr, rexpr, operator, line),
    }
}

func (ee *equalityExpr) getType() typedef { return newBoolType(ee.line) }
func (ee *equalityExpr) expressionCheck(ctx *expressionCheckContext, log util.ErrorLog) typedef {
    tdef1 := ee.lexpr.expressionCheck(ctx, log)
    tdef2 := ee.rexpr.expressionCheck(ctx, log)
    ok := true

    if tdef1 != nil && !eq.contains(tdef1) {
        ok = false
        ee.reportErrorf(
            log,
            "operand %q (type=%s) is not comparable.",
            ee.lexpr.String(), tdef1.String(),
        )
    }

    if tdef2 != nil && !eq.contains(tdef2) {
        ok = false
        ee.reportErrorf(
            log,
            "operand %q (type=%s) is not comparable.",
            ee.rexpr.String(), tdef2.String(),
        )
    }

    if tdef1 == nil || tdef2 == nil || !ok {
        return nil
    }

    if !tdef1.subtypeOf(tdef2) && !tdef2.subtypeOf(tdef1) {
        ee.reportErrorf(
            log, 
            "operands %q (type=%s), and %q (type=%s) cannot be compared using operator %q.",
            ee.lexpr.String(), tdef1.String(), ee.rexpr.String(), tdef2.String(), ee.operator,
        )
        return nil
    }
    return newBoolType(ee.line)
}

/******************************************************************************
* relExpr
******************************************************************************/

type relationalExpr struct {
    *binaryExpr
}

func newRelationalExpr(lexpr expression, rexpr expression, operator string, line int) *relationalExpr {
    return &relationalExpr {
        binaryExpr: newBinaryExpr(lexpr, rexpr, operator, line),
    }
}

func (re *relationalExpr) getType() typedef { return newBoolType(re.line) }
func (re *relationalExpr) expressionCheck(ctx *expressionCheckContext, log util.ErrorLog) typedef {
    tdef1 := re.lexpr.expressionCheck(ctx, log)
    tdef2 := re.rexpr.expressionCheck(ctx, log)
    ok := true

    if tdef1 != nil && !ord.contains(tdef1) {
        ok = false
        re.reportErrorf(log, "operand %q (type=%s) cannot be compared.", re.lexpr.String(), tdef1.String())
    }
    if tdef2 != nil && !ord.contains(tdef2) {
        ok = false
        re.reportErrorf(log, "operand %q (type=%s) cannot be compared.", re.rexpr.String(), tdef2.String())
    }

    if tdef1 == nil || tdef2 == nil || !ok {
        return nil
    }

    if !tdef1.subtypeOf(tdef2) && !tdef2.subtypeOf(tdef1) {
        re.reportErrorf(
            log,
            "operands %q (type=%s) and %q (type=%s) cannot be compared with %q.",
            re.lexpr.String(), tdef1.String(), re.rexpr.String(), tdef2.String(), re.operator,
        )
        return nil
    }

    return newBoolType(re.line)
}


/******************************************************************************
* sum expression
******************************************************************************/

type sumExpr struct {
    *binaryExpr
}

func newSumExpr(lexpr expression, rexpr expression, operator string, line int) *sumExpr {
    return &sumExpr {
        binaryExpr: newBinaryExpr(lexpr, rexpr, operator, line),
    }
}

func (se *sumExpr) getType() typedef {
    tdef1 := se.lexpr.getType()
    tdef2 := se.rexpr.getType()
    if tdef1 == nil || tdef2 == nil {
        return nil
    }
    if tdef1.subtypeOf(tdef2) {
        return tdef2
    } else if tdef2.subtypeOf(tdef1) {
        return tdef1
    }
    return nil
}

func (se *sumExpr) expressionCheck(ctx *expressionCheckContext, log util.ErrorLog) typedef {
    tdef1 := se.lexpr.expressionCheck(ctx, log)
    tdef2 := se.rexpr.expressionCheck(ctx, log)
    ok := true

    if tdef1 != nil {
        if se.operator == "-" {
            if !ord.contains(tdef1) {
                ok = false
                se.reportErrorf(
                    log,
                    "cannot subtract from expression %q (type=%s).",
                    se.lexpr.String(), tdef1.String(),
                )
            }
        } else {
            if !num.contains(tdef1) {
                ok = false
                se.reportErrorf(
                    log,
                    "cannot perform arithmetic on expression %q (type=%s).", 
                    se.lexpr.String(), tdef1.String(),
                )
            }
        }
    }

    if tdef2 != nil {
        if se.operator == "-" {
            if !ord.contains(tdef2) {
                ok = false
                se.reportErrorf(
                    log,
                    "cannot subtract expression %q (type=%s).",
                    se.rexpr.String(), tdef2.String(),
                )
            }
        } else {
            if !num.contains(tdef2) {
                ok = false
                se.reportErrorf(
                    log,
                    "cannot perform arithmetic on expression %q (type=%s).",
                    se.rexpr.String(), tdef2.String(),
                )
            }
        }
    }

    if tdef1 == nil || tdef2 == nil || !ok {
        return nil
    }

    if tdef1.subtypeOf(tdef2) {
        return tdef2
    } else if tdef2.subtypeOf(tdef1) {
        return tdef1
    }

    se.reportErrorf(
        log,
        "cannot use operator %q on operands %q (type=%s) and %q (type=%s).", 
        se.operator, se.lexpr.String(), tdef1.String(), se.rexpr.String(), tdef2.String(),
    )
    return nil
}

/******************************************************************************
* multiplication Expression
******************************************************************************/

type multExpr struct {
    *binaryExpr
}

func newMultExpr(lexpr expression, rexpr expression, operator string, line int) *multExpr {
    return &multExpr {
        binaryExpr: newBinaryExpr(lexpr, rexpr, operator, line),
    }
}

func (me *multExpr) getType() typedef {
    tdef1 := me.lexpr.getType()
    tdef2 := me.rexpr.getType()
    if tdef1 == nil || tdef2 == nil {
        return nil
    }
    if tdef1.subtypeOf(tdef2) {
        return tdef2
    } else if tdef2.subtypeOf(tdef1) {
        return tdef1
    }
    return nil
}

func (me *multExpr) expressionCheck(ctx *expressionCheckContext, log util.ErrorLog) typedef {
    tdef1 := me.lexpr.expressionCheck(ctx, log)
    tdef2 := me.rexpr.expressionCheck(ctx, log)
    ok := true
    if tdef1 != nil {
        if me.operator == "%" {
            if !tdef1.subtypeOf(newIntType(me.line)) {
                ok = false
                me.reportErrorf(
                    log,
                    "invalid operator %q on expression %q (type=%s).",
                    me.operator, me.lexpr.String(), tdef1.String(),
                )
            }
        } else {
            if !ord.contains(tdef1) {
                ok = false
                me.reportErrorf(
                    log,
                    "cannot perform arithmetic on expression %q (type=%s).",
                    me.lexpr.String(), tdef1.String(),
                )
            }
        }
    }
    if tdef2 != nil {
        if me.operator == "%" {
            if !tdef2.subtypeOf(newIntType(me.line)) {
                ok = false
                me.reportErrorf(
                    log,
                    "invalid operator %q on expression %q (type=%s).", 
                    me.operator, me.rexpr.String(), tdef2.String(),
                )
            }
        } else {
            if !ord.contains(tdef2) {
                ok = false
                me.reportErrorf(
                    log,
                    "cannot perform arithmetic on expression %q (type=%s).", 
                    me.rexpr.String(), tdef2.String(),
                )
            }
        }
    }

    if tdef1 == nil || tdef2 == nil || !ok {
        return nil
    }

    if tdef1.subtypeOf(tdef2) {
        return tdef2
    } else if tdef2.subtypeOf(tdef1) {
        return tdef1
    }
    me.reportErrorf(
        log,
        "cannot use operator %q on operands %q (type=%s) and %q (type=%s).",
        me.operator, me.lexpr.String(), tdef1.String(), me.rexpr.String(), tdef2.String(),
    )
    return nil
}

/******************************************************************************
* not expression
******************************************************************************/

type notExpr struct {
    baseNode
    expr     expression
    operator string
}

func newNotExpr(expr expression, operator string, line int) *notExpr {
    return &notExpr{
        baseNode: baseNode{line: line},
        expr:     expr,
        operator: operator,
    }
}

func (ne *notExpr) setFilename(filename string) {
    ne.baseNode.setFilename(filename)
    ne.expr.setFilename(filename)
}

func (ne *notExpr) getType() typedef { return newBoolType(ne.line) }

func (ne *notExpr) typeCheck(ctx *typeCheckContext, log util.ErrorLog) {
    ne.expr.typeCheck(ctx, log)
}

func (ne *notExpr) projectionCheck(ctx *projectionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    ne.expr.projectionCheck(ctx, elog, rlog)
}

func (ne *notExpr) expressionCheck(ctx *expressionCheckContext, log util.ErrorLog) typedef {
    ntype := ne.expr.expressionCheck(ctx, log)
    if ntype == nil {
        return nil
    }

    if !ntype.subtypeOf(newBoolType(ne.line)) {
        ne.reportErrorf(log, "expression %q (type=%s) is not of boolean type.", ne.expr.String(), ntype.String())
        return nil
    }
    return ntype
}

func (ne *notExpr) sessionCheck(ctx *sessionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    ne.expr.sessionCheck(ctx, elog, rlog)
}

func (ne *notExpr) evaluate(ctx *evaluationContext) expression {
    v := ne.expr.evaluate(ctx)
    switch v.(type) {
        case *trueExpr:
            return newFalseExpr(ne.line)
        case *falseExpr:
            return newTrueExpr(ne.line)
        case *nothing:
            return v
        default:
            return newNothingf(ne.line, "non-boolean value at runtime: %q", v.String())
    }
}

func (_ *notExpr) operation(_ *evaluationContext, _ expression, _ string) expression { return nil }

func (ne *notExpr) prettyPrint(iw util.IndentedWriter) {
    iw.Print(ne.operator + " ")
    ne.expr.prettyPrint(iw)
}

func (ne *notExpr) String() string {
    return ne.operator + " " + ne.expr.String()
}

/******************************************************************************
* sign expression
******************************************************************************/

type signExpr struct {
    baseNode
    expr     expression
    operator string
}

func newSignExpr(expr expression, operator string, line int) *signExpr {
    return &signExpr{
        baseNode: baseNode{line: line},
        expr:     expr,
        operator: operator,
    }
}

func (se *signExpr) getType() typedef {
    return se.expr.getType()
}

func (se *signExpr) setFilename(filename string) {
    se.baseNode.setFilename(filename)
    se.expr.setFilename(filename)
}

func (se *signExpr) typeCheck(ctx *typeCheckContext, log util.ErrorLog) {
    se.expr.typeCheck(ctx, log)
}

func (se *signExpr) projectionCheck(ctx *projectionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    se.expr.projectionCheck(ctx, elog, rlog)
}

func (se *signExpr) expressionCheck(ctx *expressionCheckContext, log util.ErrorLog) typedef {
    ntype := se.expr.expressionCheck(ctx, log)
    if ntype != nil && !ord.contains(ntype) {
        se.reportErrorf(
            log,
            "dannot perform unary arithmetic (operator=%q) on expression %q (type=%s).",
            se.operator, se.expr.String(), ntype.String(),
        )
    }

    return ntype
}

func (se *signExpr) sessionCheck(ctx *sessionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    se.expr.sessionCheck(ctx, elog, rlog)
}

func (se *signExpr) evaluate(ctx *evaluationContext) expression {
    v := se.expr.evaluate(ctx)
    switch v.(type) {
        case *intExpr, *floatExpr:
            return v.operation(ctx, nil, se.operator)
        case *nothing:
            return v
    }
    return newNothingf(se.line, "non-numeric value at runtime: %q", v.String())
}

func (_ *signExpr) operation(_ *evaluationContext, _ expression, _ string) expression { return nil }
func (se *signExpr) prettyPrint(iw util.IndentedWriter) {
    iw.Print(se.operator + " ")
    se.expr.prettyPrint(iw)
}

func (se *signExpr) String() string { return se.operator + " " + se.expr.String() }

/******************************************************************************
* label expression
******************************************************************************/

type labelExpr struct {
    baseNode
    label string
    //expr expression
}

func newLabelExpr(label string, line int) *labelExpr {
    return &labelExpr {
        baseNode: baseNode{line: line},
        label: label,
    }
}

func (le *labelExpr) getType() typedef { return newLabelType(le.label, le.line) }
func (_ *labelExpr) typeCheck(_ *typeCheckContext, _ util.ErrorLog) {}
func (_ *labelExpr) projectionCheck(_ *projectionCheckContext, _ util.ErrorLog, _ util.ReportLog) {}
func (le *labelExpr) expressionCheck(_ *expressionCheckContext, _ util.ErrorLog) typedef {
    return le.getType()
}

func (_ *labelExpr) sessionCheck(_ *sessionCheckContext, _ util.ErrorLog, _ util.ReportLog) {}
func (le *labelExpr) evaluate(_ *evaluationContext) expression   { return le }
func (_ *labelExpr) operation(_ *evaluationContext, _ expression, _ string) expression { return nil }
func (le *labelExpr) prettyPrint(iw util.IndentedWriter)         { iw.Print(le.String()) }
func (le *labelExpr) String() string                             { return le.label }

/******************************************************************************
* list expression
******************************************************************************/

type listExpr struct {
    baseNode
    expressions []expression
}

func newListExpr(expressions []expression, line int) *listExpr {
    return &listExpr {
        baseNode: baseNode{line: line},
        expressions: expressions,
    }
}

func (le *listExpr) setFilename(filename string) {
    le.baseNode.setFilename(filename)
    for _, expr := range le.expressions {
        expr.setFilename(filename)
    }
}

func (le *listExpr) getType() typedef {
    var max typedef = newNothingType()
    for _, e := range le.expressions {
        tdef := e.getType()
        if tdef == nil {
            return nil
        }
        ok := true
        max, ok = max.join(tdef)
        if !ok {
            return nil
        }
    }

    return newListType(max, le.line)
}

func (le *listExpr) typeCheck(ctx *typeCheckContext, log util.ErrorLog) {
    for _, expr := range le.expressions {
        expr.typeCheck(ctx, log)
    }
}

func (le *listExpr) projectionCheck(ctx *projectionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    for _, expr := range le.expressions {
        expr.projectionCheck(ctx, elog, rlog)
    }
}

func (le *listExpr) expressionCheck(ctx *expressionCheckContext, log util.ErrorLog) typedef {
    var max typedef = newNothingType()
    errorFlag := false
    for _, e := range le.expressions {
        tdef := e.expressionCheck(ctx, log)
        if tdef == nil {
            errorFlag = true
            continue
        }
        ok := true
        max, ok = max.join(tdef)
        if !ok {
            le.reportErrorf(
                log,
                "list elements do not have the same type: expecting expression %q (type=%s) to be of type %s.",
                e.String(), tdef.String(), max.String(),
            )
            errorFlag = true
        }
    }
    if errorFlag {
        return nil
    }
    return newListType(max, le.line)
}

func (le *listExpr) sessionCheck(ctx *sessionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    for _, expr := range le.expressions {
        expr.sessionCheck(ctx, elog, rlog)
    }
}

func (le *listExpr) evaluate(ctx *evaluationContext) expression {
    values := make([]expression, len(le.expressions))
    for i, expr := range le.expressions {
        values[i] = expr.evaluate(ctx)
    }
    return newListExpr(values, le.line)
}

func (le *listExpr) operation(ctx *evaluationContext, value expression, operator string) expression {
    switch operator {
        case "==", "!=":
            switch v := value.(type) {
                case *listExpr:
                    boolFor :=
                        func(flag bool) expression {
                            if flag {
                                return newTrueExpr(le.line)
                            }
                            return newFalseExpr(le.line)
                        }
                    if len(le.expressions) != len(v.expressions) {
                        return boolFor(operator == "!=")
                    }
                    for i := range le.expressions {
                        r := le.expressions[i].operation(ctx, v.expressions[i], "==")
                        switch r.(type) {
                            case *nothing:
                                return r
                            case *trueExpr:
                                // keep checking
                            case *falseExpr:
                                return boolFor(operator == "!=")
                            default:
                                return newNothing("unexpected comparison behaviour", le.line)
                        }
                    }
                    return boolFor(operator == "==")
                case *nothing:
                    return v
            }
        case "index":
            switch index := value.(type) {
                case *intExpr:
                    //throw out of bounds exception
                    if index.integer < 0 || index.integer >= len(le.expressions) {
                        return newNothingf(le.line, "index (value=%d) out of bounds (size=%d)", index.integer, len(le.expressions))
                    }
                    return le.expressions[index.integer]
                case *nothing:
                    return index
                default:
                    return newNothing("list index must be an integer.", le.line)
            }
        case "+":        
            if valueList, ok := value.(*listExpr); ok {
                list := append(le.expressions, valueList.expressions...)
                return newListExpr(list, le.line)    
            }
            return newNothing("list concatenation requires a list on the right-hand side.", le.line)
        case "tail":
            if len(le.expressions) == 0 {
                return le
            } else {
                return newListExpr(le.expressions[1:len(le.expressions)], le.line)
            }
        case "concatenate":
            return newListExpr(append([]expression{value}, le.expressions...), le.line)
    }
    return nil
}

func (le *listExpr) prettyPrint(iw util.IndentedWriter) {
    iw.Print("[")
    for i, e := range le.expressions {
        if i != 0 {
            iw.Print(", ")
        }
        e.prettyPrint(iw)
    }
    iw.Print("]")
}

func (le *listExpr) String() string {
    list := ""
    for i, e := range le.expressions {
        if i != 0 {
            list += ", "
        }
        list += e.String()
    }
    return "[" + list + "]"
}

/******************************************************************************
* list access expression
******************************************************************************/

type listAccessExpr struct {
    baseNode
    list expression
    index expression
}

func newListAccessExpr(list expression, index expression, line int) *listAccessExpr {
    return &listAccessExpr{
        baseNode: baseNode{line: line},
        list:     list,
        index:    index,
    }
}

func (la *listAccessExpr) setFilename(filename string) {
    la.baseNode.setFilename(filename)
    la.index.setFilename(filename)
    la.list.setFilename(filename)
}

func (le *listAccessExpr) getType() typedef {
    tdef := le.list.getType()
    if tdef == nil {
        return nil
    }
    switch lt := tdef.(type) {
        case *stringType:
            // s[i] has type string (single-char as string)
            return lt
        case *listType:
            // xs[i] has the element type
            return lt.tdef
        case *nothingType:
            return lt
        default:
            return nil
    }
}

func (la *listAccessExpr) typeCheck(ctx *typeCheckContext, log util.ErrorLog) {
    la.index.typeCheck(ctx, log)
    la.list.typeCheck(ctx, log)
}

func (la *listAccessExpr) projectionCheck(ctx *projectionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    la.index.projectionCheck(ctx, elog, rlog)
    la.list.projectionCheck(ctx, elog, rlog)
}

func (la *listAccessExpr) expressionCheck(ctx *expressionCheckContext, log util.ErrorLog) typedef {
    itype := la.index.expressionCheck(ctx, log)
    if itype != nil && !itype.subtypeOf(newIntType(la.line)) {
        la.reportErrorf(log, "list index %q (type=%s) is not of integer type.", la.index.String(), itype.String())
    }

    tdef := la.list.expressionCheck(ctx, log)

    if tdef != nil && !newListType(newNothingType(), la.line).subtypeOf(tdef) && !tdef.subtypeOf(newStringType(la.line)) {
        la.reportErrorf(log, "expression %v (type=%v) is not of list or string type.", la.list.String(), tdef.String())
        return nil
    }

    if tdef == nil {
        return nil
    }

    switch lt := tdef.(type) {
        case *stringType:
            // s[i] has type string (single-char as string)
            return lt
        case *listType:
            // xs[i] has the element type
            return lt.tdef
        case *nothingType:
            return lt
        default:
            // Shouldn’t happen due to the guard above, but be defensive.
            la.reportErrorf(log, "internal type error: unexpected list index base type %q.", tdef)
            return nil
    }
}

func (la *listAccessExpr) sessionCheck(ctx *sessionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    la.index.sessionCheck(ctx, elog, rlog)
    la.list.sessionCheck(ctx, elog, rlog)
}

func (la *listAccessExpr) evaluate(ctx *evaluationContext) expression {
    val2 := la.index.evaluate(ctx)
    val1 := la.list.evaluate(ctx)
    return val1.operation(ctx, val2, "index")
}

func (_ *listAccessExpr) operation(_ *evaluationContext, _ expression, _ string) expression { return nil }

func (la *listAccessExpr) prettyPrint(iw util.IndentedWriter) {
    la.list.prettyPrint(iw)
    iw.Print("[")
    la.index.prettyPrint(iw)
    iw.Print("]")
}

func (la *listAccessExpr) String() string {
    return la.list.String() + "[" + la.index.String() + "]"
}

/******************************************************************************
* slice expression
******************************************************************************/

type listSliceExpr struct {
    baseNode
    list expression
    lexpr expression
    rexpr expression
}

func newListSliceExpr(list expression, lexpr expression, rexpr expression, line int) *listSliceExpr {
    return &listSliceExpr {
        baseNode: baseNode{line: line},
        list: list,
        lexpr: lexpr,
        rexpr: rexpr,
    }
}

func (ls *listSliceExpr) setFilename(filename string) {
    ls.baseNode.setFilename(filename)
    ls.list.setFilename(filename)
    if ls.lexpr != nil {
        ls.lexpr.setFilename(filename)
    }
    if ls.rexpr != nil {
        ls.rexpr.setFilename(filename)
    }
}

func (ls *listSliceExpr) getType() typedef { return ls.list.getType() }
func (ls *listSliceExpr) typeCheck(ctx *typeCheckContext, log util.ErrorLog) {
    if ls.lexpr != nil {
        ls.lexpr.typeCheck(ctx, log)
    }
    if ls.rexpr != nil {
        ls.rexpr.typeCheck(ctx, log)
    }
    ls.list.typeCheck(ctx, log)
}

func (ls *listSliceExpr) projectionCheck(ctx *projectionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    if ls.lexpr != nil {
        ls.lexpr.projectionCheck(ctx, elog, rlog)
    }
    if ls.rexpr != nil {
        ls.rexpr.projectionCheck(ctx, elog, rlog)
    }
    ls.list.projectionCheck(ctx, elog, rlog)
}

func (ls *listSliceExpr) expressionCheck(ctx *expressionCheckContext, log util.ErrorLog) typedef {
     if ls.lexpr != nil {
        itype := ls.lexpr.expressionCheck(ctx, log)
        if itype != nil && !itype.subtypeOf(newIntType(ls.line)) {
            ls.lexpr.reportErrorf(log, "left slice index %q (type=%v) is not of integer type.", ls.lexpr.String(), itype.String())
        }
    }
    if ls.rexpr != nil {
        itype := ls.rexpr.expressionCheck(ctx, log)
        if itype != nil && !itype.subtypeOf(newIntType(ls.line)) {
            ls.rexpr.reportErrorf(log, "right slice index %q (type=%v) is not of integer type.", ls.rexpr.String(), itype.String())
        }
    }
    ltype := ls.list.expressionCheck(ctx, log)
    if ltype != nil && !newListType(newNothingType(), ls.line).subtypeOf(ltype) && !ltype.subtypeOf(newStringType(ls.line)) {
        ls.reportErrorf(log, "expression %q (type %v) is not of list or string type.", ls.list.String(), ltype.String())
        return nil
    }
    return ltype
}

func (ls *listSliceExpr) sessionCheck(ctx *sessionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    if ls.lexpr != nil {
        ls.lexpr.sessionCheck(ctx, elog, rlog)
    }
    if ls.rexpr != nil {
        ls.rexpr.sessionCheck(ctx, elog, rlog)
    }
    ls.list.sessionCheck(ctx, elog, rlog)
}

func (ls *listSliceExpr) evaluate(ctx *evaluationContext) expression {
    val := ls.list.evaluate(ctx)
    size := 0
    switch v := val.(type) {
        case *listExpr:
            size = len(v.expressions)
        case *stringExpr:
            size = len([]rune(v.stringVal))
    }

    lindex := 0
    rindex := size
    if ls.lexpr != nil {
        lindex = ls.lexpr.evaluate(ctx).(*intExpr).integer
    }
    if ls.rexpr != nil {
        rindex = ls.rexpr.evaluate(ctx).(*intExpr).integer
    }
    if lindex < 0 || lindex > size || rindex < 0 || rindex > size || lindex > rindex {
        s := "["
        if ls.lexpr != nil { s += fmt.Sprintf("%v", lindex) }
        s += ":"
        if ls.rexpr != nil { s += fmt.Sprintf("%v", rindex) }
        s += "]"
        return newNothingf(ls.line, "slice bounds (value=%s) out of range (range=[0:%d])", s, size)
    }

    switch v := val.(type) {
        case *listExpr:
            return newListExpr(v.expressions[lindex:rindex], v.line)
        case *stringExpr:
            runes := []rune(v.stringVal)
            return newStringExpr(string(runes[lindex:rindex]), v.line)
        default:
            return val
    }
}

func (_ *listSliceExpr) operation(_ *evaluationContext, _ expression, _ string) expression { return nil }

func (ls *listSliceExpr) prettyPrint(iw util.IndentedWriter) {
    ls.list.prettyPrint(iw)
    iw.Print("[")
    if ls.lexpr != nil {
        ls.lexpr.prettyPrint(iw)
    }
    iw.Print(":")
    if ls.rexpr != nil {
        ls.rexpr.prettyPrint(iw)
    }
    iw.Print("]")
}

func (ls *listSliceExpr) String() string {
    s := ls.list.String()
    if ls.lexpr != nil {
        s += ls.lexpr.String()
    }
    s += ":"
    if ls.rexpr != nil {
        s += ls.rexpr.String()
    }
    return s + "]"
}

/******************************************************************************
 * conditional expression
 ******************************************************************************/

type conditionalExpr struct {
    baseNode
    cond        expression
    thenExpr    expression
    elseExpr    expression
}

func newConditionalExpr(cond expression, thenExpr expression, elseExpr expression, line int) *conditionalExpr {
    return &conditionalExpr {
        baseNode:   baseNode{line: line},
        cond:       cond,
        thenExpr:   thenExpr,
        elseExpr:   elseExpr,
    }
}

func (ce *conditionalExpr) setFilename(filename string) {
    ce.baseNode.setFilename(filename)
    ce.cond.setFilename(filename)
    ce.thenExpr.setFilename(filename)
    ce.elseExpr.setFilename(filename)
}

func (ce *conditionalExpr) getType() typedef {
    thenTdef := ce.thenExpr.getType()
    elseTdef := ce.elseExpr.getType()
    if thenTdef == nil || elseTdef == nil {
        return nil
    }
    if thenTdef.subtypeOf(elseTdef) {
        return thenTdef
    } else if elseTdef.subtypeOf(elseTdef) {
        return elseTdef
    }
    return nil
}

func (ce *conditionalExpr) typeCheck(ctx *typeCheckContext, log util.ErrorLog) {
    ce.cond.typeCheck(ctx, log)
    ce.thenExpr.typeCheck(ctx, log)
    ce.elseExpr.typeCheck(ctx, log)
}

func (ce *conditionalExpr) projectionCheck(ctx *projectionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    ce.cond.projectionCheck(ctx, elog, rlog)
    ce.thenExpr.projectionCheck(ctx, elog, rlog)
    ce.elseExpr.projectionCheck(ctx, elog, rlog)
}

func (ce *conditionalExpr) expressionCheck(ctx *expressionCheckContext, log util.ErrorLog) typedef {
    condTdef := ce.cond.expressionCheck(ctx, log)
    thenTdef := ce.thenExpr.expressionCheck(ctx, log)
    elseTdef := ce.elseExpr.expressionCheck(ctx, log)
    if condTdef != nil {
        if _, ok := condTdef.(*boolType); !ok {
            ce.cond.reportErrorf(
                log,
                "condition %q (type=%s) is not of boolean type.", ce.cond.String(), condTdef.String(),
            )
        }
    }

    if thenTdef == nil {
        return elseTdef
    } 
    if elseTdef == nil {
        return thenTdef
    }

    // TODO consider a meet operator
    if thenTdef.subtypeOf(elseTdef) {
        return thenTdef
    } else if elseTdef.subtypeOf(thenTdef) {
        return elseTdef
    }
    return nil
}

func (ce *conditionalExpr) sessionCheck(ctx *sessionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    ce.cond.sessionCheck(ctx, elog, rlog)
    ce.thenExpr.sessionCheck(ctx, elog, rlog)
    ce.elseExpr.sessionCheck(ctx, elog, rlog)
} 

func (ce *conditionalExpr) evaluate(ctx *evaluationContext) expression {
    val := ce.cond.evaluate(ctx)
    switch val.(type) {
        case *trueExpr:
            return ce.thenExpr.evaluate(ctx)
        case *falseExpr:
            return ce.elseExpr.evaluate(ctx)
        case *nothing:
            msg := ce.cond.runtimeErrorf(
                "if-then-else condition error:\n\t\tcondition %q evaluates as %q.\n\tcontinue with else branch.",
                ce.cond.String(), val.String(),
            )
            fmt.Fprintln(os.Stderr, msg)
            return ce.elseExpr.evaluate(ctx)
        default:
            return newNothingf(
                ce.line,
                "unexpected value in if-then-else-evaluation: %q",
                val.String(),
            )
    }
}

func (_ *conditionalExpr) operation(_ *evaluationContext, _ expression, operator string) expression {
    return nil
}

func (ce *conditionalExpr) prettyPrint(iw util.IndentedWriter) {
    iw.Print("if ")
    ce.cond.prettyPrint(iw)
    iw.Print(" then ")
    ce.thenExpr.prettyPrint(iw)
    iw.Print(" else ")
    ce.elseExpr.prettyPrint(iw)
}

func (ce *conditionalExpr) String() string {
    return "if " + ce.cond.String() + " then " + ce.thenExpr.String() + " else " + ce.elseExpr.String() 
}

/******************************************************************************
 * record expression
 ******************************************************************************/

type recordExpr struct {
    baseNode
    labels      []string
    expressions []expression
    exprMap     map[string]expression
    mu          sync.RWMutex
}

func newRecordExpr(labels []string, expressions []expression, line int) *recordExpr {
    exprMap := make(map[string]expression, len(labels))
    for i, lab := range labels {
        exprMap[lab] = expressions[i]
    }
    return &recordExpr{
        baseNode:   baseNode{line: line},
        labels:     labels,
        expressions:expressions,
        exprMap:    exprMap,
    }
}

func (re *recordExpr) setFilename(filename string) {
    re.baseNode.setFilename(filename)
    for i := range re.expressions {
        re.expressions[i].setFilename(filename)
    }
}

func (re *recordExpr) getType() typedef {
    tdefs := make([]typedef, len(re.expressions) )
    for i, expr := range re.expressions {
        tdefs[i] = expr.getType()
        if tdefs[i] == nil {
            return nil
        }
    }
    return newRecordType(re.labels, tdefs, re.line)
}

func (re *recordExpr) typeCheck(ctx *typeCheckContext, log util.ErrorLog) {
    for i := range re.expressions {
        re.expressions[i].typeCheck(ctx, log)
    }
}

func (re *recordExpr) projectionCheck(ctx *projectionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    for i := range re.expressions {
        re.expressions[i].projectionCheck(ctx, elog, rlog)
    }
}

func (re *recordExpr) expressionCheck(ctx *expressionCheckContext, log util.ErrorLog) typedef {
    tdefs := make([]typedef, len(re.expressions))
    errorFlag := false
    for i, l := range re.labels {
        for j := i + 1; j < len(re.labels); j++ {
            if l == re.labels[j] {
                re.reportErrorf(log, "duplicate definition of record label: %q", l)
            }
        }
        tdef := re.expressions[i].expressionCheck(ctx, log)
        if tdef == nil {
            errorFlag = true
        }
        tdefs[i] = tdef
    }
    if errorFlag {
        return nil
    }
    return newRecordType(re.labels, tdefs, re.line)
}

func (re *recordExpr) sessionCheck(ctx *sessionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    for i := range re.expressions {
        re.expressions[i].sessionCheck(ctx, elog, rlog)
    }
} 

func (re *recordExpr) evaluate(ctx *evaluationContext) expression {
    expressions := make([]expression, len(re.expressions))
    for i := range re.expressions {
        expressions[i] = re.expressions[i].evaluate(ctx)
    }
    val := newRecordExpr(re.labels, expressions, re.line)
    val.setFilename(re.filename)
    return val
}

func (re *recordExpr) operation(_ *evaluationContext, _ expression, operator string) expression {
    re.mu.RLock()
    defer re.mu.RUnlock()
    if v, ok := re.exprMap[operator]; ok {
        return v
    }
    return newNothingf(re.line, "unknown record label: %q", operator)
}

func (re *recordExpr) prettyPrint(iw util.IndentedWriter) {
    iw.Print("{ ")
    for i := range re.labels {
        if i != 0 {
            iw.Print(", ")
        }
        iw.Print(re.labels[i] + ": ")
        re.expressions[i].prettyPrint(iw)
    }
    iw.Print(" }")
}

func (re *recordExpr) String() string {
    s := "{ "
    for i := range re.labels {
        if i != 0 {
            s += ", "
        }
        s += re.labels[i] + ": " + re.expressions[i].String()
    }
    return s + " }"
}

/******************************************************************************
 * access expression
 ******************************************************************************/

type accessExpr struct {
    baseNode
    expr expression
    name  string
}

func newAccessExpr(expr expression, name string, line int) *accessExpr {
    return &accessExpr{
        baseNode: baseNode{line: line},
        expr:     expr,
        name:     name,
    }
}

func (ra *accessExpr) setFilename(filename string) {
    ra.baseNode.setFilename(filename)
    ra.expr.setFilename(filename)
}

func (ra *accessExpr) getType() typedef {
    tdef := ra.expr.getType()
    if tdef == nil {
        return nil
    }
    switch accType := tdef.(type) {
        case *recordType:
            return accType.tdefMap[ra.name]
        case *moduleType:
            return accType.tdefMap[ra.name]
        default:
            return nil
    }
}

func (ra *accessExpr) typeCheck(ctx *typeCheckContext, log util.ErrorLog) {
    ra.expr.typeCheck(ctx, log)
}

func (ra *accessExpr) projectionCheck(ctx *projectionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    ra.expr.projectionCheck(ctx, elog, rlog)
}

func (ra *accessExpr) expressionCheck(ctx *expressionCheckContext, log util.ErrorLog) typedef {
    tdef := ra.expr.expressionCheck(ctx, log)
    if tdef == nil {
        return nil
    }

    switch accType := tdef.getType().(type) {
        case *recordType:
            td, ok := accType.tdefMap[ra.name]
            if !ok {
                ra.reportErrorf(log, "expression %q; unknown access name: %q.", ra.expr.String(), ra.name)
                return nil
            }
            return td
        case *moduleType:
            td, ok := accType.tdefMap[ra.name]
            if !ok {
                ra.reportErrorf(log, "expression %q; unknown access name: %q.", ra.expr.String(), ra.name)
                return nil
            }
            return td
        case *nothingType:
            return accType
        default:
            ra.reportErrorf(log, "expression %q; expecting expression of type module or record; instead found type %q.", ra.expr, tdef.String())
            return nil
    }
}

func (ra *accessExpr) evaluate(ctx *evaluationContext) expression {
    v := ra.expr.evaluate(ctx)
    // recordExpr.operation returns the field value; propagate nothing otherwise.
    if out := v.operation(ctx, nil, ra.name); out != nil {
        return out
    }
    return newNothingf(ra.line, "Unknown access name at runtime: %q", ra.name)
}

func (ra *accessExpr) sessionCheck(ctx *sessionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    ra.expr.sessionCheck(ctx, elog, rlog)
} 

func (_ *accessExpr) operation(_ *evaluationContext, _ expression, _ string) expression { return nil }

func (ra *accessExpr) prettyPrint(iw util.IndentedWriter) {
    ra.expr.prettyPrint(iw)
    iw.Print("." + ra.name)
}

func (ra *accessExpr) String() string { return ra.expr.String() + "." + ra.name }
