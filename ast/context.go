package ast

import (
    "fmt"
    "sync"

    "sessions/ast/util"
    "sessions/ast/communication"
)

/******************************************************************************
 * name stack
 ******************************************************************************/

type nameStack struct {
    variableStack    []map[string]typedef
    participantStack []map[string]*participantType
}

func newNameStack() *nameStack {
    return &nameStack{
        variableStack:    make([]map[string]typedef, 0, 4),
        participantStack: make([]map[string]*participantType, 0, 4),
    }
}

func (ns *nameStack) pushFrame() {
    ns.variableStack = append(ns.variableStack, make(map[string]typedef))
    ns.participantStack = append(ns.participantStack, make(map[string]*participantType))
}

func (ns *nameStack) popFrame() {
    if len(ns.variableStack) == 0 || len(ns.participantStack) == 0 {
        // nothing to pop; silently ignore (or panic)
        return
    }
    ns.variableStack = ns.variableStack[:len(ns.variableStack)-1]
    ns.participantStack = ns.participantStack[:len(ns.participantStack)-1]
}

func (ns *nameStack) findName(name string) (typedef, bool) {
    for i := len(ns.variableStack) - 1; i >= 0; i-- {
        if tdef, ok := ns.variableStack[i][name]; ok {
            return tdef, true
        }
    }
    return nil, false
}

func (ns *nameStack) findParticipant(name string) (*participantType, bool) {
    if len(ns.participantStack) == 0 {
        return nil, false
    }
    top := ns.participantStack[len(ns.participantStack) - 1]
    if tdef, ok := top[name]; ok {
        return tdef, true
    }
    return nil, false
}

func (ns *nameStack) addParticipant(name string, participant *participantType) bool {
    if len(ns.participantStack) == 0 || len(ns.variableStack) == 0 {
        return false
    }
    topName := ns.variableStack[len(ns.variableStack) - 1]
    if _, ok := topName[name]; ok {
        return false
    }
    top := ns.participantStack[len(ns.participantStack) - 1]
    if _, ok := top[name]; ok {
        return false
    }
    top[name] = participant
    return true
}

func (ns *nameStack) addName(name string, tdef typedef) bool {
    if len(ns.participantStack) == 0 || len(ns.variableStack) == 0 {
        return false
    }
    topParticipant := ns.participantStack[len(ns.participantStack) - 1]
    if _, ok := topParticipant[name]; ok {
        return false
    }
    top := ns.variableStack[len(ns.variableStack) - 1]
    if _, ok := top[name]; ok {
        return false
    }
    top[name] = tdef
    return true
}

func (ns *nameStack) removeName(name string) {
    if len(ns.variableStack) == 0 {
        return
    }
    top := ns.variableStack[len(ns.variableStack) - 1]
    delete(top, name)
}

func (ns *nameStack) removeParticipant(name string) {
    if len(ns.participantStack) == 0 {
        return
    }
    top := ns.participantStack[len(ns.participantStack) - 1]
    delete(top, name)
}

/******************************************************************************
 * typeCheck context
 ******************************************************************************/

type typeCheckContext struct {
    p       *program
    m       *module
    labels  map[string]*labelType
}

func newTypeCheckContext(p *program, m *module) *typeCheckContext {
    return &typeCheckContext{
        p: p,
        m: m,
    }
}

func (t *typeCheckContext) resetLabels() {
    t.labels = make(map[string]*labelType)
}

func (t *typeCheckContext) addLabel(ltype *labelType) bool {
    if _, ok := t.labels[ltype.label]; ok {
        return false
    }
    t.labels[ltype.label] = ltype
    return true
}

func (t *typeCheckContext) getTypeDecl(name string) typedef {
    if tdef := t.m.getTypeDecl(name); tdef != nil {
        return tdef
    }

    if imp, ok := t.m.getImport(name); ok {
        if mod := t.p.getModule(imp); mod != nil {
            return mod.getType()
        } 
    }
    return nil
}

/******************************************************************************
 * projectionCheck context
 ******************************************************************************/

type projectionCheckContext struct {
    config *util.Config
    participantStack []util.Set[string]
}

func newProjectionCheckContext(config *util.Config) *projectionCheckContext {
    return &projectionCheckContext{
        config: config,
        participantStack: make([]util.Set[string], 0, 4),
    }
}

func (pc *projectionCheckContext) push() {
    pc.participantStack = append(pc.participantStack, util.NewHashSet[string]())
}

