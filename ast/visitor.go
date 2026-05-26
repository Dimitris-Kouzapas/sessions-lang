package ast

import (
    "sessions/ast/parser"
)

type sessionsVisitor struct {
    parser.BasesessionsVisitor
}

func (sv *sessionsVisitor) VisitModule(ctx *parser.ModuleContext) *module {
    imps := sv.VisitImports(ctx.Imports().(*parser.ImportsContext))
    mod := newModule(imps, 0)

    for _, decl := range ctx.AllDeclaration() {
        d := sv.VisitDeclaration(decl.(*parser.DeclarationContext))
        mod.addDeclaration(d)
    }
    return mod
}

func (sv *sessionsVisitor) VisitImports(ctx *parser.ImportsContext) []*importModule {
    imports := make([]*importModule, len(ctx.AllImportModule()))
    for i, p := range ctx.AllImportModule() {
         imports[i] = sv.VisitImportModule(p.(*parser.ImportModuleContext))
    }
    return imports
}

func (sv *sessionsVisitor) VisitImportModule(ctx *parser.ImportModuleContext) *importModule {
    path := sv.VisitPath(ctx.Path().(*parser.PathContext))
    var name string
    if ctx.AS() != nil {
        name = ctx.ID().GetText()
    } else {
        name = path
    }
    line := ctx.IMPORT().GetSymbol().GetLine()
    return newImportModule(path, name, line)
}

func (v *sessionsVisitor) VisitPath(ctx *parser.PathContext) string {
    path := ""
    for i, id := range ctx.AllID() {
        if i != 0 {
            path += "."
        }
        path += id.GetText()
    }
    return path
}

func (sv *sessionsVisitor) VisitDeclaration(ctx *parser.DeclarationContext) declaration {
    if ctx.DeclarationSugar() != nil {
        return sv.VisitDeclarationSugar(ctx.DeclarationSugar().(*parser.DeclarationSugarContext))
    }

    id, _ := sv.VisitName(ctx.Name().(*parser.NameContext))
    if ctx.VAL() != nil {
        line := ctx.VAL().GetSymbol().GetLine()
        if ctx.Abstraction() != nil {
            abstr := sv.VisitAbstraction(ctx.Abstraction().(*parser.AbstractionContext))
            return newAbstractionDeclaration(id, abstr, line)
        } else if ctx.PrimaryTerm() != nil {
            term := sv.VisitPrimaryTerm(ctx.PrimaryTerm().(*parser.PrimaryTermContext))
            return newValueDeclaration(id, term, line)
        }
    } else if ctx.TYPE() != nil {
        line := ctx.TYPE().GetSymbol().GetLine()
        var tdef typedef
        if ctx.AbstractionType() != nil {
            tdef = sv.VisitAbstractionType(ctx.AbstractionType().(*parser.AbstractionTypeContext))
        } else if ctx.Configuration() != nil {
            tdef = sv.VisitConfiguration(ctx.Configuration().(*parser.ConfigurationContext))
        } else if ctx.RecordType() != nil {
            tdef = sv.VisitRecordType(ctx.RecordType().(*parser.RecordTypeContext))
        } else if ctx.ChannelType() != nil {
            tdef = sv.VisitChannelType(ctx.ChannelType().(*parser.ChannelTypeContext))
        }
        return newTypeDeclaration(id, tdef, line)
    } else if ctx.COLON() != nil {
        line := ctx.COLON().GetSymbol().GetLine()
        tdef := sv.VisitAbstractionTypeDef(ctx.AbstractionTypeDef().(*parser.AbstractionTypeDefContext))
        return newAbstractionTypeAssignment(id, tdef, line)
    }
    return nil
}

func (sv *sessionsVisitor) VisitDeclarationSugar(ctx *parser.DeclarationSugarContext) declaration {
    id, _ := sv.VisitName(ctx.Name().(*parser.NameContext))
    if ctx.PROC() != nil {
         line := ctx.PROC().GetSymbol().GetLine()
        proc := sv.VisitProcess(ctx.Process().(*parser.ProcessContext))

        var initParameters []*participantExpr = nil //make([]*participantExpr, 0)
        initParametersNum := 0
        if ctx.LBRA() != nil {
            initParametersNum = 1
            ipar := sv.VisitParticipant(ctx.Participant(0).(*parser.ParticipantContext))
            initParameters = append(initParameters, ipar)
        } else if ctx.ReceiveMacro() != nil {
            exprs, line := sv.VisitReceiveMacro(ctx.ReceiveMacro().(*parser.ReceiveMacroContext))
            initParameters, proc = newSeqProcMacro(exprs, proc, line)
        }
        parameters := make([]*participantExpr, len(ctx.AllParticipant()) - initParametersNum)
        for i := initParametersNum; i < len(ctx.AllParticipant()); i++ {
            parameters[i - initParametersNum] = sv.VisitParticipant(ctx.Participant(i).(*parser.ParticipantContext))
        }

        abstr := newAbstraction(initParameters, parameters, proc, line)
        return newAbstractionDeclaration(id, abstr, line)
    } else if ctx.RECORD() != nil {
        line := ctx.RECORD().GetSymbol().GetLine()
        rec := sv.VisitPrimaryRecord(ctx.PrimaryRecord().(*parser.PrimaryRecordContext))
        return newValueDeclaration(id, rec, line)
    } else if ctx.SHARED() != nil {
        line := ctx.SHARED().GetSymbol().GetLine()
        shChannel := sv.VisitChannel(ctx.Channel().(*parser.ChannelContext))
        return newValueDeclaration(id, shChannel, line)
    }
    return nil
}

func (sv *sessionsVisitor) VisitName(ctx *parser.NameContext) (string, int) {
    return ctx.ID().GetText(), ctx.ID().GetSymbol().GetLine()
}

func (sv *sessionsVisitor) VisitParticipant(ctx *parser.ParticipantContext) *participantExpr {
    return newParticipantExpr(ctx.ID().GetText(), ctx.ID().GetSymbol().GetLine())
}

func (sv *sessionsVisitor) VisitVariableDef(ctx *parser.VariableDefContext) *variableExpr {
    id, line := sv.VisitVariable(ctx.Variable().(*parser.VariableContext))
    tdef := sv.VisitType(ctx.Type_().(*parser.TypeContext))
    return newVariableExpr(id, tdef, line)
}

//TODO redundant <- same as VisitName
func (sv *sessionsVisitor) VisitVariable(ctx *parser.VariableContext) (string, int) {
    return ctx.ID().GetText(), ctx.ID().GetSymbol().GetLine()
}

