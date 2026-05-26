// Code generated from ./ast/parser/sessions.g4 by ANTLR 4.13.2. DO NOT EDIT.

package parser // sessions

import "github.com/antlr4-go/antlr/v4"

type BasesessionsVisitor struct {
	*antlr.BaseParseTreeVisitor
}

func (v *BasesessionsVisitor) VisitModule(ctx *ModuleContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitImports(ctx *ImportsContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitImportModule(ctx *ImportModuleContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitPath(ctx *PathContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitDeclaration(ctx *DeclarationContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitDeclarationSugar(ctx *DeclarationSugarContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitName(ctx *NameContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitParticipant(ctx *ParticipantContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitVariableDef(ctx *VariableDefContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitVariable(ctx *VariableContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitApplication(ctx *ApplicationContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitAbstraction(ctx *AbstractionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitConcurrent(ctx *ConcurrentContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitAcceptProc(ctx *AcceptProcContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitRequest(ctx *RequestContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitRecursion(ctx *RecursionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitProcess(ctx *ProcessContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitBlockProc(ctx *BlockProcContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitSequenceProc(ctx *SequenceProcContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitPrefix(ctx *PrefixContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitReceive(ctx *ReceiveContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitSend(ctx *SendContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitOut(ctx *OutContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitInp(ctx *InpContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitLet(ctx *LetContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitCloseChannel(ctx *CloseChannelContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitCall(ctx *CallContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitIfThenElseProc(ctx *IfThenElseProcContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitSelectProc(ctx *SelectProcContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitBranchProc(ctx *BranchProcContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitLabelExpression(ctx *LabelExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitTerminateProc(ctx *TerminateProcContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitLogicalOr(ctx *LogicalOrContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitLogicalAnd(ctx *LogicalAndContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitEqualityExpr(ctx *EqualityExprContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitEqop(ctx *EqopContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitRelationalExpr(ctx *RelationalExprContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitRelop(ctx *RelopContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitSumExpr(ctx *SumExprContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitSumop(ctx *SumopContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitMultExpr(ctx *MultExprContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitMulop(ctx *MulopContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitUnary(ctx *UnaryContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitTerm(ctx *TermContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitLiteral(ctx *LiteralContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitListExpr(ctx *ListExprContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitListSlice(ctx *ListSliceContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitConditionalExpr(ctx *ConditionalExprContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitRecord(ctx *RecordContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitChannel(ctx *ChannelContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitRename(ctx *RenameContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitMacro(ctx *MacroContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitPrefixMacro(ctx *PrefixMacroContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitSendMacro(ctx *SendMacroContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitReceiveMacro(ctx *ReceiveMacroContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitPrimaryTerm(ctx *PrimaryTermContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitPrimaryList(ctx *PrimaryListContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitPrimaryRecord(ctx *PrimaryRecordContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitType(ctx *TypeContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitPrimitiveType(ctx *PrimitiveTypeContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitNameType(ctx *NameTypeContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitIoType(ctx *IoTypeContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitAbstractionType(ctx *AbstractionTypeContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitSession(ctx *SessionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitConfiguration(ctx *ConfigurationContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitRecordType(ctx *RecordTypeContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitChannelType(ctx *ChannelTypeContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitAbstractionTypeDef(ctx *AbstractionTypeDefContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitConfigurationDef(ctx *ConfigurationDefContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitLocalSet(ctx *LocalSetContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitProjection(ctx *ProjectionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitLocalAbstraction(ctx *LocalAbstractionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitLabelType(ctx *LabelTypeContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitLocal(ctx *LocalContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitSendLocal(ctx *SendLocalContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitReceiveLocal(ctx *ReceiveLocalContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitSelectLocal(ctx *SelectLocalContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitBranchLocal(ctx *BranchLocalContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitLabelLocal(ctx *LabelLocalContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitLocalContext(ctx *LocalContextContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitGlobalDef(ctx *GlobalDefContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitGlobal(ctx *GlobalContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitPass(ctx *PassContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitChoice(ctx *ChoiceContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BasesessionsVisitor) VisitLabelGlobal(ctx *LabelGlobalContext) interface{} {
	return v.VisitChildren(ctx)
}
