package ast

import (
    "fmt"
    "os"
    "sessions/ast/util"
)

type process interface {
    setFilename(filename string)
    typeCheck(*typeCheckContext, util.ErrorLog)
    expressionCheck(ctx *expressionCheckContext, lin *linearContext, log util.ErrorLog) *linearContext
    projectionCheck(*projectionCheckContext, util.ErrorLog, util.ReportLog)
    sessionCheck(*sessionCheckContext, util.ErrorLog, util.ReportLog)
    evaluate(ctx *evaluationContext)
    prettyPrint(iw util.IndentedWriter)
    fmt.Stringer
}

/******************************************************************************
 * sequential process
 ******************************************************************************/

type sequentialProc struct {
    baseNode
    expr seqExpr
    cont process
}

func newSequentialProc(expr seqExpr, cont process, line int) *sequentialProc {
    return &sequentialProc {
        baseNode: baseNode{line: line},
        expr: expr,
        cont: cont,
    }
}

func (sp *sequentialProc) setFilename(filename string) {
    sp.baseNode.setFilename(filename)
    sp.expr.setFilename(filename)
    sp.cont.setFilename(filename)
}

func (sp *sequentialProc) typeCheck(ctx *typeCheckContext, log util.ErrorLog) {
    sp.expr.typeCheck(ctx, log)
    sp.cont.typeCheck(ctx, log)
}

func (sp *sequentialProc) projectionCheck(ctx *projectionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    sp.expr.projectionCheck(ctx, elog, rlog)
    sp.cont.projectionCheck(ctx, elog, rlog)
}

func (sp *sequentialProc) expressionCheck(ctx *expressionCheckContext, lin *linearContext, log util.ErrorLog) *linearContext {
    sp.expr.expressionCheck(ctx, log)
    lin = sp.cont.expressionCheck(ctx, lin, log)
    sp.expr.removeVariable(ctx)
    lin = sp.expr.localType(lin)
    return lin
}

func (sp *sequentialProc) sessionCheck(ctx *sessionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    sp.expr.sessionCheck(ctx, elog, rlog)
    sp.cont.sessionCheck(ctx, elog, rlog)
}

func (sp *sequentialProc) evaluate(ctx *evaluationContext) {
    sp.expr.evaluate(ctx)
    sp.cont.evaluate(ctx)
}

func (sp *sequentialProc) prettyPrint(iw util.IndentedWriter) {
    sp.expr.prettyPrint(iw)
    iw.Println(";")
    sp.cont.prettyPrint(iw)
}

func (sp *sequentialProc) String() string {
    s := sp.expr.String()
    s += ";" + sp.cont.String()
    return s
}

/******************************************************************************
* if then else process
******************************************************************************/

type ifThenElseProc struct {
    baseNode
    condition expression
    thenbranch process
    elsebranch process
}

func newIfThenElseProc(condition expression, thenbranch process, elsebranch process, line int) *ifThenElseProc {
    return &ifThenElseProc {
        baseNode: baseNode{line: line},
        condition: condition,
        thenbranch: thenbranch,
        elsebranch: elsebranch,
    }
}

func (ite *ifThenElseProc) setFilename(filename string) {
    ite.baseNode.setFilename(filename)
    ite.condition.setFilename(filename)
    ite.thenbranch.setFilename(filename)
    ite.elsebranch.setFilename(filename)
}

func (ite *ifThenElseProc) typeCheck(ctx *typeCheckContext, log util.ErrorLog) {
    ite.condition.typeCheck(ctx, log)
    ite.thenbranch.typeCheck(ctx, log)
    ite.elsebranch.typeCheck(ctx, log)
}

func (ite *ifThenElseProc) projectionCheck(ctx *projectionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    ite.condition.projectionCheck(ctx, elog, rlog)
    ite.thenbranch.projectionCheck(ctx, elog, rlog)
    ite.elsebranch.projectionCheck(ctx, elog, rlog)
}