func (sv *sessionsVisitor) VisitAbstraction(ctx *parser.AbstractionContext) *abstraction {
    line := ctx.PROC().GetSymbol().GetLine()
    proc := sv.VisitProcess(ctx.Process().(*parser.ProcessContext))

    var initParameters []*participantExpr = nil //make([]*participantExpr, 0)
    initParametersNum := 0
    if ctx.LBRA() != nil {
        initParametersNum = 1
        ipar := sv.VisitParticipant(ctx.Participant(0).(*parser.ParticipantContext))
        initParameters = append(initParameters, ipar)
    } else if ctx.ReceiveMacro() != nil {
        exprs, line := sv.VisitReceiveMacro(ctx.ReceiveMacro().(*parser.ReceiveMacroContext))
        initParameters, proc = newSeqProcMacro(exprs, proc, line)
    }
    parameters := make([]*participantExpr, len(ctx.AllParticipant()) - initParametersNum)
    for i := initParametersNum; i < len(ctx.AllParticipant()); i++ {
        parameters[i - initParametersNum] = sv.VisitParticipant(ctx.Participant(i).(*parser.ParticipantContext))
    }

    return newAbstraction(initParameters, parameters, proc, line)
}

func (sv *sessionsVisitor) VisitApplication(ctx *parser.ApplicationContext) *application {
    abstrTerm := sv.VisitTerm(ctx.Term(0).(*parser.TermContext))
    line := ctx.Term(0).GetStart().GetLine()

    args := make([]*participantExpr, len(ctx.AllParticipant()))
    for i, a := range ctx.AllParticipant() {
        args[i] = sv.VisitParticipant(a.(*parser.ParticipantContext))
    }

    var initTerm expression
    if ctx.SendMacro() != nil {
        exprs, line := sv.VisitSendMacro(ctx.SendMacro().(*parser.SendMacroContext))
        initParameters, body := newSeqProcMacro(exprs, newTerminate(line), line)
        initTerm = newAbstraction(initParameters, nil, body, line) //newSeqProcMacro(exprs, newTerminate(line), line)
    } else if ctx.LBRA() != nil {
        initTerm = sv.VisitTerm(ctx.Term(1).(*parser.TermContext))
    }

    return newApplication(abstrTerm, args, initTerm, line)
}

func (sv *sessionsVisitor) VisitConcurrent(ctx *parser.ConcurrentContext) (*participantExpr, []*participantExpr, []*application, int) {
    var participant *participantExpr = nil
    if ctx.Name() != nil {
        id, line := sv.VisitName(ctx.Name().(*parser.NameContext))
        participant = newParticipantExpr(id, line)
    }
    participants := make([]*participantExpr, len(ctx.AllApplication()))
    applications := make([]*application, len(ctx.AllApplication()))
    for i, r := range ctx.AllApplication() {
        participants[i] = sv.VisitParticipant(ctx.Participant(i).(*parser.ParticipantContext))
        applications[i] = sv.VisitApplication(r.(*parser.ApplicationContext))
    }
    line := ctx.CONC().GetSymbol().GetLine()
    return participant, participants, applications, line
}

func (sv *sessionsVisitor) VisitRequest(ctx *parser.RequestContext) (expression, *participantExpr,*application, int) {
    channel := sv.VisitTerm(ctx.Term().(*parser.TermContext))
    participant := sv.VisitParticipant(ctx.Participant().(*parser.ParticipantContext))
    var app *application
    if ctx.CURRENT() == nil {
        app = sv.VisitApplication(ctx.Application().(*parser.ApplicationContext))
    }
    line := ctx.REQ().GetSymbol().GetLine()
    return channel, participant, app, line
}

func (sv *sessionsVisitor) VisitAcceptProc(ctx *parser.AcceptProcContext) (expression, *participantExpr, *application, int) {
    channel := sv.VisitTerm(ctx.Term().(*parser.TermContext))
    participant := sv.VisitParticipant(ctx.Participant().(*parser.ParticipantContext))
    app := sv.VisitApplication(ctx.Application().(*parser.ApplicationContext))
    line := ctx.ACC().GetSymbol().GetLine()
    return channel, participant, app, line
}

func (sv *sessionsVisitor) VisitRecursion(ctx *parser.RecursionContext) *recursionProc {
    app := sv.VisitApplication(ctx.Application().(*parser.ApplicationContext))
    line := ctx.REC().GetSymbol().GetLine()
    var b expression
    var participant *participantExpr
    if ctx.Term() != nil {
        b = sv.VisitTerm(ctx.Term().(*parser.TermContext))
        participant = sv.VisitParticipant(ctx.Participant().(*parser.ParticipantContext))
    }
    return newRecursionProc(app, b, participant, line)
}

func (sv *sessionsVisitor) VisitProcess(ctx *parser.ProcessContext) process {
    if ctx.BlockProc() != nil {
        return sv.VisitBlockProc(ctx.BlockProc().(*parser.BlockProcContext))
    } else if ctx.SequenceProc() != nil {
        return sv.VisitSequenceProc(ctx.SequenceProc().(*parser.SequenceProcContext))
    } else if ctx.IfThenElseProc() != nil {
        return sv.VisitIfThenElseProc(ctx.IfThenElseProc().(*parser.IfThenElseProcContext))
    } else if ctx.SelectProc() != nil {
        return sv.VisitSelectProc(ctx.SelectProc().(*parser.SelectProcContext))
    } else if ctx.BranchProc() != nil {
        return sv.VisitBranchProc(ctx.BranchProc().(*parser.BranchProcContext))
    } else if ctx.Recursion() != nil {
        return sv.VisitRecursion(ctx.Recursion().(*parser.RecursionContext))
    } else if ctx.TerminateProc() != nil {
        return sv.VisitTerminateProc(ctx.TerminateProc().(*parser.TerminateProcContext))
    }
    return nil
}

func (sv *sessionsVisitor) VisitBlockProc(ctx *parser.BlockProcContext) process {
    return sv.VisitProcess(ctx.Process().(*parser.ProcessContext))
}

func (sv *sessionsVisitor) VisitSequenceProc(ctx *parser.SequenceProcContext) process {
    var proc process = nil
    if ctx.Process() != nil {
        proc = sv.VisitProcess(ctx.Process().(*parser.ProcessContext))
    }
    if ctx.Prefix() != nil {
        expr, line := sv.VisitPrefix(ctx.Prefix().(*parser.PrefixContext))
        if proc == nil {
            proc = newTerminate(line)
        }
        return newSequentialProc(expr, proc, line)
    } else if ctx.Concurrent() != nil {
        participant, participants, applications, line := sv.VisitConcurrent(ctx.Concurrent().(*parser.ConcurrentContext))
        if proc == nil {
            proc = newTerminate(line)
        }
        return newConcurrentProc(participant, participants, applications, proc, line)
    } else if ctx.Request() != nil {
        // channel, participant, participants, applications, line := sv.VisitRequest(ctx.Request().(*parser.RequestContext))
        channel, participant, app, line := sv.VisitRequest(ctx.Request().(*parser.RequestContext))
        if proc == nil {
            proc = newTerminate(line)
        }
        return newRequestProc(channel, participant, app, proc, line)
    } else if ctx.AcceptProc() != nil {
        channel, participant, app, line := sv.VisitAcceptProc(ctx.AcceptProc().(*parser.AcceptProcContext))
        if proc == nil {
            proc = newTerminate(line)
        }
        return newAcceptProc(channel, participant, app, proc, line)
    } else if ctx.Call() != nil {
        exprs, adef, variables, line:= sv.VisitCall(ctx.Call().(*parser.CallContext))
        if proc == nil {
            proc = newTerminate(line)
        }
        return newCall(exprs, adef, variables, proc, line)
    }

    return nil
}