func (pc *projectionCheckContext) pop() {
    if len(pc.participantStack) == 0 {
        return
    }
    pc.participantStack = pc.participantStack[:len(pc.participantStack) - 1]
}

func (pc *projectionCheckContext) top() util.Set[string] {
    if len(pc.participantStack) == 0 {
        // ensure a frame exists to avoid nil deref
        pc.push()
    }
    return pc.participantStack[len(pc.participantStack) - 1]
}

func (pc *projectionCheckContext) addParticipant(ptype *participantType) bool {
    participants := pc.top()
    if participants.Contains(ptype.participant) {
        return false
    }
    participants.Add(ptype.participant)
    return true
}

func (pc *projectionCheckContext) removeParticipant(ptype *participantType) {
    pc.top().Remove(ptype.participant)
}

func (pc *projectionCheckContext) containsParticipant(ptype *participantType) bool {
    return pc.top().Contains(ptype.participant)
}

func (pc *projectionCheckContext) participants() []string {
    return pc.top().Slice()
}

func (pc *projectionCheckContext) analysis() bool {
    ok1, ok2 := pc.config.Bool("analysis")
    return ok1 && ok2
}

/******************************************************************************
 * expression context
 ******************************************************************************/

type expressionCheckContext struct {
    p      *program
    m      *module
    nstack *nameStack
}

func newExpressionCheckContext(p *program, m *module) *expressionCheckContext {
    return &expressionCheckContext{
        p:      p,
        m:      m,
        nstack: newNameStack(),
    }
}

func (e *expressionCheckContext) pushFrame() { e.nstack.pushFrame() }
func (e *expressionCheckContext) popFrame()  { e.nstack.popFrame() }


func (e *expressionCheckContext) getVariableType(name string) typedef {
    if tdef, ok := e.nstack.findName(name); ok {
        return tdef
    }
    
    if session := e.getSession(name); session != nil {
        return session
    }

    if value := e.m.getValue(name); value != nil {
        return value.getType()
    }

    if imp, ok := e.m.getImport(name); ok {
        if value := e.p.getModule(imp); value != nil {
            return value.getType()
        } 
    }

    return nil
}

func (e *expressionCheckContext) addName(name string, tdef typedef) bool { return e.nstack.addName(name, tdef) }
func (e *expressionCheckContext) removeName(name string)                 { e.nstack.removeName(name) }


func (e *expressionCheckContext) addParticipant(pexpr *participantExpr) bool {
    return e.nstack.addParticipant(pexpr.id, pexpr.pType())
}

func (e *expressionCheckContext) removeParticipant(pexpr *participantExpr) {
    e.nstack.removeParticipant(pexpr.id)
}

func (e *expressionCheckContext) getParticipant(pexpr *participantExpr) *participantType {
    if tdef, ok := e.nstack.findParticipant(pexpr.id); ok {
        return tdef
    }
    return nil
}

func (e *expressionCheckContext) getSession(name string) typedef {
    session := e.m.getSession(name)
    if session == nil {
        return nil
    }
    stype := session.getType()
    if stype == nil {
        return nil
    }
    return stype
}

func (e *expressionCheckContext) getAbstraction(name string) *abstraction {
    return e.m.getAbstraction(name)
}

/******************************************************************************
 * linear context
 ******************************************************************************/

type linearContext struct {
    roles               map[string]local
    sessions            map[string]string
    freshCounter        int

    initialParticipants []*participantExpr
    recLoc              local
}

func newLinearContext(participants []*participantExpr, line int) *linearContext {
    lin := &linearContext{
        roles:              make(map[string]local),
        sessions:           make(map[string]string),
        freshCounter:       0,
        initialParticipants:participants,
        recLoc:             nil,
    }
    lin.newSession(participants, line)
    return lin
}

func (lc *linearContext) clone() *linearContext {
    lin := new(linearContext)
    lin.roles = make(map[string]local, len(lc.roles))
    for k, v := range lc.roles {
        lin.roles[k] = v
    }
    lin.sessions = make(map[string]string, len(lc.sessions))
    for k, v := range lc.sessions {
        lin.sessions[k] = v
    }
    lin.freshCounter = lc.freshCounter
    lin.initialParticipants = lc.initialParticipants
    lin.recLoc = lc.recLoc
    return lin
}

func (lc *linearContext) freshSession() string {
    s := fmt.Sprintf("s#%v", lc.freshCounter)
    lc.freshCounter++
    return s
}