func (ite *ifThenElseProc) expressionCheck(ctx *expressionCheckContext, lin *linearContext, log util.ErrorLog) *linearContext {
    cond := ite.condition.expressionCheck(ctx, log)
    lc1 := lin.clone()
    lin1 := ite.thenbranch.expressionCheck(ctx, lc1, log)
    lc2 := lin.clone()
    lin2 := ite.elsebranch.expressionCheck(ctx, lc2, log)
    if cond != nil {
        if _, ok := cond.(*boolType); !ok {
            ite.condition.reportErrorf(
                log,
                "Condition %q (type=%s) is not of boolean type.", ite.condition.String(), cond.String(),
            )
        }
    }

    newLin, ok := lin1.join(lin2)
    if !ok {
        stream1 := util.NewStream().Inc().Inc()
        stream2 := util.NewStream().Inc().Inc()
        lin1.prettyPrint(stream1)
        lin2.prettyPrint(stream2)
        ite.reportErrorf(log, "Branches do not have the same local types.\n\tif branch:\n%s\n\telse branch:\n%s", stream1.String(), stream2.String())

    }
    return newLin
}

func (ite *ifThenElseProc) sessionCheck(ctx *sessionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    ite.condition.sessionCheck(ctx, elog, rlog)
    ite.thenbranch.sessionCheck(ctx, elog, rlog)
    ite.elsebranch.sessionCheck(ctx, elog, rlog)
}

func (ite *ifThenElseProc) evaluate(ctx *evaluationContext) {
    cond := ite.condition.evaluate(ctx)
    switch cond.(type) {
        case *trueExpr:
            ite.thenbranch.evaluate(ctx)
        case *falseExpr:
            ite.elsebranch.evaluate(ctx)
        case *nothing:
            msg := ite.runtimeErrorf(
                "if-then-else evaluation error:\n\t\tcondition %q evaluates as %q.\n\tContinue with else branch.", 
                ite.condition.String(), cond.String(),
            )
            fmt.Fprintln(os.Stderr, msg)
            ite.elsebranch.evaluate(ctx)
    }
}

func (ite *ifThenElseProc) prettyPrint(iw util.IndentedWriter) {
    iw.Print("if\t")
    ite.condition.prettyPrint(iw)
    iw.Println()
    iw.Print("then\t")
    iw.Inc()
    ite.thenbranch.prettyPrint(iw)
    iw.Dec()
    iw.Print("else\t")
    iw.Inc()
    ite.elsebranch.prettyPrint(iw)
    iw.Dec()
}

func (ite *ifThenElseProc) String() string {
    return "if " + ite.condition.String() + " then " + ite.thenbranch.String() + " else " + ite.elsebranch.String()
}

/******************************************************************************
* select
******************************************************************************/

type selectProc struct {
    baseNode
    participant *participantExpr
    labels []*labelExpr
    conditions []expression
    sexprs []*sendExpr
    processes []process
}

func newSelectProc(participant *participantExpr, conditions []expression, labels []*labelExpr, processes []process, line int) *selectProc {
    sexprs := make([]*sendExpr, len(labels))
    for i := range labels {
        sexprs[i] = newSendExpr(participant, labels[i], labels[i].lineno())
    }
    return &selectProc {
        baseNode: baseNode{line: line},
        participant: participant,
        conditions: conditions,
        labels: labels,
        processes: processes,
        sexprs: sexprs,
    }
}

func (sp *selectProc) setFilename(filename string) {
    sp.baseNode.setFilename(filename)
    sp.participant.setFilename(filename)
    for _, cond := range sp.conditions {
        cond.setFilename(filename)
    }
    for _, lab := range sp.labels {
        lab.setFilename(filename)
    }
    for _, proc := range sp.processes {
        proc.setFilename(filename)
    }
}

func (sp *selectProc) typeCheck(ctx *typeCheckContext, log util.ErrorLog) {
    sp.participant.typeCheck(ctx, log)
    for _, cond := range sp.conditions {
        cond.typeCheck(ctx, log)
    }
    for _, lab := range sp.labels {
        lab.typeCheck(ctx, log)
    }
    for _, proc := range sp.processes {
        proc.typeCheck(ctx, log)
    }
}