func (sv *sessionsVisitor) VisitPrefix(ctx *parser.PrefixContext) (seqExpr, int) {
    line := 0
    if ctx.Receive() != nil {
        line = ctx.Receive().GetStart().GetLine()
        return sv.VisitReceive(ctx.Receive().(*parser.ReceiveContext)), line
    } else if ctx.Let() != nil {
        line = ctx.Let().GetStart().GetLine()
        return sv.VisitLet(ctx.Let().(*parser.LetContext)), line
    } else if ctx.Out() != nil {
        line = ctx.Out().GetStart().GetLine()
        return sv.VisitOut(ctx.Out().(*parser.OutContext)), line
    } else if ctx.Inp() != nil {
        line = ctx.Inp().GetStart().GetLine()
        return sv.VisitInp(ctx.Inp().(*parser.InpContext)), line
    } else if ctx.CloseChannel() != nil {
        line := ctx.CloseChannel().GetStart().GetLine()
        return sv.VisitCloseChannel(ctx.CloseChannel().(*parser.CloseChannelContext)), line
    }
    return nil, line
}

func (sv *sessionsVisitor) VisitLet(ctx *parser.LetContext) *letExpr {
    id, line := sv.VisitVariable(ctx.Variable().(*parser.VariableContext))
    var tdef typedef = nil
    if ctx.Type_() != nil {
        tdef = sv.VisitType(ctx.Type_().(*parser.TypeContext))
    }
    variable := newVariableExpr(id, tdef, line)
    expr := sv.VisitLogicalOr(ctx.LogicalOr().(*parser.LogicalOrContext))
    line = ctx.LET().GetSymbol().GetLine()
    return newLetExpr(variable, expr, line)
}

func (sv *sessionsVisitor) VisitCloseChannel(ctx *parser.CloseChannelContext) *closeExpr {
    expr := sv.VisitTerm(ctx.Term().(*parser.TermContext))
    line := ctx.ARROW().GetSymbol().GetLine()
    return newCloseExpr(expr, line)
}

func (sv *sessionsVisitor) VisitIfThenElseProc(ctx *parser.IfThenElseProcContext) process {
    cond := sv.VisitLogicalOr(ctx.LogicalOr().(*parser.LogicalOrContext))
    thenProc := sv.VisitProcess(ctx.Process(0).(*parser.ProcessContext))
    elseProc := sv.VisitProcess(ctx.Process(1).(*parser.ProcessContext))
    line := ctx.IF().GetSymbol().GetLine()
    return newIfThenElseProc(cond, thenProc, elseProc, line)
}

func (sv *sessionsVisitor) VisitSelectProc(ctx *parser.SelectProcContext) process {
    line := 0
    participant := sv.VisitParticipant(ctx.Participant().(*parser.ParticipantContext))
    conds := make([]expression, len(ctx.AllLogicalOr()))
    labels := make([]*labelExpr, len(ctx.AllLabelExpression()))
    processes := make([]process, len(ctx.AllProcess()))
    if ctx.SELECT() != nil {
        line = ctx.SELECT().GetSymbol().GetLine()
        for i := range ctx.AllLogicalOr() {
            conds[i] = sv.VisitLogicalOr(ctx.LogicalOr(i).(*parser.LogicalOrContext))
            labels[i] = sv.VisitLabelExpression(ctx.LabelExpression(i).(*parser.LabelExpressionContext))
            processes[i] = sv.VisitProcess(ctx.Process(i).(*parser.ProcessContext))
        }
        psize := len(ctx.AllProcess()) - 1
        labels[psize] = sv.VisitLabelExpression(ctx.LabelExpression(psize).(*parser.LabelExpressionContext))
        processes[psize] = sv.VisitProcess(ctx.Process(psize).(*parser.ProcessContext))
    } else /*if ctx.ARROW() != nil*/ {
        line = ctx.ARROW(0).GetSymbol().GetLine()
        labels[0] = sv.VisitLabelExpression(ctx.LabelExpression(0).(*parser.LabelExpressionContext))
        if len(ctx.AllProcess()) != 0 {
            processes[0] = sv.VisitProcess(ctx.Process(0).(*parser.ProcessContext))
        } else {
            processes = append(processes, newTerminate(line))
        }
    }
    return newSelectProc(participant, conds, labels, processes, line)
}

func (sv *sessionsVisitor) VisitBranchProc(ctx *parser.BranchProcContext) process {
    participant := sv.VisitParticipant(ctx.Participant().(*parser.ParticipantContext))

    labels := make([]*labelExpr, len(ctx.AllLabelExpression()))
    processes := make([]process, len(ctx.AllProcess()))
    for i, p := range ctx.AllProcess() {
        labels[i] = sv.VisitLabelExpression(ctx.LabelExpression(i).(*parser.LabelExpressionContext))
        processes[i] = sv.VisitProcess(p.(*parser.ProcessContext))
    }
    line := ctx.BRANCH().GetSymbol().GetLine()
    return newBranchProc(participant, labels, processes, line)
}

func (sv *sessionsVisitor) VisitTerminateProc(ctx *parser.TerminateProcContext) *terminate {
    return newTerminate(ctx.TERMINATE().GetSymbol().GetLine())
}

func (sv *sessionsVisitor) VisitReceive(ctx *parser.ReceiveContext) seqExpr {
    var expr expression
    if ctx.Send() != nil {
        expr = sv.VisitSend(ctx.Send().(*parser.SendContext))
    } else if ctx.Participant() != nil {
        expr = sv.VisitParticipant(ctx.Participant().(*parser.ParticipantContext))
    }
    if ctx.ARROW() == nil {
        return expr.(seqExpr)
    }
    line  := ctx.ARROW().GetSymbol().GetLine()
    variable := sv.VisitVariableDef(ctx.VariableDef().(*parser.VariableDefContext))
    return newReceiveExpr(variable, expr, line)
}

func (sv *sessionsVisitor) VisitSend(ctx *parser.SendContext) seqExpr {
    var lexpr seqExpr = nil
    if ctx.Receive() != nil {
        lexpr = sv.VisitReceive(ctx.Receive().(*parser.ReceiveContext))
    }
    for i, lor := range ctx.AllLogicalOr() {
        line := ctx.ARROW(i).GetSymbol().GetLine()
        rexpr := sv.VisitLogicalOr(lor.(*parser.LogicalOrContext))
        if i == 0 && ctx.Participant() != nil {
            participant := sv.VisitParticipant(ctx.Participant().(*parser.ParticipantContext))
            lexpr = newSendExpr(participant, rexpr, line)
        } else {
            lexpr = newSendExpr(lexpr, rexpr, line)
        }
    }
    return lexpr
}

func (sv *sessionsVisitor) VisitOut(ctx *parser.OutContext) *outExpr {
    var ioConfig expression = nil
    if ctx.Term() != nil {
        ioConfig = sv.VisitTerm(ctx.Term().(*parser.TermContext))
    }
    line := ctx.OUT().GetSymbol().GetLine()
    exprs := make([]expression, len(ctx.AllLogicalOr()))
    for i, lor := range ctx.AllLogicalOr() {
        exprs[i] = sv.VisitLogicalOr(lor.(*parser.LogicalOrContext))
    }
    return newOutExpr(ioConfig, exprs, line)
}