func (lc *linearContext) newSession(participants []*participantExpr, line int) *linearContext {
    f := ""
    if len(participants) != 0 {
        f = lc.freshSession()
        lc.roles[f] = newEndLocal(line)
    }
    for _, pexpr := range participants {
        lc.sessions[pexpr.id] = f
    }
    return lc
}

func (lc *linearContext) removeSession(participants []*participantExpr, line int) local {
    if len(participants) == 0 {
        return newEndLocal(line)
    }
    f, ok := lc.sessions[participants[0].id]
    if !ok {
        return newEndLocal(line)
    }
    for _, p := range participants { delete(lc.sessions, p.id) }
    loc := lc.roles[f]
    delete(lc.roles, f)
    return loc
}

func (lc *linearContext) newEndLocal(line int) *linearContext {
    old := lc.roles
    lc.roles = make(map[string]local, len(lc.roles))
    for s := range old {
        lc.roles[s] = newEndLocal(line)
    }
    return lc
}

func (lc *linearContext) newRecLocal(line int) *linearContext {
    lin := lc.newEndLocal(line)
    if len(lc.initialParticipants) == 0 {
        return lin
    }
    f, ok := lc.sessions[lc.initialParticipants[0].id]
    if !ok {
        return lin
    }
    lin.roles[f] = newRecLocal(line)
    return lin
}

func (lc *linearContext) newSendLocal(ptype *participantType, tdef typedef, line int) *linearContext {
    if f, ok := lc.sessions[ptype.participant]; ok {
        loc := lc.roles[f]
        lc.roles[f] = newSendLocal(ptype, tdef, loc, line)
    }
    return lc
}

func (lc *linearContext) newReceiveLocal(ptype *participantType, tdef typedef, line int) *linearContext {
    if f, ok := lc.sessions[ptype.participant]; ok {
        loc := lc.roles[f]
        lc.roles[f] = newReceiveLocal(ptype, tdef, loc, line)
    }
    return lc
}

func (lc *linearContext) newSelectLocal(ptype *participantType, labels []*labelType, choices []*linearContext, line int) (*linearContext, bool) {
    f, ok := lc.sessions[ptype.participant]
    if !ok {
        return lc, false
    }

    locals := make([]local, 0, len(choices))
    var newLin *linearContext
    status := true

    for i, c := range choices {
        loc, ok := c.roles[f]
        if !ok {
            status = false
            continue
        }
        locals = append(locals, loc)
        delete(c.roles, f)

        if i == 0 {
            newLin = c.clone()
        } else {
            var st bool
            newLin, st = newLin.join(c)
            if !st {
                status = false
            }
        }
    }
    if newLin == nil {
        return lc, false
    }
    newLin.roles[f] = newSelectLocal(ptype, labels, locals, line)
    return newLin, status
}

func (lc *linearContext) newBranchLocal(ptype *participantType, labels []*labelType, choices []*linearContext, line int) (*linearContext, bool) {
    f, ok := lc.sessions[ptype.participant]
    if !ok {
        return lc, false
    }

    locals := make([]local, 0, len(choices))
    var newLin *linearContext
    status := true

    for i, c := range choices {
        loc, ok := c.roles[f]
        if !ok {
            status = false
            continue
        }
        locals = append(locals, loc)
        delete(c.roles, f)

        if i == 0 {
            newLin = c.clone()
        } else {
            var st bool
            newLin, st = newLin.join(c)
            if !st {
                status = false
            }
        }
    }
    if newLin == nil {
        return lc, false
    }
    newLin.roles[f] = newBranchLocal(ptype, labels, locals, line)
    return newLin, status
}

func (lc *linearContext) join(lin *linearContext) (*linearContext, bool) {
    if len(lc.roles) != len(lin.roles) {
        return lc, false
    }
    newLin := lc.clone()
    status := true
    for s, loc1 := range lc.roles {
        loc2, ok := lin.roles[s]
        if !ok {
            status = false
            continue
        }
        loc, ok2 := loc1.join(loc2)
        if !ok2 {
            status = false
        }
        newLin.roles[s] = loc
    }

    if newLin.recLoc != nil && lin.recLoc != nil {
        ok := true
        //TODO not sure if it is join or equality here
        newLin.recLoc, ok = newLin.recLoc.join(lin.recLoc)
        status = status && ok
    } else if newLin.recLoc == nil && lin.recLoc != nil {
        newLin.recLoc = lin.recLoc
    }
    return newLin, status
}