func (sp *selectProc) projectionCheck(ctx *projectionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    for _, cond := range sp.conditions {
        cond.projectionCheck(ctx, elog, rlog)
    }
    for _, lab := range sp.labels {
        lab.projectionCheck(ctx, elog, rlog)
    }
    for _, proc := range sp.processes {
        proc.projectionCheck(ctx, elog, rlog)
    }
}

func (sp *selectProc) expressionCheck(ctx *expressionCheckContext, lin *linearContext, log util.ErrorLog) *linearContext {
    ptype := sp.participant.expressionCheck(ctx, log).(*participantType)

    labels  := make([]*labelType, 0, len(sp.labels))
    choices := make([]*linearContext, 0, len(sp.processes))
    seen := util.NewHashSet[string]()

    for i := range sp.processes {
        if i < len(sp.processes) - 1 {
            if t := sp.conditions[i].expressionCheck(ctx, log); t != nil {
                if _, ok := t.(*boolType); !ok {
                    sp.conditions[i].reportErrorf(log, "Condition %q (type=%s) is not of boolean type.", sp.conditions[i].String(), t.String())
                }
            }
        }
        lab := sp.labels[i].expressionCheck(ctx, log).(*labelType)
        if seen.Contains(lab.label) {
            lab.reportErrorf(log, "Duplicate label definition: %q", lab.String())
        } else {
            seen.Add(lab.label)
        }
        labels = append(labels, lab)
        lc := lin.clone()
        choices = append(choices, sp.processes[i].expressionCheck(ctx, lc, log))
    }
    newLin, ok := lin.newSelectLocal(ptype, labels, choices, sp.line)
    if !ok {
        stream := util.NewStream().Inc().Inc()
        for i := range labels {
            if i != 0 {
                stream.Println()
            }
            stream.Printf("label %s: ", labels[i].String())
            stream.Println()
            stream.Inc()
            choices[i].prettyPrint(stream)
            stream.Dec()
        }
        sp.reportErrorf(log, "Select cases do not have the same non-select local types:\n%s", stream.String())
    }
    return newLin
}

func (sp *selectProc) sessionCheck(ctx *sessionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    for _, cond := range sp.conditions {
        cond.sessionCheck(ctx, elog, rlog)
    }
    for _, lab := range sp.labels {
        lab.sessionCheck(ctx, elog, rlog)
    }
    for _, proc := range sp.processes {
        proc.sessionCheck(ctx, elog, rlog)
    }
}

func (sp *selectProc) evaluate(ctx *evaluationContext) {
    for i, cond := range sp.conditions {
        v := cond.evaluate(ctx)
        switch v.(type) {
            case *trueExpr:
                sp.sexprs[i].evaluate(ctx)
                sp.processes[i].evaluate(ctx)
                return
            case *falseExpr:
            case *nothing:
                 msg := cond.runtimeErrorf(
                    "select evaluation error:\n\t\tcondition %q evaluates as %q.\n\tContinue with the next branch.", 
                    cond.String(), v.String(),
                )
                fmt.Fprintln(os.Stderr, msg)
            default:
                return
        }
    }
    index := len(sp.processes) - 1
    sp.sexprs[index].evaluate(ctx)
    sp.processes[index].evaluate(ctx)
}

func (sp *selectProc) prettyPrint(iw util.IndentedWriter) {
    iw.Print("select ")
    sp.participant.prettyPrint(iw)
    iw.Println(" of")
    iw.Inc()
    for i := range sp.conditions {
        sp.conditions[i].prettyPrint(iw)
        iw.Print(" <- ")
        sp.labels[i].prettyPrint(iw)
        iw.Println(":")
        iw.Inc()
        sp.processes[i].prettyPrint(iw)
        iw.Dec()
    }
    sp.labels[len(sp.labels)-1].prettyPrint(iw)
    iw.Println(":")
    iw.Inc()
    sp.processes[len(sp.processes)-1].prettyPrint(iw)
    iw.Dec()
    iw.Dec()
    iw.Println()
}

func (sp *selectProc) String() string {
    s := "select " + sp.participant.String() + " of "
    for i := range sp.conditions {
        s += sp.conditions[i].String() + "\t" + sp.labels[i].String() + ": " + sp.processes[i].String() + " "
    }
    s += sp.labels[len(sp.labels) - 1].String() + ": " + sp.processes[len(sp.processes) - 1].String()
    return s
}