func (sv *sessionsVisitor) VisitInp(ctx *parser.InpContext) *inpExpr {
    var ioConfig expression = nil
    if ctx.Term() != nil {
        ioConfig = sv.VisitTerm(ctx.Term().(*parser.TermContext))
    }

    line := ctx.ARROW().GetSymbol().GetLine()
    variable := sv.VisitVariableDef(ctx.VariableDef().(*parser.VariableDefContext))
    return newInpExpr(ioConfig, variable, line)
}

func (sv *sessionsVisitor) VisitCall(ctx *parser.CallContext) ([]expression, expression, []*variableExpr, int) {
    exprs := make([]expression, len(ctx.AllLogicalOr()))
    for i, expr := range ctx.AllLogicalOr() {
        exprs[i] = sv.VisitLogicalOr(expr.(*parser.LogicalOrContext))
    }

    adef := sv.VisitTerm(ctx.Term().(*parser.TermContext))

    variables := make([]*variableExpr, len(ctx.AllVariableDef()))
    for i, variable := range ctx.AllVariableDef() {
        variables[i] = sv.VisitVariableDef(variable.(*parser.VariableDefContext))
    }

    line := ctx.EQUALS().GetSymbol().GetLine()
    return exprs, adef, variables, line
}

func (sv *sessionsVisitor) VisitLogicalOr(ctx *parser.LogicalOrContext) expression {
    var lexpr expression = nil
    line := 0
    if ctx.LogicalOr() != nil {
        lexpr = sv.VisitLogicalOr(ctx.LogicalOr().(*parser.LogicalOrContext))
        line = ctx.LOR().GetSymbol().GetLine()
    }
    rexpr := sv.VisitLogicalAnd(ctx.LogicalAnd().(*parser.LogicalAndContext))
    if lexpr != nil {
        return newLogicalExpr(lexpr, rexpr, ctx.LOR().GetText(), line)
    }
    return rexpr
}

func (sv *sessionsVisitor) VisitLogicalAnd(ctx *parser.LogicalAndContext) expression {
    var lexpr expression = nil
    line := 0
    if ctx.LogicalAnd() != nil {
        lexpr = sv.VisitLogicalAnd(ctx.LogicalAnd().(*parser.LogicalAndContext))
        line = ctx.LAND().GetSymbol().GetLine()
    }
    rexpr := sv.VisitEqualityExpr(ctx.EqualityExpr().(*parser.EqualityExprContext))
    if lexpr != nil {
        return newLogicalExpr(lexpr, rexpr, ctx.LAND().GetText(), line)
    }
    return rexpr
}

func (sv *sessionsVisitor) VisitEqualityExpr(ctx *parser.EqualityExprContext) expression {
    var lexpr expression = nil
    var op string
    var line int
    if ctx.EqualityExpr() != nil {
        lexpr = sv.VisitEqualityExpr(ctx.EqualityExpr().(*parser.EqualityExprContext))
        op, line = sv.VisitEqop(ctx.Eqop().(*parser.EqopContext))
    }
    rexpr := sv.VisitRelationalExpr(ctx.RelationalExpr().(*parser.RelationalExprContext))
    if lexpr != nil {
        return newEqualityExpr(lexpr, rexpr, op, line)
    }
    return rexpr
}

func (sv *sessionsVisitor) VisitEqop(ctx *parser.EqopContext) (string, int) {
    var op string
    var line int
    if ctx.EQ() != nil {
        op, line = ctx.EQ().GetText(), ctx.EQ().GetSymbol().GetLine()
    } else if ctx.NEQ() != nil {
        op, line = ctx.NEQ().GetText(), ctx.NEQ().GetSymbol().GetLine()
    }
    return op, line
}

func (sv *sessionsVisitor) VisitRelationalExpr(ctx *parser.RelationalExprContext) expression {
    var lexpr expression = nil
    var op string
    var line int
    if ctx.RelationalExpr() != nil {
        lexpr = sv.VisitRelationalExpr(ctx.RelationalExpr().(*parser.RelationalExprContext))
        op, line = sv.VisitRelop(ctx.Relop().(*parser.RelopContext))
    }
    rexpr := sv.VisitSumExpr(ctx.SumExpr().(*parser.SumExprContext))
    if lexpr != nil {
        return newRelationalExpr(lexpr, rexpr, op, line)
    }
    return rexpr
}

func (sv *sessionsVisitor) VisitRelop(ctx *parser.RelopContext) (string, int) {
    var op string
    var line int
    if ctx.LT() != nil {
        op, line = ctx.LT().GetText(), ctx.LT().GetSymbol().GetLine()
    } else if ctx.GT() != nil {
        op, line = ctx.GT().GetText(), ctx.GT().GetSymbol().GetLine()
    } else if ctx.LTE() != nil {
        op, line = ctx.LTE().GetText(), ctx.LTE().GetSymbol().GetLine()
    } else if ctx.GTE() != nil {
        op, line = ctx.GTE().GetText(), ctx.GTE().GetSymbol().GetLine()
    }
    return op, line
}

func (sv *sessionsVisitor) VisitSumExpr(ctx *parser.SumExprContext) expression {
    var lexpr expression = nil
    var op string
    var line int
    if ctx.SumExpr() != nil {
        lexpr = sv.VisitSumExpr(ctx.SumExpr().(*parser.SumExprContext))
        op, line = sv.VisitSumop(ctx.Sumop().(*parser.SumopContext))
    }
    rexpr := sv.VisitMultExpr(ctx.MultExpr().(*parser.MultExprContext))
    if lexpr != nil {
        return newSumExpr(lexpr, rexpr, op, line)
    }
    return rexpr
}

func (sv *sessionsVisitor) VisitSumop(ctx *parser.SumopContext) (string, int) {
    var op string
    var line int
    if ctx.PLUS() != nil {
        op, line = ctx.PLUS().GetText(), ctx.PLUS().GetSymbol().GetLine()
    } else if ctx.MINUS() != nil {
        op, line = ctx.MINUS().GetText(), ctx.MINUS().GetSymbol().GetLine()
    }
    return op, line
}

func (sv *sessionsVisitor) VisitMultExpr(ctx *parser.MultExprContext) expression {
    var lexpr expression = nil
    var op string
    var line int
    if ctx.MultExpr() != nil {
        lexpr = sv.VisitMultExpr(ctx.MultExpr().(*parser.MultExprContext))
        op, line = sv.VisitMulop(ctx.Mulop().(*parser.MulopContext))
    }
    rexpr := sv.VisitUnary(ctx.Unary().(*parser.UnaryContext))
    if lexpr != nil {
        return newMultExpr(lexpr, rexpr, op, line)
    }
    return rexpr
}

