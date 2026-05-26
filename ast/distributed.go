package ast

import (
	"fmt"
	"os"
	"sessions/ast/util"
)

/******************************************************************************
 * init process
 ******************************************************************************/

type initProc struct {
	baseNode
	channel 		expression
	participant 	*participantExpr
	app             *application
	
	ctype 			*reqChannelType
	gconfig 		globalConfig
	loc 			local

	cont 			process
	symbol 			string	
}

func (ip *initProc) setFilename(filename string) {
	ip.baseNode.setFilename(filename)
	ip.channel.setFilename(filename)
	ip.participant.setFilename(filename)
	if ip.app != nil {
		ip.app.setFilename(filename)
	}
	ip.cont.setFilename(filename)
}

func (ip *initProc) typeCheck(ctx *typeCheckContext, log util.ErrorLog) {
	ip.channel.typeCheck(ctx, log)
	ip.participant.typeCheck(ctx, log)
	if ip.app != nil {
		ip.app.typeCheck(ctx, log)
	}
	ip.cont.typeCheck(ctx, log)
}

func (ip *initProc) projectionCheck(ctx *projectionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
	ip.channel.projectionCheck(ctx, elog, rlog)
	ip.participant.projectionCheck(ctx, elog, rlog)
	if ip.app != nil {
		ip.app.projectionCheck(ctx, elog, rlog)
	}
	ip.cont.projectionCheck(ctx, elog, rlog)
}

func (ip *initProc) channelCheck(ctx *expressionCheckContext, log util.ErrorLog) bool {
	tdef := ip.channel.expressionCheck(ctx, log)
	if tdef == nil {
		return false
	}
	var ok bool
	ip.ctype, ok = tdef.getType().(*reqChannelType)
	if !ok {
		ip.reportErrorf(log, "expected channel type; instead found: %s", tdef.String())
		return ok
	}

	ip.gconfig, ok = ip.ctype.gconfig.getType().(globalConfig)
	if !ok {
		//Not gonna happen
		ip.reportErrorf(log, "exppecting global configuration for channel type; instead found %s", ip.ctype.gconfig.String())
	}

	return ok
}

func (ip *initProc) applicationCheck(ctx *expressionCheckContext, log util.ErrorLog) []*participantExpr {
	//pset := util.NewHashSet[string]()
	participants := make([]*participantExpr, len(ip.gconfig.participants()))
	for i, participant := range ip.gconfig.participants() {
		participants[i] = participant.defaultValue().(*participantExpr)
	}

	if ip.app != nil {
		ip.loc = ip.app.expressionCheck(ip.participant, /*pset, */ nil, ctx, log)
	}
	return participants
}

func (ip *initProc) sessionCheck(ctx *sessionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
	ip.participant.sessionCheck(ctx, elog, rlog)
	if ip.app != nil {
		ip.app.sessionCheck(ctx, elog, rlog)
	}
	locAbstr := ip.gconfig.project(ip.participant.pType())
	if !ip.loc.subtypeOf(locAbstr.loc, util.NewHashSet[typePair]()) {
		stream1 := util.NewStream().Inc().Inc()
		stream1.Print(ip.participant.String() + ": ")
		ip.loc.prettyPrint(stream1)
		stream2 := util.NewStream().Inc().Inc()
		ip.channel.prettyPrint(stream2)
		stream2.Print(": ")
		ip.gconfig.prettyPrint(stream2)
		ip.participant.reportErrorf(elog, "role:\n%v\n\tis not projectable from channel type:\n%v", stream1.String(), stream2.String())
	}
	ip.cont.sessionCheck(ctx, elog, rlog)
}

func (ip *initProc) evaluate(ctx *evaluationContext) (requesterChannel, map[string]string) {
	v := ip.channel.evaluate(ctx)
	if v == nil {
		msg := ip.runtimeErrorf("unknown requester channel: %q.", ip.channel.String())
		fmt.Fprintln(os.Stderr, msg)
		return nil, nil
	}

	switch shChannel := v.(type) {
		case requesterChannel:
			var substitution map[string]string
			return shChannel, substitution
		default:
			msg := ip.runtimeErrorf("encountered a non requester channel: %q.", ip.channel.String())
			fmt.Fprintln(os.Stderr, msg)
			return nil, nil	
	}
}