/******************************************************************************
* branch process
******************************************************************************/

type branchProc struct {
    baseNode
    participant *participantExpr
    labels      []*labelExpr
    processes   []process
    labelMap    map[string]process
}

func newBranchProc(participant *participantExpr, labels []*labelExpr, processes []process, line int) *branchProc {
    labelMap := make(map[string]process, len(labels))
    for i, lab := range labels {
        labelMap[lab.label] = processes[i]
    }
    return &branchProc {
        baseNode: baseNode{line: line},
        participant: participant,
        labels: labels,
        processes: processes,
        labelMap: labelMap,
    }
}

func (bp *branchProc) setFilename(filename string) {
    bp.baseNode.setFilename(filename)
    bp.participant.setFilename(filename)
    for _, lab := range bp.labels {
        lab.setFilename(filename)
    }
    for _, proc := range bp.processes {
        proc.setFilename(filename)
    }
}

func (bp *branchProc) typeCheck(ctx *typeCheckContext, log util.ErrorLog) {
    bp.participant.typeCheck(ctx, log)
    for _, lab := range bp.labels {
        lab.typeCheck(ctx, log)
    }
    for _, proc := range bp.processes {
        proc.typeCheck(ctx, log)
    }
}

func (np *branchProc) projectionCheck(ctx *projectionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    for _, lab := range np.labels {
        lab.projectionCheck(ctx, elog, rlog)
    }
    for _, proc := range np.processes {
        proc.projectionCheck(ctx, elog, rlog)
    }
}

func (bp *branchProc) expressionCheck(ctx *expressionCheckContext, lin *linearContext, log util.ErrorLog) *linearContext {
    ptype := bp.participant.expressionCheck(ctx, log).(*participantType)

    labels  := make([]*labelType, 0, len(bp.labels))
    choices := make([]*linearContext, 0, len(bp.processes))
    seen := util.NewHashSet[string]()

    for i, proc := range bp.processes {
        lab := bp.labels[i].expressionCheck(ctx, log).(*labelType)
        if seen.Contains(lab.label) {
            lab.reportErrorf(log, "Duplicate label definition: %q", lab.String())
        } else {
            seen.Add(lab.label)
        }
        labels = append(labels, lab)
        lc := lin.clone()
        choices = append(choices, proc.expressionCheck(ctx, lc, log))
    }
    newLin, ok := lin.newBranchLocal(ptype, labels, choices, bp.line)
    if !ok {
        stream := util.NewStream().Inc().Inc()
        for i := range labels {
            if i != 0 {
                stream.Println()
            }
            stream.Printf("label %s: ", labels[i].String())
            stream.Println()
            stream.Inc()
            choices[i].prettyPrint(stream)
            stream.Dec()
        }
        bp.reportErrorf(log, "Branch cases do not have the same non-branch local types:\n%s", stream.String())
    }
    return newLin
}

func (np *branchProc) sessionCheck(ctx *sessionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    for _, lab := range np.labels {
        lab.sessionCheck(ctx, elog, rlog)
    }
    for _, proc := range np.processes {
        proc.sessionCheck(ctx, elog, rlog)
    }
}

func (bp *branchProc) evaluate(ctx *evaluationContext) {
    p := bp.participant.evaluate(ctx)
    //v := p.operation(nil, "receive")
    pp := p.(*participantExpr)
    ch := ctx.getReceiveParticipantChannel(pp)
    //v := <- ch
    v, ok := ch.Receive()
    if !ok {
        //channel ch is closed - choose a label
        v = bp.labels[0]
    }
    lab, ok1 := v.(*labelExpr)
    if !ok1 {
        msg := p.runtimeErrorf("Received a non label value: %q", v.String())
        fmt.Fprintln(os.Stderr, msg)
        return
    }

    proc, ok2 := bp.labelMap[lab.label]
    if !ok2 {
        msg := p.runtimeErrorf("Received an unknown label: %q", lab.label)
        fmt.Fprintln(os.Stderr, msg)
        return
    }
    proc.evaluate(ctx)
}

