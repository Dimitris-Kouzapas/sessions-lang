package ast

import (
	"sessions/ast/util"
	"sessions/ast/communication"
)

/******************************************************************************
 * []*participantType
 ******************************************************************************/
//TODO put the code for participantTypeList  somewhere else
type participantTypeList struct {
	util.List[*participantType]
}

func newParticipantTypeList(list []*participantType) *participantTypeList {
	return &participantTypeList {
		List: util.NewList(list),		
	}
}

func (p participantTypeList) setFilename(filename string) {
	f := func(_ int, recursor *participantType) { recursor.setFilename(filename) }
	p.List.Iterate(f)
}

func (p participantTypeList) typeCheck(ctx *typeCheckContext, log util.ErrorLog) {
	f := func(_ int, recursor *participantType) { recursor.typeCheck(ctx, log) }
	p.List.Iterate(f)
}

func (p participantTypeList) projectionCheck(ctx *projectionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
	f := func(_ int, recursor *participantType) {
		recursor.projectionCheck(ctx, elog, rlog)
    	if !ctx.addParticipant(recursor) {
            recursor.reportErrorf(elog, "duplicated participant definition: %q.", recursor.String())
        }
    }
    p.List.Iterate(f)
}

func (p participantTypeList) participantList() *participantList {
	f := func(recursor *participantType) *participantExpr { return recursor.defaultValue().(*participantExpr) }
	plist := util.Map(f, p.List)
    return &participantList{List: plist}
}

/******************************************************************************
 * []*participantExpr
 ******************************************************************************/
//TODO put the code for participantList somewhere else

type participantList struct {
	util.List[*participantExpr]
}

func newParticipantList(list []*participantExpr) *participantList {
	return &participantList {
		List: util.NewList(list),		
	}
}

func (p participantList) setFilename(filename string) {
	f := func(_ int, recursor *participantExpr) { recursor.setFilename(filename) }
	p.List.Iterate(f)
}

func (p participantList) typeCheck(ctx *typeCheckContext, log util.ErrorLog) {
	f := func(_ int, recursor *participantExpr) { recursor.typeCheck(ctx, log) }
	p.List.Iterate(f)
}

func (p participantList) expressionCheck(ctx *expressionCheckContext, log util.ErrorLog) {
	f := func (_ int, recursor *participantExpr) {
		if !ctx.addParticipant(recursor) {
			recursor.reportErrorf(log, "Duplicate definition of participant: %q.", recursor.String())
	    }
	}
	p.List.Iterate(f)
}

func (p participantList) projectionCheck(ctx *projectionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
	f := func(_ int, recursor *participantExpr) {
		recursor.projectionCheck(ctx, elog, rlog)
    }
    p.List.Iterate(f)
}

func (p participantList) participantTypeList() *participantTypeList {
	f := func(recursor *participantExpr) *participantType { return recursor.pType() }
	plist := util.Map(f, p.List)
    return &participantTypeList{List: plist}
}

func (p participantList) sessionCheck(ctx *sessionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
	f := func(_ int, parameter *participantExpr) { parameter.sessionCheck(ctx, elog, rlog) }
	p.List.Iterate(f)
}

/******************************************************************************
 * channel interface
 ******************************************************************************/

type channelMap map[string](map[string] communication.Channel[expression])

func newBasicChannel(active bool) func() communication.Channel[expression] {
	return func() communication.Channel[expression] {
		return communication.NewBasicChannel[expression](active)
	}
}

/******************************************************************************
 * 
 * 
 *  Requester Broker Data Structures 
 * 
 *  
 ******************************************************************************/

/******************************************************************************
 * reqChannel type
 ******************************************************************************/

type reqChannelType struct {
    baseNode
    gconfig typedef
    requester *participantType
}

func newReqChannelType(gconfig typedef, requester *participantType, line int) *reqChannelType {
    return &reqChannelType {
         baseNode: baseNode {
            line: line,
        },
        gconfig: gconfig,
        requester: requester,
    }
}