func (sv *sessionsVisitor) VisitMulop(ctx *parser.MulopContext) (string, int) {
    var op string
    var line int
    if ctx.MUL() != nil {
        op, line = ctx.MUL().GetText(), ctx.MUL().GetSymbol().GetLine()
    } else if ctx.DIV() != nil {
        op, line = ctx.DIV().GetText(), ctx.DIV().GetSymbol().GetLine()
    } else if ctx.MOD() != nil {
        op, line = ctx.MOD().GetText(), ctx.MOD().GetSymbol().GetLine()
    } else if ctx.POW() != nil {
        op, line = ctx.POW().GetText(), ctx.POW().GetSymbol().GetLine()
    }
    return op, line
}

func (sv *sessionsVisitor) VisitUnary(ctx *parser.UnaryContext) expression {
    if ctx.Unary() != nil {
        lexpr := sv.VisitUnary(ctx.Unary().(*parser.UnaryContext))
        var line int
        if ctx.NOT() != nil {
            line = ctx.NOT().GetSymbol().GetLine()
            return newNotExpr(lexpr, ctx.NOT().GetText(), line)
        } else if ctx.Sumop() != nil {
            var op string
            op, line = sv.VisitSumop(ctx.Sumop().(*parser.SumopContext))
            return newSignExpr(lexpr, op, line)
        }
    }
    return sv.VisitTerm(ctx.Term().(*parser.TermContext))
}

func (sv *sessionsVisitor) VisitTerm(ctx *parser.TermContext) expression {
    if ctx.Literal() != nil {
        return sv.VisitLiteral(ctx.Literal().(*parser.LiteralContext))
    } else if ctx.Variable() != nil {
        id, line := sv.VisitVariable(ctx.Variable().(*parser.VariableContext))
        return newVariableExpr(id, nil, line)
    } else if ctx.ListExpr() != nil {
        return sv.VisitListExpr(ctx.ListExpr().(*parser.ListExprContext))
    } else if ctx.LSQ() != nil {
        term := sv.VisitTerm(ctx.Term().(*parser.TermContext))
        expr := sv.VisitLogicalOr(ctx.LogicalOr().(*parser.LogicalOrContext))
        line := ctx.LSQ().GetSymbol().GetLine()
        return newListAccessExpr(term, expr, line)
    } else if ctx.ListSlice() != nil {
        term := sv.VisitTerm(ctx.Term().(*parser.TermContext))
        lexpr, rexpr := sv.VisitListSlice(ctx.ListSlice().(*parser.ListSliceContext))
        line := ctx.Term().GetStart().GetLine()
        return newListSliceExpr(term, lexpr, rexpr, line)
    } else if ctx.ConditionalExpr() != nil {
        return sv.VisitConditionalExpr(ctx.ConditionalExpr().(*parser.ConditionalExprContext))
    } else if ctx.Abstraction() != nil {
        return sv.VisitAbstraction(ctx.Abstraction().(*parser.AbstractionContext))
    } else if ctx.Record() != nil {
        return sv.VisitRecord(ctx.Record().(*parser.RecordContext))
    } else if ctx.Rename() != nil {
        return sv.VisitRename(ctx.Rename().(*parser.RenameContext))
    } else if ctx.Channel() != nil {
        return sv.VisitChannel(ctx.Channel().(*parser.ChannelContext))
    } else if ctx.DOT() != nil{
        term := sv.VisitTerm(ctx.Term().(*parser.TermContext))
        id := ctx.ID().GetText()
        line := ctx.DOT().GetSymbol().GetLine()
        return newAccessExpr(term, id, line)
    } else if ctx.Macro() != nil {
        exprs, line := sv.VisitMacro(ctx.Macro().(*parser.MacroContext))
        initParameters, body := newSeqProcMacro(exprs, newTerminate(line), line)
        return  newAbstraction(initParameters, nil, body, line)
    } else if ctx.LPAR() != nil {
        return sv.VisitLogicalOr(ctx.LogicalOr().(*parser.LogicalOrContext))
    }
	return nil
}

func (sv *sessionsVisitor) VisitLiteral(ctx *parser.LiteralContext) expression {
    if ctx.INT_LIT() != nil {
        return newIntExpr(ctx.INT_LIT().GetText(), ctx.INT_LIT().GetSymbol().GetLine())
    } else if ctx.FLOAT_LIT() != nil {
        return newFloatExpr(ctx.FLOAT_LIT().GetText(), ctx.FLOAT_LIT().GetSymbol().GetLine())
    } else if ctx.TRUE() != nil {
        return newTrueExpr(ctx.TRUE().GetSymbol().GetLine())
    } else if ctx.FALSE() != nil {
        return newFalseExpr(ctx.FALSE().GetSymbol().GetLine())
    } else if ctx.STRING_LIT() != nil /*&& ctx.NOTHING() == nil*/ {
        str := ctx.STRING_LIT().GetText()
        str = str[1:len(str)-1]
        return newStringExpr(str, ctx.STRING_LIT().GetSymbol().GetLine())
    }
    return nil
}

func (sv *sessionsVisitor) VisitLabelExpression(ctx *parser.LabelExpressionContext) *labelExpr {
    return newLabelExpr(ctx.LABEL().GetText(), ctx.LABEL().GetSymbol().GetLine())
}

func (sv *sessionsVisitor) VisitListExpr(ctx *parser.ListExprContext) expression {
    expressions := make([]expression, len(ctx.AllLogicalOr()))
    for i, lor := range ctx.AllLogicalOr() {
        expressions[i] = sv.VisitLogicalOr(lor.(*parser.LogicalOrContext))
    }
    return newListExpr(expressions, ctx.LSQ().GetSymbol().GetLine())
}

func (sv *sessionsVisitor) VisitListSlice(ctx *parser.ListSliceContext) (expression, expression) {
    var lexpr expression = nil
    var rexpr expression = nil
    if ctx.GetLeft() != nil {
        lexpr = sv.VisitLogicalOr(ctx.GetLeft().(*parser.LogicalOrContext))
    }
    if ctx.GetRight() != nil {
        rexpr = sv.VisitLogicalOr(ctx.GetRight().(*parser.LogicalOrContext))
    }
    return lexpr, rexpr
}

func (sv *sessionsVisitor) VisitConditionalExpr(ctx *parser.ConditionalExprContext) expression {
    cond := sv.VisitLogicalOr(ctx.LogicalOr(0).(*parser.LogicalOrContext))
    thenExpr := sv.VisitLogicalOr(ctx.LogicalOr(1).(*parser.LogicalOrContext))
    elseExpr := sv.VisitLogicalOr(ctx.LogicalOr(2).(*parser.LogicalOrContext))
    line := ctx.IF().GetSymbol().GetLine()
    return newConditionalExpr(cond, thenExpr, elseExpr, line)
}

func (sv *sessionsVisitor) VisitRecord(ctx *parser.RecordContext) expression {
    labels := make([]string, len(ctx.AllID()))
    exprs := make([]expression, len(ctx.AllLogicalOr()))
    for i := range ctx.AllID() {
        labels[i] = ctx.ID(i).GetText()
        exprs[i] = sv.VisitLogicalOr(ctx.LogicalOr(i).(*parser.LogicalOrContext))
    }
    line := ctx.LBRA().GetSymbol().GetLine()
    return newRecordExpr(labels, exprs, line)
}