func (ip *initProc) prettyPrint(iw util.IndentedWriter) {
	iw.Print(ip.symbol + "(")
	ip.channel.prettyPrint(iw)
	iw.Println(") with")
	iw.Inc()
	iw.Print("role ")
	ip.participant.prettyPrint(iw)
	iw.Println(" as ")
	if ip.app != nil {
		ip.app.prettyPrint(iw)
	} else {
		iw.Print("current")
	}
	iw.Println("")
	iw.Dec()
}

func (ip *initProc) String() string {
	s := ip.symbol + "(" + ip.channel.String() + ") with role " + ip.participant.String() + " as "
	if ip.app != nil {
		s += ip.app.String()
	} else {
		s += "current"
	}
	return s
}

/******************************************************************************
 * accept process
 ******************************************************************************/

type acceptProc struct {
	initProc
}

func newAcceptProc(channel expression, participant *participantExpr, app*application, cont process, line int) *acceptProc {
	return &acceptProc {
		initProc: initProc {
			baseNode: baseNode{line: line},
			channel: channel,
			participant: participant,
			app: app,
			cont: cont,
			symbol: "acc",
		},
	}
}

func (ap *acceptProc) expressionCheck(ctx *expressionCheckContext, lin *linearContext, log util.ErrorLog) *linearContext {
	ok := ap.initProc.channelCheck(ctx, log)
	if ok {
		found := false
		for _, parameter := range ap.gconfig.participants() {
			if ap.participant.id == parameter.String() {
				found = true
				break
			}
		}
		if !found {
			ap.participant.reportErrorf(log, "unknown participant: %q", ap.participant.id)
		}
		ap.applicationCheck(ctx, log)
	}
	return ap.cont.expressionCheck(ctx, lin, log)
}

func (ap *acceptProc) evaluate(ctx *evaluationContext) {
	shChannel, substitution := ap.initProc.evaluate(ctx)
	go func() {
		ctx.add()
		defer ctx.done()

		channels, running := shChannel.accept(ap.participant.id)
		if running && channels != nil {
			ap.app.evaluate(ap.participant, substitution, channels, ctx)
		}
	}()
	ap.cont.evaluate(ctx)
}

/******************************************************************************
 * request process
 ******************************************************************************/

type requestProc struct {
	initProc
}

func newRequestProc(channel expression, participant *participantExpr, app *application, cont process, line int) *requestProc {
	return &requestProc {
		initProc: initProc {
			baseNode: baseNode{line: line},
			channel: channel,
			participant: participant,
			app: app,
			cont: cont,
			symbol: "req",
		},
	}
}

func (rp *requestProc) expressionCheck(ctx *expressionCheckContext, lin *linearContext, log util.ErrorLog) *linearContext {
	ok := rp.initProc.channelCheck(ctx, log)

	if rp.ctype.requester == nil {
		rp.reportErrorf(
			log,
			"expecting request participant: %q",
			rp.participant.String(),
		)
		return rp.cont.expressionCheck(ctx, lin, log)
	}

	if ok {
		if (rp.participant.id != rp.ctype.requester.String()) {
			rp.reportErrorf(
				log, 
				"mismatched request participant: expecting %q, but found %q.", 
				rp.ctype.requester.String(), rp.participant.id,
			)
		}

		participants := rp.initProc.applicationCheck(ctx, log)
		if rp.app == nil {
			for _, participant := range participants {
				if participant.String() == rp.ctype.requester.String() { continue }
				if !ctx.addParticipant(participant) {
					rp.reportErrorf(log, "duplicate definition of participant: %q.", participant.String())
				}
			}
			lin = lin.newSession(participants, rp.line)
			lin = rp.cont.expressionCheck(ctx, lin, log)
			rp.loc = lin.removeSession(participants, rp.line)

			for _, participant := range participants {
				if participant.String() == rp.ctype.requester.String() { continue }
				ctx.removeParticipant(participant)
			}
			return lin
		}
	}
	return rp.cont.expressionCheck(ctx, lin, log)
}

func (rp *requestProc) evaluate(ctx *evaluationContext) {
	shChannel, substitution := rp.initProc.evaluate(ctx)

	channels, running := shChannel.request(rp.participant.id)

	if rp.app != nil {
		if running && channels != nil {
			rp.app.evaluate(rp.participant, substitution, channels, ctx)
		}
	} else {
		if !running || channels == nil {
			channels = shChannel.channels(false) //.gconfig.channels(newBasicChannel(false))
		}
		participants := make([]*participantExpr, shChannel.participants().Len())
		iterator := shChannel.participants().Iterator()
		// for i, parameter := range shChannel.participants() {
		for i, parameter, hasElement := iterator(); hasElement; i, parameter, hasElement = iterator() {
			participants[i] = parameter
		}

		for _, participant := range participants {
			if rp.participant.id == participant.id {
				continue
			}
			ctx.addParticipantChannel(participant, channels[rp.participant.id][participant.id], channels[participant.id][rp.participant.id])
		}
	}
	rp.cont.evaluate(ctx)
}