func (rct *reqChannelType) setFilename(filename string) {
    rct.baseNode.setFilename(filename)
    rct.gconfig.setFilename(filename)
    if rct.requester != nil {
    	rct.requester.setFilename(filename)
    }
}

func (rct *reqChannelType) subtypeOf(tdef typedef) bool {
    visited := util.NewHashSet[typePair]() 
    return rct.subtypeOf_(tdef, visited)
}

func (rct *reqChannelType) subtypeOf_(tdef typedef, visited util.Set[typePair]) bool {
    switch ctype := tdef.getType().(type) {
        case *nothingType:
            return true
        case *reqChannelType:
            if !rct.gconfig.subtypeOf_(ctype.gconfig, visited) {
                return false
            }
            if rct.requester == nil && ctype.requester == nil {
            	return true
            } else if rct.requester != nil && ctype.requester == nil {
            	return true //false
            } else if rct.requester == nil && ctype.requester != nil {
            	return false
            }

            return rct.requester.subtypeOf(ctype.requester)
        default:
            return false
    }
}

func (rct *reqChannelType) join(tdef typedef) (typedef, bool) {
    switch ctype := tdef.getType().(type) {
        case *nothingType:
            return rct, true
        case *reqChannelType:
            gconfig, ok := rct.gconfig.join(ctype.gconfig)
            chType := newReqChannelType(gconfig, rct.requester, rct.line)
            if !ok {
                return chType, false
            }

            if rct.requester == nil && ctype.requester == nil {
            	return chType, true
            } else if rct.requester != nil && ctype.requester == nil {
            	return chType, true
            } else if rct.requester == nil && ctype.requester != nil {
            	return tdef, false
            }

            return chType, rct.requester.subtypeOf(ctype.requester) 
        default:
            return tdef, false
    }
}

func (rct *reqChannelType) getType() typedef { return rct }
func (rct *reqChannelType) typeCheck(ctx *typeCheckContext, log util.ErrorLog) {
    if rct.requester != nil {
        rct.requester.typeCheck(ctx, log)
    }
    rct.gconfig.typeCheck(ctx, log)
    
    gconfig := rct.gconfig.getType()

    if gconfig == nil { return }
    gc, ok := gconfig.(globalConfig)
    if !ok {
        rct.gconfig.reportErrorf(log, "expecting global configuration type; instead found %q.", gconfig.String())
        return
    }

	if rct.requester != nil {
	    found := false
	    for i := range gc.participants() {
	        p := gc.participant(i)
	        if p == rct.requester.participant {
	            found = true
	            break
	        }
	    }

	    if !found {
	        stream := util.NewStream().Inc().Inc()
	        rct.gconfig.prettyPrint(stream)
	        rct.requester.reportErrorf(
	            log,
	            "channel type; global configuration:\n%s\n\tdoes not define request participant %q",
	            stream.String(), rct.requester.String(),
	        )
	    }
    }
}

func (rct *reqChannelType) projectionCheck(ctx *projectionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
	if rct.requester != nil {
    	rct.requester.projectionCheck(ctx, elog, rlog)
    }
    //COMMENT (TODO for searching) uncomment the next line as channel type definitions might be partial
    // switch gc := rct.gconfig.(type) {
    //     case *globalDef:
    //         gc.projectionCheck(ctx, elog, rlog)
    //     case *localContext:
    //         ctx.push()
    //         defer ctx.pop()
    //         gc.globalConfigImpl.projectionCheck(ctx, elog, rlog)
    // }
    rct.gconfig.projectionCheck(ctx, elog, rlog)
}

func (rct *reqChannelType) defaultValue() expression {
	var requester *participantExpr
	if rct.requester != nil {
		requester = rct.requester.defaultValue().(*participantExpr)
	}
    return newReqChannel(requester, rct.gconfig, rct.line)
}