func (lc* linearContext) prettyPrint(iw util.IndentedWriter) {
    flag := false
    for _, r := range lc.roles {
        if flag {
            iw.Println()
        }
        r.prettyPrint(iw)
        flag = true
    }

    if lc.recLoc != nil {
        if flag {
            iw.Println()
        }
        iw.Print("rec: ")
        lc.recLoc.prettyPrint(iw)
    }
}

/******************************************************************************
 * session check context
 ******************************************************************************/
type sessionCheckContext struct {
    config *util.Config
}

func newSessionCheckContext(config *util.Config) *sessionCheckContext {
    return &sessionCheckContext{
        config: config,
    }
}

func (sc *sessionCheckContext) analysis() bool {
    ok1, ok2 := sc.config.Bool("analysis")
    return ok1 && ok2
}


/******************************************************************************
 * evaluation context
 ******************************************************************************/
type evaluationContext struct {
    p                   *program
    m                   *module
    sendParticipants    map[string]communication.Channel[expression]
    receiveParticipants map[string]communication.Channel[expression]
    variables           map[string]expression
    wg                  *sync.WaitGroup
}

func newEvaluationContext(p *program, m *module) *evaluationContext {
    return &evaluationContext{
        p:                   p,
        m:                   m,
        sendParticipants:    make(map[string]communication.Channel[expression]),
        receiveParticipants: make(map[string]communication.Channel[expression]),
        variables:           make(map[string]expression),
        wg:                  new(sync.WaitGroup),
    }
}

func (e *evaluationContext) emptyCopy(m *module, variables map[string]expression) *evaluationContext {
    return &evaluationContext {
        p:                   e.p,
        m:                   m,
        sendParticipants:    make(map[string]communication.Channel[expression]),
        receiveParticipants: make(map[string]communication.Channel[expression]),
        variables:           variables,
        wg:                  e.wg,
    }
}

func (e *evaluationContext) cloneVariables() map[string]expression {
    variables := make(map[string]expression)
    for k, v := range e.variables {
        variables[k] = v
    }
    return variables 
}

func (e *evaluationContext) getValue(vexpr *variableExpr) expression {
    if expr, ok := e.variables[vexpr.id]; ok {
        return expr
    }
    if abstr := e.m.getAbstraction(vexpr.id); abstr != nil {
        return abstr
    }

    if val := e.m.getValue(vexpr.id); val !=nil {
        return val
    }

    if imp, ok := e.m.getImport(vexpr.id); ok {
        return e.p.getModule(imp)
    }

    return nil
}

func (e *evaluationContext) addValue(vexpr *variableExpr, expr expression)   { e.variables[vexpr.id] = expr }
func (e *evaluationContext) removeValue(vexpr *variableExpr)                 { delete(e.variables, vexpr.id) }
func (e *evaluationContext) addParticipantChannel(pexpr *participantExpr, sendCh communication.Channel[expression], receiveCh communication.Channel[expression]) {
    e.sendParticipants[pexpr.id] = sendCh
    e.receiveParticipants[pexpr.id] = receiveCh
}

func (e *evaluationContext) getParticipantChannels(pexpr *participantExpr) channelMap {
    m := make(channelMap, 0)
    m[pexpr.id] = make(map[string] communication.Channel[expression])
    for to, v := range e.sendParticipants {
        m[pexpr.id][to] = v
    }
    for from, v := range e.receiveParticipants {
        m[from] = make(map[string] communication.Channel[expression])
        m[from][pexpr.id] = v
    }
    return m
}

func (e *evaluationContext) getSendParticipantChannel(pexpr *participantExpr) communication.Channel[expression] {
    ch, ok := e.sendParticipants[pexpr.id]
    if !ok {
        return nil
    }
    return ch
}

func (e *evaluationContext) getReceiveParticipantChannel(pexpr *participantExpr) communication.Channel[expression] {
    ch, ok := e.receiveParticipants[pexpr.id]
    if !ok {
        return nil
    }
    return ch
}

func (e *evaluationContext) cleanup() {
    for _, ch := range e.sendParticipants {
        ch.Close()
    }
}

func (e *evaluationContext) add()  { e.wg.Add(1) }
func (e *evaluationContext) done() { e.wg.Done() }
func (e *evaluationContext) wait() { e.wg.Wait() }
