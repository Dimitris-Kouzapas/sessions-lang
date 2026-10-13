package ast

import (
    "sessions/ast/util"
    "sessions/ast/communication"
)

/******************************************************************************
 * abstraction
 ******************************************************************************/
type abstraction struct {
    baseNode
    initParameters []*participantExpr
    parameters []*participantExpr
    body process
}

func newAbstraction(initParameters []*participantExpr, parameters []*participantExpr, body process, line int) *abstraction {
    return &abstraction {
        baseNode: baseNode{line: line},
        initParameters: initParameters,
        parameters: parameters,
        body: body,
    }
}

func (a *abstraction) setFilename(filename string) {
    a.baseNode.setFilename(filename)
    for _, parameter := range a.parameters {
        parameter.setFilename(filename)
    }
    for _, iparameter := range a.initParameters {
        iparameter.setFilename(filename)
    }
    a.body.setFilename(filename)
}

func (a *abstraction) getType() typedef {
    // cannot decide on the type during the typecheck phase
    return nil
}

func (a *abstraction) typeCheck(ctx *typeCheckContext, log util.ErrorLog) {
    for _, parameter := range a.parameters {
          parameter.typeCheck(ctx, log)
    }
    for _, iparameter := range a.initParameters {
        iparameter.typeCheck(ctx, log)
    }
    a.body.typeCheck(ctx, log)
}

func (a *abstraction) projectionCheck(ctx *projectionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    for _, parameter := range a.parameters {
        parameter.projectionCheck(ctx, elog, rlog)
    }
    for _, iparameter := range a.initParameters {
        iparameter.projectionCheck(ctx, elog, rlog)
    }
    a.body.projectionCheck(ctx, elog, rlog) 
}

func (a *abstraction) expressionCheck(ctx *expressionCheckContext, log util.ErrorLog) typedef {
    ctx.pushFrame()
    defer ctx.popFrame()

    parameters := make([]*participantType, len(a.parameters))
    for i, parameter := range a.parameters {
        if !ctx.addParticipant(parameter) {
            parameter.reportErrorf(log, "duplicate definition of parameter: %q.", parameter.String())
        }
        td := parameter.expressionCheck(ctx, log)
        ptype, ok := td.(*participantType)
        if !ok || ptype == nil {
            parameter.reportErrorf(log, "parameter %q: expecting participant type; instead found: %q.", parameter.String(), td.String())
            continue
        }
        parameters[i] = ptype
    }
    
    initParameters := make([]*participantType, len(a.initParameters))
    for i, iparameter := range a.initParameters {
        if !ctx.addParticipant(iparameter) {
            iparameter.reportErrorf(log, "duplicate definition of initialisation parameter: %q.", iparameter.String())
        }
        td := iparameter.expressionCheck(ctx, log)
        iptype, ok := td.(*participantType)
        if !ok || iptype == nil {
            iparameter.reportErrorf(log, "initialisation parameter %q: expecting participant type; instead found: %q.", iparameter, td.String())
            continue
        }
        initParameters[i] = iptype
    }

    lin := newLinearContext(a.parameters, a.line)
    lin = lin.newSession(a.initParameters, a.line)
    lin = a.body.expressionCheck(ctx, lin, log)
    loc := lin.removeSession(a.parameters, a.line)
    init := lin.removeSession(a.initParameters, a.line)

    if lin.recLoc != nil && !loc.subtypeOf(lin.recLoc, util.NewHashSet[typePair]()) {
        stream1 := util.NewStream()
        stream1.Inc().Inc()
        stream2 := util.NewStream()
        stream2.Inc().Inc()
        loc.prettyPrint(stream1)
        lin.recLoc.prettyPrint(stream2)
        a.reportErrorf(log, "expecting recursive type\n %s\n but found\n %s\n instead", stream2.String(), stream1.String())
    }

    return  newAbstractionType(
                newLocalAbstraction(initParameters, init, a.line),
                newLocalAbstraction(parameters, loc, a.line),
                a.line,
            )
}

func (a *abstraction) sessionCheck(ctx *sessionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    a.body.sessionCheck(ctx, elog, rlog)
}

func (a *abstraction) runtimeAbstraction(ctx *evaluationContext) *runtimeAbstraction {
    return &runtimeAbstraction {
        abstraction: abstraction{
            baseNode: baseNode{line: a.line, filename: a.filename},
            initParameters: a.initParameters,
            parameters: a.parameters,
            body: a.body,
        },
        m:          ctx.m,
        variables:  ctx.cloneVariables(),
    }
}

func (a *abstraction) evaluate(ctx *evaluationContext) expression {
    return a.runtimeAbstraction(ctx)
}

func (_ *abstraction) operation(_ *evaluationContext, _ expression, _ string) expression { return nil }