func (rct *reqChannelType) prettyPrint(iw util.IndentedWriter) {
    iw.Println("shared { ")
    iw.Inc()
    rct.gconfig.prettyPrint(iw)
    iw.Print(" chan")
    iw.Println()
    if rct.requester != nil {
	    rct.requester.prettyPrint(iw)
	    iw.Println(" requester")
	}
    iw.Dec()
    iw.Println("}")
}

func (rct *reqChannelType) String() string {
    s := "shared { " + rct.gconfig.String() + " chan; "
    if rct.requester != nil {
    	s += rct.requester.String() + " requester;"
    }
    s += " }"
    return s
}

/******************************************************************************
 * shared channel interface
 ******************************************************************************/

type sharedChannel interface {
	expression
	close()
}

/******************************************************************************
 * requester channel interface
 ******************************************************************************/

type requesterChannel interface {
	participants() *participantList
	channels(bool) channelMap
	request(string) (channelMap, bool)
	accept(string) (channelMap, bool)
}

/******************************************************************************
 * reqChannel expression
 ******************************************************************************/

type reqChannel struct {
	baseNode
	tdef 		typedef
	requester 	*participantExpr

	gconfig 	globalConfig
	reqBroker 	*communication.RequesterBroker[expression]
}

func newReqChannel(requester *participantExpr, tdef typedef, line int) *reqChannel {
	return &reqChannel {
		baseNode:	baseNode{line: line},
		requester:	requester,
		tdef:		tdef,
	}
}

func (rc *reqChannel) getType() typedef {
	tdef := rc.tdef.getType()
	return newReqChannelType(tdef, rc.requester.pType(), rc.line)
}

func (rc *reqChannel) setFilename(filename string) {
	rc.baseNode.setFilename(filename)
	rc.tdef.setFilename(filename)
}

func (rc *reqChannel) typeCheck(ctx *typeCheckContext, log util.ErrorLog) {
	rc.tdef.typeCheck(ctx, log)
	if tdef := rc.tdef.getType(); tdef != nil {
		ok := true
		if rc.gconfig, ok = tdef.(globalConfig); !ok {
			rc.reportErrorf(log, "Expecting global configuration type. Found %s instead", rc.tdef.String())
		}
	}
	rc.requester.typeCheck(ctx, log)
}

func (rc *reqChannel) expressionCheck(ctx *expressionCheckContext, log util.ErrorLog) typedef {
	found := false
	acceptors := make([]string, 0)
	for i := range rc.gconfig.participants() {
		if rc.gconfig.participant(i) == rc.requester.id {
			found = true
		} else {
			acceptors = append(acceptors, rc.gconfig.participant(i))
		}
	}

	if !found {
		stream := util.NewStream().Inc().Inc()
		rc.gconfig.prettyPrint(stream)
		rc.requester.reportErrorf(
			log,
			"channel expression: global configuration\n%s\tdoes not define request participant %q",
			stream.String(), rc.requester.String(),
		)
		return nil
	}

	f := func() map[string](map[string] communication.Channel[expression]) {
		return rc.gconfig.channels(newBasicChannel(true))
	}

	rc.reqBroker = communication.NewRequesterBroker[expression](rc.requester.id, acceptors, f)
	return newReqChannelType(rc.gconfig, rc.requester.pType(), rc.line)
}

func (rc *reqChannel) projectionCheck(ctx *projectionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
	rc.requester.projectionCheck(ctx, elog, rlog)
	rc.tdef.projectionCheck(ctx, elog, rlog)
}

func (rc *reqChannel) sessionCheck(ctx *sessionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
	rc.requester.sessionCheck(ctx, elog, rlog)
}

func (_ *reqChannel) operation(_ *evaluationContext, _ expression, _ string) expression { return nil }

func (rc *reqChannel) evaluate(_ *evaluationContext) expression {
	return &reqChannel {
		baseNode:	baseNode{line: rc.line},
		requester:	rc.requester,
		tdef:		rc.tdef,
		gconfig:	rc.gconfig,
		reqBroker: 	rc.reqBroker.Copy(),
	}
}