func (bp *branchProc) prettyPrint(iw util.IndentedWriter) {
    iw.Print("branch ")
    bp.participant.prettyPrint(iw)
    iw.Println(" of")
    iw.Inc()
    for i, proc := range bp.processes {
        bp.labels[i].prettyPrint(iw)
        iw.Print(":\t")
        iw.Inc()
        proc.prettyPrint(iw)
        iw.Dec()
    }
    iw.Dec()
    iw.Println()
}

func (bp *branchProc) String() string {
    s := "branch " + bp.participant.String() + " of "
    for i, proc := range bp.processes {
        s += bp.labels[i].String() + "<-" + proc.String() + " "
    }
    return s
}

/******************************************************************************
 * intro Process
 ******************************************************************************/

type concurrentProc struct {
    baseNode
    participant *participantExpr
    participants []*participantExpr
    applications []*application
    proc process
    participantLoc local
    lcont *localContext
}

func newConcurrentProc(participant *participantExpr, participants []*participantExpr, applications []*application, proc process, line int) *concurrentProc {
    return &concurrentProc {
        baseNode: baseNode{line: line},
        participant: participant,
        participants: participants,
        applications: applications,
        proc: proc,
    }
}

func (ip *concurrentProc) setFilename(filename string) {
    ip.baseNode.setFilename(filename)
    if ip.participant != nil {
        ip.participant.setFilename(filename)
    }
    for i := range ip.participants {
        ip.participants[i].setFilename(filename)
        ip.applications[i].setFilename(filename)
    }
    ip.proc.setFilename(filename)
    if ip.lcont != nil {
        ip.lcont.setFilename(filename)
    }
}

func (ip *concurrentProc) typeCheck(ctx *typeCheckContext, log util.ErrorLog) {
    if ip.participant != nil {
        ip.participant.typeCheck(ctx, log)
    }
    for i := range ip.participants {
        ip.participants[i].typeCheck(ctx, log)
        ip.applications[i].typeCheck(ctx, log)
    }
    ip.proc.typeCheck(ctx, log)
    if ip.lcont != nil {
        ip.lcont.typeCheck(ctx, log)
    }
}

func (ip *concurrentProc) projectionCheck(ctx *projectionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    if ip.participant != nil {
        ip.participant.projectionCheck(ctx, elog, rlog)
    }
    for i := range ip.applications {
        ip.participants[i].projectionCheck(ctx, elog, rlog)
        ip.applications[i].projectionCheck(ctx, elog, rlog)
    }
    ip.proc.projectionCheck(ctx, elog, rlog)
    if ip.lcont != nil {
        ip.lcont.projectionCheck(ctx, elog, rlog)
    }
}

func (ip *concurrentProc) expressionCheck(ctx *expressionCheckContext, lin *linearContext, log util.ErrorLog) *linearContext {
    pset := util.NewHashSet[string]()
    var locals []local
    var ptypes []*participantType
    if ip.participant != nil {
        pset.Add(ip.participant.String())
        locals = make([]local, len(ip.applications) + 1)
        ptypes = make([]*participantType, len(ip.participants) + 1)
        ptypes[len(ip.participants)] = ip.participant.pType()
    } else {
        locals = make([]local, len(ip.applications))
        ptypes = make([]*participantType, len(ip.participants))
    }
    for i, participant := range ip.participants {
        if !ctx.addParticipant(participant) {
            participant.reportErrorf(log, "Duplicate definition of participant: %q.", participant.String())
        } else {
            pset.Add(participant.String())
        }
        ptypes[i] = participant.pType()
    }

    errorFlag := false
    for i := range ip.applications {
        locals[i] = ip.applications[i].expressionCheck(ip.participants[i], pset, ctx, log)
        if locals[i] == nil {
            errorFlag = true
        }
    }

    lin = lin.newSession(ip.participants, ip.line)
    lin = ip.proc.expressionCheck(ctx, lin, log)
    ip.participantLoc = lin.removeSession(ip.participants, ip.line)
    if ip.participant != nil {
        locals[len(ip.applications)] = ip.participantLoc
    }

    for _, participant := range ip.participants {
        ctx.removeParticipant(participant)
    }
    if !errorFlag {
        ip.lcont = newLocalContext(ptypes, locals, ip.line)
    }
    return lin
}