func (sv *sessionsVisitor) VisitRename(ctx *parser.RenameContext) *renameChannel {
    participants := make([]*participantExpr, 0)
    for _, p := range ctx.AllParticipant() {
        participants = append(participants, sv.VisitParticipant(p.(*parser.ParticipantContext)))
    }
    term := sv.VisitTerm(ctx.Term().(*parser.TermContext))
    line := ctx.LSQ().GetSymbol().GetLine()
    return newRenameChannel(participants, term, line)
}

func (sv sessionsVisitor) VisitChannel(ctx *parser.ChannelContext) sharedChannel {
    if ctx.ASYNC() != nil {
        line := ctx.ASYNC().GetSymbol().GetLine()
        participants := make([]*participantExpr, len(ctx.AllParticipant()))
        for i, p := range ctx.AllParticipant() {
            participants[i] = sv.VisitParticipant(p.(*parser.ParticipantContext))
        }
        return newRecChannel(participants, line)
    }
    line := ctx.SYNC().GetSymbol().GetLine()
    gconfig := sv.VisitConfigurationDef(ctx.ConfigurationDef().(*parser.ConfigurationDefContext))
    requester := sv.VisitParticipant(ctx.Participant(0).(*parser.ParticipantContext))
    return newReqChannel(requester, gconfig, line)
}

func (sv *sessionsVisitor) VisitMacro(ctx *parser.MacroContext) ([]expression, int) {
    exprs := make([]expression, len(ctx.AllPrefixMacro()))
    for i, m := range ctx.AllPrefixMacro() {
        exprs[i] = sv.VisitPrefixMacro(m.(*parser.PrefixMacroContext))
    }
    line := ctx.SEND().GetSymbol().GetLine()
    return exprs, line
}

func (sv *sessionsVisitor) VisitPrefixMacro(ctx *parser.PrefixMacroContext) expression {
    if ctx.LogicalOr() != nil {
        return sv.VisitLogicalOr(ctx.LogicalOr().(*parser.LogicalOrContext))
    } else if ctx.VariableDef() != nil {
        return sv.VisitVariableDef(ctx.VariableDef().(*parser.VariableDefContext))
    }
    return nil
}

func (sv *sessionsVisitor) VisitSendMacro(ctx *parser.SendMacroContext) ([]expression, int) {
    exprs := make([]expression, len(ctx.AllLogicalOr()))
    for i, expr := range ctx.AllLogicalOr() {
        exprs[i] = sv.VisitLogicalOr(expr.(*parser.LogicalOrContext))
    }
    line := ctx.GT().GetSymbol().GetLine()
    return exprs, line
}

func (sv *sessionsVisitor) VisitReceiveMacro(ctx *parser.ReceiveMacroContext) ([]expression, int) {
    exprs := make([]expression, len(ctx.AllVariableDef()))
    for i, expr := range ctx.AllVariableDef() {
        exprs[i] = sv.VisitVariableDef(expr.(*parser.VariableDefContext))
    }
    line := ctx.LPAR().GetSymbol().GetLine()
    return exprs, line
}

func (sv *sessionsVisitor) VisitPrimaryTerm(ctx *parser.PrimaryTermContext) expression {
    if ctx.Literal() != nil {
        return sv.VisitLiteral(ctx.Literal().(*parser.LiteralContext))
    } else if ctx.PrimaryList() != nil {
        return sv.VisitPrimaryList(ctx.PrimaryList().(*parser.PrimaryListContext))
    } else if ctx.PrimaryRecord() != nil {
        return sv.VisitPrimaryRecord(ctx.PrimaryRecord().(*parser.PrimaryRecordContext))
    } else if ctx.Channel() != nil {
        return sv.VisitChannel(ctx.Channel().(*parser.ChannelContext))
    } else if ctx.Macro() != nil {
        exprs, line := sv.VisitMacro(ctx.Macro().(*parser.MacroContext))
        initParameters, body := newSeqProcMacro(exprs, newTerminate(line), line)
        return  newAbstraction(initParameters, nil, body, line)
    }
    return nil
}

func (sv *sessionsVisitor) VisitPrimaryList(ctx *parser.PrimaryListContext) expression {
    expressions := make([]expression, len(ctx.AllPrimaryTerm()))
    for i, pt := range ctx.AllPrimaryTerm() {
        expressions[i] = sv.VisitPrimaryTerm(pt.(*parser.PrimaryTermContext))
    }
    return newListExpr(expressions, ctx.LSQ().GetSymbol().GetLine())
}

func (sv *sessionsVisitor) VisitPrimaryRecord(ctx *parser.PrimaryRecordContext) expression {
    labels := make([]string, len(ctx.AllID()))
    exprs := make([]expression, len(ctx.AllPrimaryTerm()))
    for i := range ctx.AllID() {
        labels[i] = ctx.ID(i).GetText()
        exprs[i] = sv.VisitPrimaryTerm(ctx.PrimaryTerm(i).(*parser.PrimaryTermContext))
    }
    line := ctx.LBRA().GetSymbol().GetLine()
    return newRecordExpr(labels, exprs, line)
}

// Session types

func (sv *sessionsVisitor) VisitLocalAbstraction(ctx *parser.LocalAbstractionContext) *localAbstraction {
    args := make([]*participantType, len(ctx.AllID()))
    for i, arg := range ctx.AllID() {
        args[i] = newParticipantType(arg.GetText(), arg.GetSymbol().GetLine())
    }
    loc := sv.VisitLocal(ctx.Local().(*parser.LocalContext))
    line := ctx.LOCAL().GetSymbol().GetLine()
    return newLocalAbstraction(args, loc, line)
}

func (sv *sessionsVisitor) VisitType(ctx *parser.TypeContext) typedef {
    if ctx.PrimitiveType() != nil {
        return sv.VisitPrimitiveType(ctx.PrimitiveType().(*parser.PrimitiveTypeContext))
    } else if ctx.LSQ() != nil {
        tdef := sv.VisitType(ctx.Type_().(*parser.TypeContext))
        line := ctx.LSQ().GetSymbol().GetLine()
        return newListType(tdef, line)
    } else if ctx.RecordType() != nil {
        return sv.VisitRecordType(ctx.RecordType().(*parser.RecordTypeContext))
    } else if ctx.ChannelType() != nil {
        return sv.VisitChannelType(ctx.ChannelType().(*parser.ChannelTypeContext))
    } else if ctx.AbstractionType() != nil {
        return sv.VisitAbstractionType(ctx.AbstractionType().(*parser.AbstractionTypeContext))
    } else if ctx.NameType() != nil {
        return sv.VisitNameType(ctx.NameType().(*parser.NameTypeContext))
    } else if ctx.IoType() != nil {
        return sv.VisitIoType(ctx.IoType().(*parser.IoTypeContext))
    } else if ctx.Type_() != nil {
        return sv.VisitType(ctx.Type_().(*parser.TypeContext))
    }
    return nil
}