func (rc *reqChannel) participants() *participantList {
	return newParticipantTypeList(rc.gconfig.participants()).participantList()
}

func (rc *reqChannel) channels(active bool) channelMap {
	return rc.gconfig.channels(newBasicChannel(active))
}

func (rc *reqChannel) request(requester string) (channelMap, bool) {
	channels, running, exists := rc.reqBroker.Request(requester)
	if !exists {
		rc.runtimeErrorf("channel recursion: unknown requester participant: %q.", requester)
	}
	return channels, running
}

func (rc *reqChannel) accept(acceptor string) (channelMap, bool) {
	channels, running, exists := rc.reqBroker.Accept(acceptor)
	if !exists {
		rc.runtimeErrorf("channel recursion: unknown acceptor participant: %q.", acceptor)
	}
	return channels, running
}

func (rc *reqChannel) close() {
	rc.reqBroker.Stop()
}

func (rc *reqChannel) prettyPrint(iw util.IndentedWriter) {
	iw.Println("sync { ")
    iw.Inc()
    iw.Print("chan: ")
    iw.Inc()
    rc.gconfig.prettyPrint(iw)
    iw.Dec()
    iw.Println()
    if rc.requester != nil {
        iw.Print("requester: ")
        iw.Inc()
        rc.requester.prettyPrint(iw)
    }
    iw.Println()
    iw.Dec()
    iw.Dec()
    iw.Println("}")
}

func (rc *reqChannel) String() string {
	s := "sync { chan: " + rc.gconfig.String() + ";" 
    if rc.requester != nil {
        s += "requester " + rc.requester.String() + ";"
    }
    s += " }"
    return s
}

/******************************************************************************
 * 
 * 
 *  Recursive Channel Data Structures 
 * 
 *  
 ******************************************************************************/

/******************************************************************************
 * rec channel type
 ******************************************************************************/

type recChannelType struct {
	baseNode
	recursors *participantTypeList
}

func newRecChannelType(recursors []*participantType,  line int) *recChannelType {
	return newRecChannelType_(newParticipantTypeList(recursors), line)
}

func newRecChannelType_(recursors *participantTypeList,  line int) *recChannelType {
    return &recChannelType {
        baseNode: baseNode {
        	line: line,
        },
        recursors: recursors,
    }
}

func (rct *recChannelType) hasParticipant(participant *participantType) bool {
	return rct.recursors.Contains(participant)
}

func (rct *recChannelType) setFilename(filename string) {
    rct.baseNode.setFilename(filename)
	rct.recursors.setFilename(filename)
}

func (rct *recChannelType) subtypeOf(tdef typedef) bool {
    visited := util.NewHashSet[typePair]() 
    return rct.subtypeOf_(tdef, visited)
}

func (rct *recChannelType) subtypeOf_(tdef typedef, visited util.Set[typePair]) bool {
    switch ctype := tdef.getType().(type) {
        case *nothingType:
            return true
        case *recChannelType:
        	iterator := ctype.recursors.Iterator()
        	for _, r, hasRecursor := iterator(); hasRecursor; _, r, hasRecursor = iterator() {
        		if !rct.recursors.Contains(r) {
        			return false
        		}
        	}
            return true
        default:
            return false
    }
}

func (rct *recChannelType) join(tdef typedef) (typedef, bool) {
    switch ctype := tdef.getType().(type) {
        case *nothingType:
            return rct, true
        case *recChannelType:
        	iterator := ctype.recursors.Iterator()
        	for _, r, hasRecursor := iterator(); hasRecursor; _, r, hasRecursor = iterator() {
        		if !rct.recursors.Contains(r) {
        			return tdef, false
        		}
        	}
            return newRecChannelType_(ctype.recursors, rct.line), true
        default:
            return tdef, false
    }
}