func (ip *concurrentProc) sessionCheck(ctx *sessionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    if ip.participant != nil {
        ip.participant.sessionCheck(ctx, elog, rlog)
    }
    for i := range ip.applications {
        ip.participants[i].sessionCheck(ctx, elog, rlog)
        ip.applications[i].sessionCheck(ctx, elog, rlog)
    }
    if ip.participant == nil {
        if !ip.participantLoc.subtypeOf(newEndLocal(ip.line), util.NewHashSet[typePair]()) {
            stream := util.NewStream().Inc().Inc()
            ip.participantLoc.prettyPrint(stream)
            ip.reportErrorf(
                elog, 
                "Expecting no interaction on session initiation participants. Found interaction:\n%v\n\tinstead.", 
                stream.String(),
            )
        }
    }
    if ip.lcont != nil {
        //checking for compatibility between local types
        if glob, ok := ip.lcont.liveness(); !ok {
            stream := util.NewStream().Inc().Inc()
            ip.lcont.prettyPrint(stream)
            ip.reportErrorf(elog, "non-live session roles:\n%v", stream.String())
        } else if ctx.analysis() {
            gdef := newGlobalDef(ip.lcont.participants(), glob, ip.line)
            stream := util.NewStream()
            stream.Printf("Local context liveness analysis at line %v produces global type:\n", ip.line)
            stream.Inc().Inc()
            gdef.prettyPrint(stream)
            ip.reportf(rlog, stream.String())
        }
    }
    ip.proc.sessionCheck(ctx, elog, rlog)
}

func (ip *concurrentProc) evaluate(ctx *evaluationContext) {
    channels := ip.lcont.channels(newBasicChannel(true)) //ip.lcont.channels(true)

    for i := range ip.applications {
        ip.applications[i].evaluate(ip.participants[i], nil, channels, ctx)
    }

    if ip.participant != nil {
        for _, p := range ip.participants {
            ctx.addParticipantChannel(p, channels[ip.participant.id][p.id], channels[p.id][ip.participant.id])
        }
    }

    ip.proc.evaluate(ctx)
}

func (ip *concurrentProc) prettyPrint(iw util.IndentedWriter) {
    iw.Print("conc")
    if ip.participant != nil {
        iw.Print(" as role ")
        ip.participant.prettyPrint(iw)
    }
    iw.Println(" with")
    iw.Inc()
    for i := range ip.applications {
        iw.Print("role ")
        ip.participants[i].prettyPrint(iw)
        iw.Print(" as ")
        ip.applications[i].prettyPrint(iw)
        iw.Println(";")
    }
    iw.Dec()
    ip.proc.prettyPrint(iw)
}

func (ip *concurrentProc) String() string {
    s := "conc"
    if ip.participant != nil {
        s += " as role " + ip.participant.String()
    }
    s += " with"
    for i := range ip.applications {
        s += " role " + ip.participants[i].String() + " as " + ip.applications[i].String() + ";"
    }
    s += ip.proc.String()
    return s
}

/******************************************************************************
 * term Process
 ******************************************************************************/

type terminate struct {
    baseNode
}

func newTerminate(line int) *terminate {
    return &terminate {
        baseNode: baseNode{line: line},
    }
}

func (t *terminate) setFilename(filename string)                    { t.baseNode.setFilename(filename) }
func (_ *terminate) typeCheck(_ *typeCheckContext, _ util.ErrorLog) {}
func (_ *terminate) projectionCheck(_ *projectionCheckContext, _ util.ErrorLog, _ util.ReportLog) { }
func (t *terminate) expressionCheck(_ *expressionCheckContext, lin *linearContext, _ util.ErrorLog) *linearContext {
    return lin.newEndLocal(t.line)
}

func (_ *terminate) sessionCheck(_ *sessionCheckContext, _ util.ErrorLog, _ util.ReportLog) { }
func (_ *terminate) evaluate(_ *evaluationContext)      { }
func (t *terminate) prettyPrint(iw util.IndentedWriter) { iw.Println(t.String() + ";") }
func (t *terminate) String() string                     { return "term" }