func (sv *sessionsVisitor) VisitPrimitiveType(ctx *parser.PrimitiveTypeContext) typedef {
    if ctx.BOOLEAN() != nil {
        line := ctx.BOOLEAN().GetSymbol().GetLine()
        return newBoolType(line)
    } else if ctx.INT() != nil {
        line := ctx.INT().GetSymbol().GetLine()
        return newIntType(line)
    } else if ctx.FLOAT() != nil {
        line := ctx.FLOAT().GetSymbol().GetLine()
        return newFloatType(line)
    } else if ctx.STRING() != nil {
        line := ctx.STRING().GetSymbol().GetLine()
        return newStringType(line)
    }
    return nil
}

func (sv *sessionsVisitor) VisitNameType(ctx *parser.NameTypeContext) typedef {
    n1, line := sv.VisitName(ctx.Name(0).(*parser.NameContext))
    if ctx.DOT() == nil {
        return newNameType(n1, line)
    }
    line = ctx.DOT().GetSymbol().GetLine()
    n2, _ := sv.VisitName(ctx.Name(1).(*parser.NameContext))
    return newAccessType(n1, n2, line)
}

func (sv *sessionsVisitor) VisitIoType(ctx *parser.IoTypeContext) *ioType {
    line := ctx.IO().GetSymbol().GetLine()
    return newioType(line)
} 

func (sv *sessionsVisitor) VisitRecordType(ctx *parser.RecordTypeContext) *recordType {
    labels := make([]string, len(ctx.AllID()))
    tdefs := make([]typedef, len(ctx.AllType_()))
    for i := range ctx.AllID() {
        labels[i] = ctx.ID(i).GetText()
        tdefs[i] = sv.VisitType(ctx.Type_(i).(*parser.TypeContext))
    }
    line := ctx.RECORD().GetSymbol().GetLine()
    return newRecordType(labels, tdefs, line)
}

func (sv *sessionsVisitor) VisitLocalSet(ctx *parser.LocalSetContext) ([]*participantType, []local, int) {
    participants := make([]*participantType, len(ctx.AllID()))
    for i, p := range ctx.AllID() {
        participants[i] = newParticipantType(p.GetText(), p.GetSymbol().GetLine())
    }
    locals := make([]local, len(ctx.AllLocal()))
    for i, loc := range ctx.AllLocal() {
        locals[i] = sv.VisitLocal(loc.(*parser.LocalContext))
    }
    line := ctx.LBRA().GetSymbol().GetLine()
    return participants, locals, line
}

func (sv *sessionsVisitor) VisitChannelType(ctx *parser.ChannelTypeContext) typedef {
    line := ctx.SHARED().GetSymbol().GetLine()
    if ctx.CHANNEL() == nil {
        participants := make([]*participantType, len(ctx.AllID()))
        for i := range ctx.AllID() {
            id := ctx.ID(i).GetText()
            line := ctx.ID(i).GetSymbol().GetLine()
            participants[i] = newParticipantType(id, line)
        }
        return newRecChannelType(participants, line)
    }
    var gconfig typedef
    if ctx.ConfigurationDef() != nil {
        gconfig = sv.VisitConfigurationDef(ctx.ConfigurationDef().(*parser.ConfigurationDefContext))
    } else if ctx.LocalSet() != nil {
        participants, locals, line := sv.VisitLocalSet(ctx.LocalSet().(*parser.LocalSetContext))
        gconfig = newLocalSet(/*parametres, */participants, locals, line)
    }

    var requester *participantType
    if ctx.REQUESTER() != nil {
        id := ctx.ID(len(ctx.AllID()) - 1).GetText()
        line := ctx.ID(len(ctx.AllID()) - 1).GetSymbol().GetLine()
        requester = newParticipantType(id, line)
    }

    return newReqChannelType(gconfig, requester, line)
}

func (sv *sessionsVisitor) VisitAbstractionType(ctx *parser.AbstractionTypeContext) typedef {
    session := sv.VisitSession(ctx.Session().(*parser.SessionContext))
    if ctx.INIT() != nil {
        args := make([]*participantType, 1)
        args[0] = newParticipantType(ctx.ID().GetText(), ctx.ID().GetSymbol().GetLine())
        loc := sv.VisitLocal(ctx.Local().(*parser.LocalContext))
        line := ctx.INIT().GetSymbol().GetLine()
        initAbstr := newLocalAbstraction(args, loc, line)
        return newAbstractionType(initAbstr, session, line)
    } else {
        line := ctx.Session().GetStart().GetLine()
        args := make([]*participantType, 0)
        loc := newEndLocal(line)
        initAbstr := newLocalAbstraction(args, loc, line)
        return newAbstractionType(initAbstr, session, line)
    }
}

func (sv *sessionsVisitor) VisitSession(ctx *parser.SessionContext) typedef {
    if ctx.LocalAbstraction() != nil {
        return sv.VisitLocalAbstraction(ctx.LocalAbstraction().(*parser.LocalAbstractionContext))
    } else if ctx.Projection() != nil {
        return sv.VisitProjection(ctx.Projection().(*parser.ProjectionContext))
    }
    return nil
}

func (sv *sessionsVisitor) VisitConfiguration(ctx *parser.ConfigurationContext) globalConfig {
    if ctx.GlobalDef() != nil {
        return sv.VisitGlobalDef(ctx.GlobalDef().(*parser.GlobalDefContext))
    } else if ctx.LocalContext() != nil {
        return sv.VisitLocalContext(ctx.LocalContext().(*parser.LocalContextContext))
    }
    return nil
}

func (sv *sessionsVisitor) VisitAbstractionTypeDef(ctx *parser.AbstractionTypeDefContext) typedef {
    if ctx.AbstractionType() != nil {
        return sv.VisitAbstractionType(ctx.AbstractionType().(*parser.AbstractionTypeContext))
    } else if ctx.NameType() != nil {
        return sv.VisitNameType(ctx.NameType().(*parser.NameTypeContext))
    }  
    return nil
}

func (sv *sessionsVisitor) VisitConfigurationDef(ctx *parser.ConfigurationDefContext) typedef {
    if ctx.Configuration() != nil {
        return sv.VisitConfiguration(ctx.Configuration().(*parser.ConfigurationContext))
    } else if ctx.NameType() != nil {
        return sv.VisitNameType(ctx.NameType().(*parser.NameTypeContext))
    }
    return nil
}

func (sv *sessionsVisitor) VisitLocal(ctx *parser.LocalContext) local {
    if ctx.SendLocal() != nil {
        return sv.VisitSendLocal(ctx.SendLocal().(*parser.SendLocalContext))
    } else if ctx.ReceiveLocal() != nil {
        return sv.VisitReceiveLocal(ctx.ReceiveLocal().(*parser.ReceiveLocalContext))
    } else if ctx.SelectLocal() != nil {
        return sv.VisitSelectLocal(ctx.SelectLocal().(*parser.SelectLocalContext))
    } else if ctx.BranchLocal() != nil {
        return sv.VisitBranchLocal(ctx.BranchLocal().(*parser.BranchLocalContext))
    } else if ctx.END() != nil {
        line := ctx.END().GetSymbol().GetLine()
        return newEndLocal(line)
    } else if ctx.REC() != nil {
        line := ctx.REC().GetSymbol().GetLine()
        return newRecLocal(line)
    }
    return nil
}