func (rct *recChannelType) getType() typedef { return rct }
func (rct *recChannelType) typeCheck(ctx *typeCheckContext, log util.ErrorLog) {
	rct.recursors.typeCheck(ctx, log)
}

func (rct *recChannelType) projectionCheck(ctx *projectionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
	ctx.push()
	defer ctx.pop()
	rct.recursors.projectionCheck(ctx, elog, rlog)
}

func (rct *recChannelType) defaultValue() expression {
	plist := rct.recursors.participantList()
    return newRecChannel_(plist, rct.line)
}

func (rct *recChannelType) prettyPrint(iw util.IndentedWriter) {
    iw.Print("channel")
    f := func(_ int, recursor *participantType) {
		iw.Print(" ")
    	recursor.prettyPrint(iw)
    }
    rct.recursors.Iterate(f)
    iw.Print(" {}")
}

func (rct *recChannelType) String() string {
	s := "channel"
	f := func(_ int, recursor *participantType) {
		s += " " + recursor.String()
    }
    rct.recursors.Iterate(f)
    return s + " {}"
}

/******************************************************************************
 * recursor channel interface
 ******************************************************************************/

type recursorChannel interface {
	participants() *participantList
	recurse(string) bool
}

/******************************************************************************
 * rec channel
 ******************************************************************************/

type recChannel struct {
	baseNode
	recs *participantList
	recBroker *communication.RecursorBroker
}

func newRecChannel(recs []*participantExpr, line int) *recChannel {
	return newRecChannel_(newParticipantList(recs), line)
}

func newRecChannel_(recs *participantList, line int) *recChannel {
	return &recChannel {
		baseNode: baseNode{ line: line },
		recs: recs,
		recBroker: communication.NewRecursorBroker(recs.Strings()),
	}
}

func (rc *recChannel) getType() typedef {
	plist := rc.recs.participantTypeList()
	return newRecChannelType_(plist, rc.line)
}

func (rc *recChannel) setFilename(filename string) {
	rc.baseNode.setFilename(filename)
	rc.recs.setFilename(filename)
}

func (rc *recChannel) typeCheck(ctx *typeCheckContext, log util.ErrorLog) {
	rc.recs.typeCheck(ctx, log)
}
func (rc *recChannel) projectionCheck(ctx *projectionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
	rc.recs.projectionCheck(ctx, elog, rlog)
}
func (rc *recChannel) expressionCheck(ctx *expressionCheckContext, log util.ErrorLog) typedef {
	ctx.pushFrame()
	defer ctx.popFrame()

	rc.recs.expressionCheck(ctx, log)
	return rc.getType()
}

func (rc *recChannel) sessionCheck(ctx *sessionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
	rc.recs.sessionCheck(ctx, elog, rlog)
}

func (rc *recChannel) evaluate(_ *evaluationContext) expression {
	return newRecChannel_(rc.recs, rc.line)
}

func (_ *recChannel) operation(_ *evaluationContext, _ expression, _ string) expression { return nil }
func (rc *recChannel) prettyPrint(iw util.IndentedWriter) {
	iw.Print("async")
	f := func(_ int, recursor *participantExpr) {
		iw.Print(" ")
    	recursor.prettyPrint(iw)
    }
    rc.recs.Iterate(f)
    iw.Print(" {}")
}

func (rc *recChannel) String() string {
	s := "async"
	f := func(_ int, recursor *participantExpr) {
		s += " " + recursor.String()
    }
    rc.recs.Iterate(f)
    return s + " {}"
}

func (rc *recChannel) participants() *participantList {
	return rc.recs
}

func (rc *recChannel) recurse(recursor string) bool {
	running, exists := rc.recBroker.Recurse(recursor)
	if !exists {
		rc.runtimeErrorf("channel recursion; unknown recursor participant: %q.", recursor)
	}
	return running
}

func (rc *recChannel) close() {
	rc.recBroker.Stop()
}

/******************************************************************************
 * renameChannel expression
 *****************************************************************************/