func (a *abstraction) prettyPrint(iw util.IndentedWriter) {
    iw.Print("proc")
    for _, iparameter := range a.initParameters {
        iw.Print(" {")
        iparameter.prettyPrint(iw)
        iw.Print("}")
    }
    for _, parameter := range a.parameters {
        iw.Print(" ")
        parameter.prettyPrint(iw)
    }
    iw.Println(".")
    iw.Inc()
    a.body.prettyPrint(iw)
    iw.Dec()
}

func (a *abstraction) String() string {
    s := "proc"
    for _, iparameter := range a.initParameters {
        s += " {" + iparameter.String() + "}"
    }
    for _, parameter := range a.parameters {
        s += " " + parameter.String()
    }
    return s + ". " + a.body.String()
}

/******************************************************************************
 * runtime abstraction
 ******************************************************************************/

type runtimeAbstraction struct {
    abstraction
    m           *module
    variables   map[string]expression
}

func (ra *runtimeAbstraction) spawn(sendChannels []communication.Channel[expression], receiveChannels []communication.Channel[expression], ctx *evaluationContext) {
    defer ctx.done()
    //TODO channel clean up should not be done whenever we have true recursion
    // defer ctx.cleanup() // <- closes channels and releases resources

    newContext := ctx.emptyCopy(ra.m, ra.variables)

    for i := range ra.parameters {
        newContext.addParticipantChannel(ra.parameters[i], sendChannels[i], receiveChannels[i])
    }

    initChannelIndex := len(ra.parameters)
    for i := range ra.initParameters {
        newContext.addParticipantChannel(ra.initParameters[i], sendChannels[initChannelIndex + i], receiveChannels[initChannelIndex + i])
    }

    ra.body.evaluate(newContext)
}

func (ra *runtimeAbstraction) runtimeAbstraction() *runtimeAbstraction {
    variables := make(map[string]expression)
    for k, v := range ra.variables {
       variables[k] = v
    }
    return &runtimeAbstraction {
        abstraction: abstraction{
            baseNode:       baseNode{line: ra.line, filename: ra.filename},
            body:           ra.body,
            parameters:     ra.parameters,
            initParameters: ra.initParameters,
        },
        m:          ra.m,
        variables:  variables,
    }
}

func (ra *runtimeAbstraction) evaluate(_ *evaluationContext) expression {
    return ra.runtimeAbstraction()
}

/******************************************************************************
 * application
 ******************************************************************************/

type application struct {
    baseNode
    abstrTerm   expression
    arguments   []*participantExpr
    initTerm    expression

    lcont      *localContext
}

func newApplication(abstrTerm expression, arguments []*participantExpr, initTerm expression, line int) *application {
    return &application {
        baseNode:   baseNode{line: line},
        abstrTerm:  abstrTerm,
        arguments:  arguments,
        initTerm:   initTerm,
    }
}

func (a *application) setFilename(filename string) {
    a.baseNode.setFilename(filename)
    a.abstrTerm.setFilename(filename)
    for _, argument := range a.arguments {
        argument.setFilename(filename)
    }
    if a.initTerm != nil {
        a.initTerm.setFilename(filename)
    }
}

func (a *application) typeCheck(ctx *typeCheckContext, log util.ErrorLog) {
    a.abstrTerm.typeCheck(ctx, log)
    for _, argument := range a.arguments {
        argument.typeCheck(ctx, log)
    }
    if a.initTerm != nil {
        a.initTerm.typeCheck(ctx, log)
    }
}

func (a *application) projectionCheck(ctx *projectionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    a.abstrTerm.projectionCheck(ctx, elog, rlog)
    for _, argument := range a.arguments {
        argument.projectionCheck(ctx, elog, rlog)
    }
    if a.initTerm != nil {
        a.initTerm.projectionCheck(ctx, elog, rlog)
    }
}

