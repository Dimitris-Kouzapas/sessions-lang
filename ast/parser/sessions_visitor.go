// Code generated from ./ast/parser/sessions.g4 by ANTLR 4.13.2. DO NOT EDIT.

package parser // sessions

import "github.com/antlr4-go/antlr/v4"

// A complete Visitor for a parse tree produced by sessionsParser.
type sessionsVisitor interface {
	antlr.ParseTreeVisitor

	// Visit a parse tree produced by sessionsParser#module.
	VisitModule(ctx *ModuleContext) interface{}

	// Visit a parse tree produced by sessionsParser#imports.
	VisitImports(ctx *ImportsContext) interface{}

	// Visit a parse tree produced by sessionsParser#importModule.
	VisitImportModule(ctx *ImportModuleContext) interface{}

	// Visit a parse tree produced by sessionsParser#path.
	VisitPath(ctx *PathContext) interface{}

	// Visit a parse tree produced by sessionsParser#declaration.
	VisitDeclaration(ctx *DeclarationContext) interface{}

	// Visit a parse tree produced by sessionsParser#declarationSugar.
	VisitDeclarationSugar(ctx *DeclarationSugarContext) interface{}

	// Visit a parse tree produced by sessionsParser#name.
	VisitName(ctx *NameContext) interface{}

	// Visit a parse tree produced by sessionsParser#participant.
	VisitParticipant(ctx *ParticipantContext) interface{}

	// Visit a parse tree produced by sessionsParser#variableDef.
	VisitVariableDef(ctx *VariableDefContext) interface{}

	// Visit a parse tree produced by sessionsParser#variable.
	VisitVariable(ctx *VariableContext) interface{}

	// Visit a parse tree produced by sessionsParser#application.
	VisitApplication(ctx *ApplicationContext) interface{}

	// Visit a parse tree produced by sessionsParser#abstraction.
	VisitAbstraction(ctx *AbstractionContext) interface{}

	// Visit a parse tree produced by sessionsParser#concurrent.
	VisitConcurrent(ctx *ConcurrentContext) interface{}

	// Visit a parse tree produced by sessionsParser#acceptProc.
	VisitAcceptProc(ctx *AcceptProcContext) interface{}

	// Visit a parse tree produced by sessionsParser#request.
	VisitRequest(ctx *RequestContext) interface{}

	// Visit a parse tree produced by sessionsParser#recursion.
	VisitRecursion(ctx *RecursionContext) interface{}

	// Visit a parse tree produced by sessionsParser#process.
	VisitProcess(ctx *ProcessContext) interface{}

	// Visit a parse tree produced by sessionsParser#blockProc.
	VisitBlockProc(ctx *BlockProcContext) interface{}

	// Visit a parse tree produced by sessionsParser#sequenceProc.
	VisitSequenceProc(ctx *SequenceProcContext) interface{}

	// Visit a parse tree produced by sessionsParser#prefix.
	VisitPrefix(ctx *PrefixContext) interface{}

	// Visit a parse tree produced by sessionsParser#receive.
	VisitReceive(ctx *ReceiveContext) interface{}

	// Visit a parse tree produced by sessionsParser#send.
	VisitSend(ctx *SendContext) interface{}

	// Visit a parse tree produced by sessionsParser#out.
	VisitOut(ctx *OutContext) interface{}

	// Visit a parse tree produced by sessionsParser#inp.
	VisitInp(ctx *InpContext) interface{}

	// Visit a parse tree produced by sessionsParser#let.
	VisitLet(ctx *LetContext) interface{}

	// Visit a parse tree produced by sessionsParser#closeChannel.
	VisitCloseChannel(ctx *CloseChannelContext) interface{}

	// Visit a parse tree produced by sessionsParser#call.
	VisitCall(ctx *CallContext) interface{}

	// Visit a parse tree produced by sessionsParser#ifThenElseProc.
	VisitIfThenElseProc(ctx *IfThenElseProcContext) interface{}

	// Visit a parse tree produced by sessionsParser#selectProc.
	VisitSelectProc(ctx *SelectProcContext) interface{}

	// Visit a parse tree produced by sessionsParser#branchProc.
	VisitBranchProc(ctx *BranchProcContext) interface{}

	// Visit a parse tree produced by sessionsParser#labelExpression.
	VisitLabelExpression(ctx *LabelExpressionContext) interface{}

	// Visit a parse tree produced by sessionsParser#terminateProc.
	VisitTerminateProc(ctx *TerminateProcContext) interface{}

	// Visit a parse tree produced by sessionsParser#logicalOr.
	VisitLogicalOr(ctx *LogicalOrContext) interface{}

	// Visit a parse tree produced by sessionsParser#logicalAnd.
	VisitLogicalAnd(ctx *LogicalAndContext) interface{}

	// Visit a parse tree produced by sessionsParser#equalityExpr.
	VisitEqualityExpr(ctx *EqualityExprContext) interface{}

	// Visit a parse tree produced by sessionsParser#eqop.
	VisitEqop(ctx *EqopContext) interface{}

	// Visit a parse tree produced by sessionsParser#relationalExpr.
	VisitRelationalExpr(ctx *RelationalExprContext) interface{}

	// Visit a parse tree produced by sessionsParser#relop.
	VisitRelop(ctx *RelopContext) interface{}

	// Visit a parse tree produced by sessionsParser#sumExpr.
	VisitSumExpr(ctx *SumExprContext) interface{}

	// Visit a parse tree produced by sessionsParser#sumop.
	VisitSumop(ctx *SumopContext) interface{}

	// Visit a parse tree produced by sessionsParser#multExpr.
	VisitMultExpr(ctx *MultExprContext) interface{}

	// Visit a parse tree produced by sessionsParser#mulop.
	VisitMulop(ctx *MulopContext) interface{}

	// Visit a parse tree produced by sessionsParser#unary.
	VisitUnary(ctx *UnaryContext) interface{}

	// Visit a parse tree produced by sessionsParser#term.
	VisitTerm(ctx *TermContext) interface{}

	// Visit a parse tree produced by sessionsParser#literal.
	VisitLiteral(ctx *LiteralContext) interface{}

	// Visit a parse tree produced by sessionsParser#listExpr.
	VisitListExpr(ctx *ListExprContext) interface{}

	// Visit a parse tree produced by sessionsParser#listSlice.
	VisitListSlice(ctx *ListSliceContext) interface{}

	// Visit a parse tree produced by sessionsParser#conditionalExpr.
	VisitConditionalExpr(ctx *ConditionalExprContext) interface{}

	// Visit a parse tree produced by sessionsParser#record.
	VisitRecord(ctx *RecordContext) interface{}

	// Visit a parse tree produced by sessionsParser#channel.
	VisitChannel(ctx *ChannelContext) interface{}

	// Visit a parse tree produced by sessionsParser#rename.
	VisitRename(ctx *RenameContext) interface{}

	// Visit a parse tree produced by sessionsParser#macro.
	VisitMacro(ctx *MacroContext) interface{}

	// Visit a parse tree produced by sessionsParser#prefixMacro.
	VisitPrefixMacro(ctx *PrefixMacroContext) interface{}

	// Visit a parse tree produced by sessionsParser#sendMacro.
	VisitSendMacro(ctx *SendMacroContext) interface{}

	// Visit a parse tree produced by sessionsParser#receiveMacro.
	VisitReceiveMacro(ctx *ReceiveMacroContext) interface{}

	// Visit a parse tree produced by sessionsParser#primaryTerm.
	VisitPrimaryTerm(ctx *PrimaryTermContext) interface{}

	// Visit a parse tree produced by sessionsParser#primaryList.
	VisitPrimaryList(ctx *PrimaryListContext) interface{}

	// Visit a parse tree produced by sessionsParser#primaryRecord.
	VisitPrimaryRecord(ctx *PrimaryRecordContext) interface{}

	// Visit a parse tree produced by sessionsParser#type.
	VisitType(ctx *TypeContext) interface{}

	// Visit a parse tree produced by sessionsParser#primitiveType.
	VisitPrimitiveType(ctx *PrimitiveTypeContext) interface{}

	// Visit a parse tree produced by sessionsParser#nameType.
	VisitNameType(ctx *NameTypeContext) interface{}

	// Visit a parse tree produced by sessionsParser#ioType.
	VisitIoType(ctx *IoTypeContext) interface{}

	// Visit a parse tree produced by sessionsParser#abstractionType.
	VisitAbstractionType(ctx *AbstractionTypeContext) interface{}

	// Visit a parse tree produced by sessionsParser#session.
	VisitSession(ctx *SessionContext) interface{}

	// Visit a parse tree produced by sessionsParser#configuration.
	VisitConfiguration(ctx *ConfigurationContext) interface{}

	// Visit a parse tree produced by sessionsParser#recordType.
	VisitRecordType(ctx *RecordTypeContext) interface{}

	// Visit a parse tree produced by sessionsParser#channelType.
	VisitChannelType(ctx *ChannelTypeContext) interface{}

	// Visit a parse tree produced by sessionsParser#abstractionTypeDef.
	VisitAbstractionTypeDef(ctx *AbstractionTypeDefContext) interface{}

	// Visit a parse tree produced by sessionsParser#configurationDef.
	VisitConfigurationDef(ctx *ConfigurationDefContext) interface{}

	// Visit a parse tree produced by sessionsParser#localSet.
	VisitLocalSet(ctx *LocalSetContext) interface{}

	// Visit a parse tree produced by sessionsParser#projection.
	VisitProjection(ctx *ProjectionContext) interface{}

	// Visit a parse tree produced by sessionsParser#localAbstraction.
	VisitLocalAbstraction(ctx *LocalAbstractionContext) interface{}

	// Visit a parse tree produced by sessionsParser#labelType.
	VisitLabelType(ctx *LabelTypeContext) interface{}

	// Visit a parse tree produced by sessionsParser#local.
	VisitLocal(ctx *LocalContext) interface{}

	// Visit a parse tree produced by sessionsParser#sendLocal.
	VisitSendLocal(ctx *SendLocalContext) interface{}

	// Visit a parse tree produced by sessionsParser#receiveLocal.
	VisitReceiveLocal(ctx *ReceiveLocalContext) interface{}

	// Visit a parse tree produced by sessionsParser#selectLocal.
	VisitSelectLocal(ctx *SelectLocalContext) interface{}

	// Visit a parse tree produced by sessionsParser#branchLocal.
	VisitBranchLocal(ctx *BranchLocalContext) interface{}

	// Visit a parse tree produced by sessionsParser#labelLocal.
	VisitLabelLocal(ctx *LabelLocalContext) interface{}

	// Visit a parse tree produced by sessionsParser#localContext.
	VisitLocalContext(ctx *LocalContextContext) interface{}

	// Visit a parse tree produced by sessionsParser#globalDef.
	VisitGlobalDef(ctx *GlobalDefContext) interface{}

	// Visit a parse tree produced by sessionsParser#global.
	VisitGlobal(ctx *GlobalContext) interface{}

	// Visit a parse tree produced by sessionsParser#pass.
	VisitPass(ctx *PassContext) interface{}

	// Visit a parse tree produced by sessionsParser#choice.
	VisitChoice(ctx *ChoiceContext) interface{}

	// Visit a parse tree produced by sessionsParser#labelGlobal.
	VisitLabelGlobal(ctx *LabelGlobalContext) interface{}
}