func (sv *sessionsVisitor) VisitSendLocal(ctx *parser.SendLocalContext) *sendLocal {
    participant := ctx.ID().GetText()
    t := sv.VisitType(ctx.Type_().(*parser.TypeContext))
    line := ctx.SEND().GetSymbol().GetLine()
    var loc local = nil
    if ctx.Local() != nil {
        loc = sv.VisitLocal(ctx.Local().(*parser.LocalContext))
    } else {
        loc = newEndLocal(line)
    }
    ptype := newParticipantType(participant, line)
    return newSendLocal(ptype, t, loc, line)
}

func (sv *sessionsVisitor) VisitReceiveLocal(ctx *parser.ReceiveLocalContext) *receiveLocal {
    participant := ctx.ID().GetText()
    t := sv.VisitType(ctx.Type_().(*parser.TypeContext))
    line := ctx.RECEIVE().GetSymbol().GetLine()
    var loc local = nil
    if ctx.Local() != nil {
        loc = sv.VisitLocal(ctx.Local().(*parser.LocalContext))
    } else {
        loc = newEndLocal(line)
    }
    ptype := newParticipantType(participant, line)
    return newReceiveLocal(ptype, t, loc, line)
}

func (sv *sessionsVisitor) VisitSelectLocal(ctx *parser.SelectLocalContext) *selectLocal {
    participant := ctx.ID().GetText()
    line := ctx.ID().GetSymbol().GetLine()
    ptype := newParticipantType(participant, line)
    labels := make([]*labelType, len(ctx.AllLabelLocal()))
    locals := make([]local, len(ctx.AllLabelLocal()))
    for i, s := range ctx.AllLabelLocal() {
        labels[i], locals[i] = sv.VisitLabelLocal(s.(*parser.LabelLocalContext))
    }
    line = ctx.SELECT().GetSymbol().GetLine()
    return newSelectLocal(ptype, labels, locals, line)
}

func (sv *sessionsVisitor) VisitBranchLocal(ctx *parser.BranchLocalContext) *branchLocal {
    participant := ctx.ID().GetText()
    line := ctx.ID().GetSymbol().GetLine()
    ptype := newParticipantType(participant, line)
    labels := make([]*labelType, len(ctx.AllLabelLocal()))
    locals := make([]local, len(ctx.AllLabelLocal()))
    for i, s := range ctx.AllLabelLocal() {
        labels[i], locals[i] = sv.VisitLabelLocal(s.(*parser.LabelLocalContext))
    }
    line = ctx.BRANCH().GetSymbol().GetLine()
    return newBranchLocal(ptype, labels, locals, line)
}

func (sv *sessionsVisitor) VisitLabelLocal(ctx *parser.LabelLocalContext) (*labelType, local) {
    lab := sv.VisitLabelType(ctx.LabelType().(*parser.LabelTypeContext))
    loc := sv.VisitLocal(ctx.Local().(*parser.LocalContext))
    return lab, loc
}

func (sv *sessionsVisitor) VisitLabelType(ctx *parser.LabelTypeContext) *labelType {
    label := ctx.LABEL().GetText()
    line := ctx.LABEL().GetSymbol().GetLine()
    return newLabelType(label, line)
}

func (sv *sessionsVisitor) VisitLocalContext(ctx *parser.LocalContextContext) *localContext{
    participants, locals, _ := sv.VisitLocalSet(ctx.LocalSet().(*parser.LocalSetContext))
    line := ctx.CONTEXT().GetSymbol().GetLine()
    return newLocalContext(participants, locals, line)
}

func (sv *sessionsVisitor) VisitProjection(ctx *parser.ProjectionContext) *projection {
    conf := sv.VisitConfigurationDef(ctx.ConfigurationDef().(*parser.ConfigurationDefContext))
    id := ctx.ID().GetText()
    line := ctx.ID().GetSymbol().GetLine()
    ptype := newParticipantType(id, line)
    line = ctx.LPAR().GetSymbol().GetLine()
    return newProjection(conf, ptype, line)
}

func (sv *sessionsVisitor) VisitGlobalDef(ctx *parser.GlobalDefContext) *globalDef {
    args := make([]*participantType, len(ctx.AllID()))
    for i, arg := range ctx.AllID() {
        args[i] = newParticipantType(arg.GetText(), arg.GetSymbol().GetLine())
    }
    glob := sv.VisitGlobal(ctx.Global().(*parser.GlobalContext))
    line := ctx.GLOBAL().GetSymbol().GetLine()
    return newGlobalDef(args, glob, line)
}

func (sv *sessionsVisitor) VisitGlobal(ctx *parser.GlobalContext) global {
    if ctx.Pass() != nil {
        return sv.VisitPass(ctx.Pass().(*parser.PassContext))
    } else if ctx.Choice() != nil {
      return sv.VisitChoice(ctx.Choice().(*parser.ChoiceContext))
    } else if ctx.END() != nil {
        line := ctx.END().GetSymbol().GetLine()
        return newEndGlobal(line)
    } else if ctx.REC() != nil {
        line := ctx.REC().GetSymbol().GetLine()
        return newRecGlobal(line)
    } 
    return nil
}

func (sv *sessionsVisitor) VisitPass(ctx *parser.PassContext) global {
    senderID := ctx.ID(0).GetText()
    receiverID := ctx.ID(1).GetText()
    senderLine := ctx.ID(0).GetSymbol().GetLine()
    receiverLine := ctx.ID(1).GetSymbol().GetLine()
    tdef := sv.VisitType(ctx.Type_().(*parser.TypeContext))
    line := ctx.PASS().GetSymbol().GetLine()
    var glob global = nil
    if ctx.DOT() != nil {
        glob = sv.VisitGlobal(ctx.Global().(*parser.GlobalContext))
    } else {
        glob = newEndGlobal(line)
    }
    sender := newParticipantType(senderID, senderLine)
    receiver := newParticipantType(receiverID, receiverLine)
    return newPassGlobal(sender, receiver, tdef, glob, line)
}

func (sv *sessionsVisitor) VisitChoice(ctx *parser.ChoiceContext) global {
    senderID := ctx.ID(0).GetText()
    receiverID := ctx.ID(1).GetText()
    senderLine := ctx.ID(0).GetSymbol().GetLine()
    receiverLine := ctx.ID(1).GetSymbol().GetLine()
    line := ctx.PASS().GetSymbol().GetLine()
    labels := make([]*labelType, len(ctx.AllLabelGlobal()))
    globals := make([]global, len(ctx.AllLabelGlobal()))
    for i, s := range ctx.AllLabelGlobal() {
        labels[i], globals[i] = sv.VisitLabelGlobal(s.(*parser.LabelGlobalContext))
    }
    sender := newParticipantType(senderID, senderLine)
    receiver := newParticipantType(receiverID, receiverLine)
    return newChoiceGlobal(sender, receiver, labels, globals, line)
}

func (sv *sessionsVisitor) VisitLabelGlobal(ctx *parser.LabelGlobalContext) (*labelType, global) {
    lab := sv.VisitLabelType(ctx.LabelType().(*parser.LabelTypeContext))
    glob := sv.VisitGlobal(ctx.Global().(*parser.GlobalContext))
    return lab, glob
}