type renameChannel struct {
	baseNode
	parameters 	*participantList
	expr 		expression

	substitution map[string]string
	fromSubstitution map[string]string
}

func newRenameChannel(parameters []*participantExpr, expr expression, line int) *renameChannel {
	return &renameChannel {
		baseNode: baseNode{ line: line },
		parameters: newParticipantList(parameters),
		expr: expr,
	}
}

func (rc *renameChannel) setFilename(filename string) {
	rc.baseNode.setFilename(filename)
	rc.parameters.setFilename(filename)
	rc.expr.setFilename(filename)
}

func (rc *renameChannel) getType() typedef {
	tdef := rc.expr.getType()
	switch ctype := tdef.(type) {
		case *recChannelType:
			parameters := rc.parameters.participantTypeList()
			return newRecChannelType_(parameters, rc.line)
		case *reqChannelType:
			gconfig, ok := ctype.gconfig.(globalConfig)
			if !ok {
				return nil
			}

			parameters := make([]*participantType, rc.parameters.Len())
			var requester *participantType
			if ctype.requester != nil {
				for i := range gconfig.participants() {
					if ctype.requester.String() == gconfig.participant(i) {
						requester = newParticipantType(rc.parameters.String(i), rc.line)
					}
					parameters[i] = newParticipantType(rc.parameters.String(i), rc.line) 
				}
			}

			switch gc := gconfig.(type) {
				case *localContext:
					lcont := gc.substitute(parameters)
					return newReqChannelType(lcont, requester, rc.line)
				case *globalDef:
					lcont := gc.projectionSet().substitute(parameters)
					return newReqChannelType(lcont, requester, rc.line)
				case *localSet:
					lset := gc.substitute(parameters)
					return newReqChannelType(lset, requester, rc.line)
			}
	}
	return nil
}

func (rc *renameChannel) typeCheck(ctx *typeCheckContext, log util.ErrorLog) {
	rc.parameters.typeCheck(ctx, log)
	rc.expr.typeCheck(ctx, log)
}

func (rc *renameChannel) expressionCheck(ctx *expressionCheckContext, log util.ErrorLog) typedef {
	ctx.pushFrame()
	defer ctx.popFrame()

	rc.parameters.expressionCheck(ctx, log)

	tdef := rc.expr.expressionCheck(ctx, log)
	if tdef == nil {
		return nil
	}

	switch ctype := tdef.(type) {
		case *recChannelType:
			if rc.parameters.Len() != ctype.recursors.Len() {
				rc.reportErrorf(log, "rename list does not correspond to channel participants list")
				return nil
			}
			rc.substitution = make(map[string]string, rc.parameters.Len())
			rc.fromSubstitution = make(map[string]string, rc.parameters.Len())
			iterator := rc.parameters.Iterator()
			for i, _, hasElement := iterator(); hasElement; i, _, hasElement = iterator() {
				rc.substitution[rc.parameters.String(i)] = ctype.recursors.String(i)
				rc.fromSubstitution[ctype.recursors.String(i)] = rc.parameters.String(i)
			}
			parameters := rc.parameters.participantTypeList()
			return newRecChannelType_(parameters, rc.line)
		case *reqChannelType:
			gconfig, ok := ctype.gconfig.(globalConfig)
			if !ok {
				return nil
			}
			if rc.parameters.Len() != len(gconfig.participants()) {
				rc.reportErrorf(log, "rename list does not correspond to channel participant list")
				return nil
			}
			var requester *participantType
			rc.substitution = make(map[string]string, rc.parameters.Len())
			rc.fromSubstitution = make(map[string]string, rc.parameters.Len())

			parameters := make([]*participantType, rc.parameters.Len())
			iterator := rc.parameters.Iterator()
			for i, _, hasElement := iterator(); hasElement; i, _, hasElement = iterator() {
				rc.substitution[rc.parameters.String(i)] = gconfig.participant(i)
				rc.fromSubstitution[gconfig.participant(i)] = rc.parameters.String(i)
				if ctype.requester != nil && (ctype.requester.String() == gconfig.participant(i)) {
					requester = newParticipantType(rc.parameters.String(i), rc.line)
				}
				parameters[i] = newParticipantType(rc.parameters.String(i), rc.line)
			}

			switch gc := gconfig.(type) {
				case *localContext:
					lcont := gc.substitute(parameters)
					return newReqChannelType(lcont, requester, rc.line)
				case *globalDef:
					lcont := gc.projectionSet().substitute(parameters)
					return newReqChannelType(lcont, requester, rc.line)
				case *localSet:
					lset := gc.substitute(parameters)
					return newReqChannelType(lset, requester, rc.line)
			}
		default:
			rc.reportErrorf(log, "expecting recursion channel type; found %q instead", tdef.String())
			return nil
	}
	return nil
}