/******************************************************************************
 * recursion process
 ******************************************************************************/

type recursionProc struct {
	baseNode
	rec 		*participantExpr
	app 		*application

	participant *participantExpr
	rchannel 	expression
}

func newRecursionProc(app *application, rchannel expression, participant *participantExpr, line int) *recursionProc {
	return &recursionProc {
		baseNode: baseNode {line: line},
		rec: newParticipantExpr("rec", line),
		app: app,
		rchannel: rchannel,
		participant: participant,
	}
}

func (rp *recursionProc) setFilename(filename string) {
	rp.baseNode.setFilename(filename)
	rp.rec.setFilename(filename)
	rp.app.setFilename(filename)
	if rp.rchannel != nil {
		rp.rchannel.setFilename(filename)
		rp.participant.setFilename(filename)
	}
}

func (rp *recursionProc) typeCheck(ctx *typeCheckContext, log util.ErrorLog) {
	rp.rec.typeCheck(ctx, log)
	rp.app.typeCheck(ctx, log)
	if rp.rchannel != nil {
		rp.rchannel.typeCheck(ctx, log)
		rp.participant.typeCheck(ctx, log)
	}
}

func (rp *recursionProc) projectionCheck(ctx *projectionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
	rp.rec.projectionCheck(ctx, elog, rlog)
	rp.app.projectionCheck(ctx, elog, rlog)
	if rp.rchannel != nil {
		rp.rchannel.projectionCheck(ctx, elog, rlog)
		rp.participant.projectionCheck(ctx, elog, rlog)
	}
}

func (rp *recursionProc) expressionCheck(ctx *expressionCheckContext, lin *linearContext, log util.ErrorLog) *linearContext {
	if rp.rchannel != nil {
		tdef := rp.rchannel.expressionCheck(ctx, log)
		if tdef != nil {
			ctype, ok := tdef.getType().(*recChannelType)
			if !ok {
				rp.reportErrorf(log, "expecting recursive channel type; instead found %q", tdef.String())
			} else if !ctype.hasParticipant(rp.participant.pType()) {
				rp.reportErrorf(log, "channel %q does not define participant %q", rp.rchannel.String(), rp.participant.String())
			}
		}
	}

	participants := lin.initialParticipants
	parameters := util.NewHashSet[string]()
	for _, p := range participants {
		parameters.Add(p.String())
	}
	loc := rp.app.expressionCheck(rp.rec, parameters, ctx, log)
	lin.recLoc = loc
	return lin.newRecLocal(rp.line)
}

func (rp *recursionProc) sessionCheck(ctx *sessionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
	rp.rec.sessionCheck(ctx, elog, rlog)
	rp.app.sessionCheck(ctx, elog, rlog)
	if rp.rchannel != nil {
		rp.rchannel.sessionCheck(ctx, elog, rlog)
		rp.participant.sessionCheck(ctx, elog, rlog)
	}
}

func (rp *recursionProc) evaluate(ctx *evaluationContext) {
	running := true
	if rp.rchannel != nil {
		v := rp.rchannel.evaluate(ctx)
		switch rchannel := v.(type) {
			case recursorChannel:
				running = rchannel.recurse(rp.participant.id)
			default:
				msg := rp.rchannel.runtimeErrorf(
                	"encountered a non recursive channel: %q", 
                	rchannel.String(),
            	)
            	fmt.Fprintln(os.Stderr, msg)
				running = false
		}
	}
	if running {
		channels := ctx.getParticipantChannels(rp.rec)
		rp.app.evaluate(rp.rec, nil, channels, ctx)
	}
}

func (rp *recursionProc) prettyPrint(iw util.IndentedWriter) {
	iw.Print("rec")
	if rp.rchannel != nil {
		iw.Print("(")
		rp.rchannel.prettyPrint(iw)
	}
	iw.Print(") ")
	rp.app.prettyPrint(iw)
	iw.Println("")
}

func (rp *recursionProc) String() string {
	s := "rec" 
	if rp.rchannel != nil {
		s += "(" + rp.rchannel.String() + ")"
	}
	return s + rp.app.String()
}