func (a *application) expressionCheck(pexpr *participantExpr, pset util.Set[string], ctx *expressionCheckContext, log util.ErrorLog) local {
    tdef := a.abstrTerm.expressionCheck(ctx, log)

    var abstrType *abstractionType
    var ok bool
    if tdef != nil {
        abstrType, ok = tdef.(*abstractionType)
        if !ok {
            a.reportErrorf(log, "expecting local abstraction type; instead found: %q.", tdef)
        }
    }

    seen := util.NewHashSet[string]()
    for _, argument := range a.arguments {
        if seen.Contains(argument.id) {
            argument.reportErrorf(log, "cannot use participant %q twice as argument.", argument.String())
        } else {
            seen.Add(argument.id)
        }
        if argument.id == pexpr.id {
            a.reportErrorf(log, "self-reference participant argument: %q", argument.String())
        }
        if pset != nil && !pset.Contains(argument.id) {
            a.reportErrorf(log, "unknown participant: %q", argument.String())
        }
    }

    locals := make([]local, 2)
    ptypes := make([]*participantType, 2)
    ptypes[0] = newParticipantType("init", a.line)
    ptypes[1] = pexpr.pType()

    if a.initTerm != nil {
        td := a.initTerm.expressionCheck(ctx, log)
        if td != nil { 
            initTermType, ok := td.(*abstractionType)
            if !ok {
                a.initTerm.reportErrorf(log, "expecting local abstraction type; instead found: %q.", td)
            } else {
                if !initTermType.tdef.subtypeOf(  newLocalAbstraction( make([]*participantType, 0), newEndLocal(a.line), a.line) ) {
                    a.initTerm.reportErrorf(log, "expecting only initialisation participant.")
                }
                initLa := initTermType.initAbstr.substitute([]*participantType{ pexpr.pType() })
                locals[0] = initLa.loc
            }
        } else {
            locals[0] = newEndLocal(a.line)    
        }
    } else {
        locals[0] = newEndLocal(a.line)
    }

    if tdef == nil {
        locals[1] = newEndLocal(a.line)
        a.lcont = newLocalContext(ptypes, locals, a.line)
        return nil
    }

    if abstrType == nil {
        return nil
    }

    abstrLa := abstrType.initAbstr.substitute([]*participantType{ ptypes[0] })
    locals[1] = abstrLa.loc
    a.lcont = newLocalContext(ptypes, locals, a.line)

    if len(abstrType.locAbstr().parameters) != len(a.arguments) {
        a.reportErrorf(
            log,
            "wrong number of arguments; expecting %d, but found %d.", 
            len(abstrType.locAbstr().parameters),
            len(a.arguments),
        )
        return newEndLocal(a.line)
    }

    return abstrType.locAbstr().apply(a.arguments)
}

func (a *application) sessionCheck(ctx *sessionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    a.abstrTerm.sessionCheck(ctx, elog, rlog)
    if a.initTerm != nil {
        a.initTerm.sessionCheck(ctx, elog, rlog)
    }

    if a.lcont != nil {
        //checking for compatibility between initialisation local types
        if glob, ok := a.lcont.liveness(); !ok {
            stream := util.NewStream().Inc().Inc()
            a.lcont.prettyPrint(stream)
            a.reportErrorf(elog, "non-live initialisation roles:\n%v", stream.String())
        } else if ctx.analysis() {
            gdef := newGlobalDef(a.lcont.participants(), glob, a.line)
            stream := util.NewStream()
            stream.Printf("Local context liveness analysis at line %v produces global type:\n", a.line)
            stream.Inc().Inc()
            gdef.prettyPrint(stream)
            a.reportf(rlog, stream.String())
        }
    }
}

func evaluateAbstractionExpr(abstr expression, ctx *evaluationContext, sendChannels []communication.Channel[expression], receiveChannels []communication.Channel[expression]) {
    ra := abstr.evaluate(ctx).(*runtimeAbstraction)
    ctx.add()
    go ra.spawn(sendChannels, receiveChannels, ctx)
}

func (a *application) evaluate(pexpr *participantExpr, substitution map[string]string, channels channelMap, ctx *evaluationContext) {
    sendChannels := make([]communication.Channel[expression], len(a.arguments))
    receiveChannels := make([]communication.Channel[expression], len(a.arguments))

    from := pexpr.id
    if substitution != nil {
        from = substitution[pexpr.id]
    }

    for i, pexpr2 := range a.arguments {
        to := pexpr2.id
        if substitution != nil {
            to = substitution[pexpr2.id]
        }
        sendChannels[i] = channels[from][to]
        receiveChannels[i] = channels[to][from]
    }

    if a.initTerm != nil {
        fromInit := newBasicChannel(true)()
        toInit := newBasicChannel(true)()
        sendChannels = append(sendChannels, toInit)
        receiveChannels = append(receiveChannels, fromInit)
        initAbstr := a.initTerm.evaluate(ctx)
        evaluateAbstractionExpr(initAbstr, ctx, []communication.Channel[expression]{fromInit}, []communication.Channel[expression]{toInit})
    }

    abstr := a.abstrTerm.evaluate(ctx)
    evaluateAbstractionExpr(abstr, ctx, sendChannels, receiveChannels)
}

func (a *application) prettyPrint(iw util.IndentedWriter) {
    a.abstrTerm.prettyPrint(iw)
    if a.initTerm != nil {
        iw.Print(" {")
        a.initTerm.prettyPrint(iw)
        iw.Print("}")
    }
    for _, argument := range a.arguments {
        iw.Print(" ")
        argument.prettyPrint(iw)
    }
}

func (a *application) String() string {
    s := a.abstrTerm.String()
    if a.initTerm != nil {
        s += " {" + a.initTerm.String() + "} "
    }
    for _, argument := range a.arguments {
        s += " " + argument.String()
    }
    return s
}