func (rc *renameChannel) projectionCheck(ctx *projectionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
	rc.parameters.projectionCheck(ctx, elog, rlog)
	rc.expr.projectionCheck(ctx, elog, rlog)
}

func (rc *renameChannel) sessionCheck(ctx *sessionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
	rc.parameters.sessionCheck(ctx, elog, rlog)
	rc.expr.sessionCheck(ctx, elog, rlog)
}

func (rc *renameChannel) evaluate(ctx *evaluationContext) expression {
	rchannel := rc.expr.evaluate(ctx)
	return &renameChannel {
		baseNode: baseNode{line: rc.line},
		parameters: rc.parameters,
		expr: rchannel,
		substitution: rc.substitution,
		fromSubstitution: rc.fromSubstitution,
	}
}

func (rc *renameChannel) operation(_ *evaluationContext, _ expression, _ string) expression {
	return nil
}

func (rc *renameChannel) prettyPrint(iw util.IndentedWriter) {
	iw.Print("[")
	f := func(i int, parameter *participantExpr) {
		if i != 0 {
			iw.Print(" ")
		}
		parameter.prettyPrint(iw)
    }
    rc.parameters.Iterate(f)
	iw.Print("] ")
	rc.expr.prettyPrint(iw)
}

func (rc *renameChannel) String() string {
	s := "["
	f := func(i int, parameter *participantExpr) {
		if i != 0 {
			s += " "
		}
		s += parameter.String()
	}
	rc.parameters.Iterate(f)
	return s + "] " + rc.expr.String()
}

func (rc *renameChannel) participants() *participantList {
	return rc.parameters
}

func (rc *renameChannel) substituteChannels(channels channelMap) channelMap {
	outer := make(channelMap)
	for k1, v1 := range channels {
		inner := make(map[string]communication.Channel[expression])
		for k2, v2 := range v1 {
			inner[rc.fromSubstitution[k2]] = v2
		}
		outer[rc.fromSubstitution[k1]] = inner
	}
	return outer
}

func (rc *renameChannel) channels(active bool) channelMap {
	switch rchannel := rc.expr.(type) {
		case requesterChannel:
			channels := rchannel.channels(active)
			return rc.substituteChannels(channels)
		default:
			return nil
	}
}

func (rc *renameChannel) request(requester string) (channelMap, bool) {
	switch rchannel := rc.expr.(type) {
		case requesterChannel:
			req := rc.substitution[requester]
			channels, ok := rchannel.request(req)
			channels = rc.substituteChannels(channels)
			return channels, ok
	}
	return nil, false
}

func (rc *renameChannel) accept(acceptor string) (channelMap, bool) {
	switch rchannel := rc.expr.(type) {
		case requesterChannel:
			acc := rc.substitution[acceptor]
			channels, ok := rchannel.accept(acc)
			channels = rc.substituteChannels(channels)
			return channels, ok
	}
	return nil, false
}

func (rc *renameChannel) recurse(recursor string) bool {
	switch rchannel := rc.expr.(type) {
		case recursorChannel:
			rec := rc.substitution[recursor]
			return rchannel.recurse(rec)
	}
	return false
}
