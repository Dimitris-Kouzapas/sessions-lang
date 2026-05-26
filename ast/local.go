package ast

import "fmt"
import "sessions/ast/util"

/******************************************************************************
* liveness
*******************************************************************************/

func liveness(locals map[string]local) (global, bool) {
    for p, loc := range locals {
        if  glob, ok := loc.liveness(p, locals); ok {
            return glob, true
        }
    }
    return nil, false
}

/******************************************************************************
 * local
 ******************************************************************************/
type local interface {
    setFilename(string)
    typeCheck(*typeCheckContext, util.ErrorLog)
    projectionCheck(*projectionCheckContext, util.ErrorLog, util.ReportLog)
    substitute(map[string]string) local
    subtypeOf(local, util.Set[typePair]) bool
    join(local) (local, bool)
    equiv(local) bool
    merge(local) (local, bool)
    liveness(string, map[string]local) (global, bool)
    hasParticipant(*participantType) bool
    prettyPrint(util.IndentedWriter)
    fmt.Stringer
}

/******************************************************************************
 * end local
 ******************************************************************************/

type endLocal struct {
    baseNode
}

func newEndLocal(line int) *endLocal {
    return &endLocal {
        baseNode: baseNode {line: line},
    }
}

func (e *endLocal) setFilename(filename string) {
    e.baseNode.setFilename(filename)
}

func (_ *endLocal) typeCheck(_ *typeCheckContext, _ util.ErrorLog) {}
func (_ *endLocal) projectionCheck(_ *projectionCheckContext, _ util.ErrorLog, _ util.ReportLog) {}

func (_ *endLocal) subtypeOf(loc local, visited util.Set[typePair]) bool {
    _, ok := loc.(*endLocal)
    return ok
}

func (e *endLocal) join(loc local) (local, bool) {
    _, ok := loc.(*endLocal)
    return e, ok
}

func (_ *endLocal) equiv(loc local) bool {
    _, ok := loc.(*endLocal)
    return ok
}

func (e *endLocal) merge(loc local) (local, bool) { return e, e.equiv(loc) }
func (e *endLocal) substitute(substitution map[string]string) local { return e }

func (e *endLocal) liveness(_ string, locals map[string]local) (global, bool) {
    for _, loc := range locals {
        if _, ok := loc.(*endLocal); !ok {
            return nil, false
        }
    }
    return newEndGlobal(e.line), true
}

func (_ *endLocal) hasParticipant(_ *participantType) bool  { return false }
func (e *endLocal) prettyPrint(iw util.IndentedWriter)      { iw.Print(e.String()) }
func (_ *endLocal) String() string                          { return "end" }

/******************************************************************************
 * pass local
 ******************************************************************************/

type passLocal struct {
    baseNode
    ptype *participantType
    tdef typedef
    cont local
    symbol string
}

func (p *passLocal) setFilename(filename string) {
    p.baseNode.setFilename(filename)
    p.ptype.setFilename(filename)
    p.tdef.setFilename(filename)
    p.cont.setFilename(filename)
}

func (p *passLocal) typeCheck(ctx *typeCheckContext, log util.ErrorLog) {
    p.tdef.typeCheck(ctx, log)
    p.cont.typeCheck(ctx, log)
}

func (p *passLocal) projectionCheck(ctx *projectionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    if !ctx.containsParticipant(p.ptype) {
        p.reportErrorf(elog, "undefined participant: %q.", p.ptype.String())
    }
    p.tdef.projectionCheck(ctx, elog, rlog)
    p.cont.projectionCheck(ctx, elog, rlog)
}

func (p *passLocal) substitute(substitution map[string]string) (*participantType, local) {
    id, ok := substitution[p.ptype.participant]
    if !ok {
        id = p.ptype.participant
    }
    ptype := newParticipantType(id, p.ptype.line)
    loc := p.cont.substitute(substitution)
    return ptype, loc
}

func (p *passLocal) hasParticipant(ptype *participantType) bool {
    if p.ptype.subtypeOf(ptype) {
        return true
    }
    return p.cont.hasParticipant(ptype)
}

func (p *passLocal) prettyPrint(iw util.IndentedWriter) {
    p.ptype.prettyPrint(iw)
    iw.Print(p.symbol)
    iw.Print("(")
    p.tdef.prettyPrint(iw)
    iw.Println(").")
    p.cont.prettyPrint(iw)
}

func (p *passLocal) String() string {
    s := p.ptype.String() + p.symbol
    s += "(" + p.tdef.String() + "). "
    s += p.cont.String()
    return s
}

/******************************************************************************
 * sendLocal
 ******************************************************************************/

type sendLocal struct {
    passLocal
}

func newSendLocal(ptype *participantType, tdef typedef, cont local, line int) *sendLocal {
    return &sendLocal {
        passLocal: passLocal {
            baseNode: baseNode {line: line},
            ptype:    ptype,
            tdef:     tdef,
            cont:     cont,
            symbol:   "!",
        },
    }
}

func (s *sendLocal) subtypeOf(loc local, visited util.Set[typePair]) bool {
    sloc, ok := loc.(*sendLocal)
    if !ok {
        return false
    }
    if !s.ptype.subtypeOf(sloc.ptype) {
        return false
    }
    if !s.tdef.subtypeOf_(sloc.tdef, visited) {
        return false
    }
    return s.cont.subtypeOf(sloc.cont, visited)
}

func (s *sendLocal) join(loc local) (local, bool) {
    sloc, ok := loc.(*sendLocal)
    if !ok {
        return s, false
    }
    if !s.ptype.subtypeOf(sloc.ptype) {
        return s, false
    }
    var payload typedef
    switch {
        case s.tdef.subtypeOf(sloc.tdef):
            payload = sloc.tdef
        case sloc.tdef.subtypeOf(s.tdef):
            payload = s.tdef
        default:
            // incomparable payloads
            return s, false
    }
    next, ok2 := s.cont.join(sloc.cont)
    return newSendLocal(s.ptype, payload, next, s.line), ok2
}

func (s *sendLocal) equiv(loc local) bool {
    sloc, ok := loc.(*sendLocal)
    if !ok {
        return false
    }
    if !sloc.ptype.subtypeOf(s.ptype) {
        return false
    }
    if !(sloc.tdef.subtypeOf(s.tdef) && s.tdef.subtypeOf(sloc.tdef)) {
        return false
    }
    return s.cont.equiv(sloc.cont)
}

func (s *sendLocal) merge(loc local) (local, bool) {
    return s, s.equiv(loc)
}

func (s *sendLocal) substitute(substitution map[string]string) local {
    ptype, loc := s.passLocal.substitute(substitution)
    return newSendLocal(ptype, s.tdef, loc, s.line)
}

func (s *sendLocal) liveness(p string, locals map[string]local) (global, bool) {
    q := s.ptype.participant
    loc, ok := locals[q]
    if !ok {
        return nil, false
    }

    rloc, ok := loc.(*receiveLocal)
    if !ok {
        return nil, false
    }

    if p != rloc.ptype.participant {
        return nil, false
    }

    if !s.tdef.subtypeOf(rloc.tdef) {
        return nil, false
    }

    newLocals := make(map[string]local, len(locals))
    for k, v := range locals {
        newLocals[k] = v
    }
    newLocals[p] = s.cont
    newLocals[q] = rloc.cont
    if glob, ok := liveness(newLocals); ok {
        return newPassGlobal(newParticipantType(p, s.line), s.ptype, s.tdef, glob, s.line), true
    }
    return nil, false
}

/******************************************************************************
 * receiveLocal
 ******************************************************************************/

type receiveLocal struct {
    passLocal
}

func newReceiveLocal(ptype *participantType, tdef typedef, cont local, line int) *receiveLocal {
    return &receiveLocal {
        passLocal: passLocal {
            baseNode: baseNode {line: line},
            ptype:    ptype,
            tdef:     tdef,
            cont:     cont,
            symbol:   "?",
        },
    }
}

func (r *receiveLocal) subtypeOf(loc local, visited util.Set[typePair]) bool {
    rloc, ok := loc.(*receiveLocal)
    if !ok {
        return false
    }
    if !r.ptype.subtypeOf(rloc.ptype) {
        return false
    }
    if !rloc.tdef.subtypeOf_(r.tdef, visited) {
        return false
    }
    return r.cont.subtypeOf(rloc.cont, visited)
}

func (r *receiveLocal) join(loc local) (local, bool) {
    rloc, ok := loc.(*receiveLocal)
    if !ok {
        return r, false
    }
    if !r.ptype.subtypeOf(rloc.ptype) {
        return r, false
    }
    var payload typedef
    switch {
        case r.tdef.subtypeOf(rloc.tdef):
            payload = r.tdef
        case rloc.tdef.subtypeOf(r.tdef):
            payload = rloc.tdef
        default:
            // incomparable payloads
            return r, false
    }

    next, ok2 := r.cont.join(rloc.cont)
    return newReceiveLocal(r.ptype, payload, next, r.line), ok2
}

func (r *receiveLocal) equiv(loc local) bool {
    rloc, ok := loc.(*receiveLocal)
    if !ok {
        return false
    }
    if !r.ptype.subtypeOf(rloc.ptype) {
        return false
    }
    if !(rloc.tdef.subtypeOf(r.tdef) && r.tdef.subtypeOf(rloc.tdef)) {
        return false
    }
    return r.cont.equiv(rloc.cont)
}

func (r *receiveLocal) merge(loc local) (local, bool) {
    return r, r.equiv(loc)
}

func (r *receiveLocal) substitute(substitution map[string]string) local {
    ptype, loc := r.passLocal.substitute(substitution)
    return newReceiveLocal(ptype, r.tdef, loc, r.line)
}

func (r *receiveLocal) liveness(p string, locals map[string]local) (global, bool) {
    q := r.ptype.participant

    loc, ok := locals[q]
    if !ok {
        return nil, false
    }

    sloc, ok := loc.(*sendLocal)
    if !ok {
        return nil, false
    }

    if p != sloc.ptype.participant {
        return nil, false
    }

    if !sloc.tdef.subtypeOf(r.tdef) {
        return nil, false
    }

    newLocals := make(map[string]local, len(locals))
    for k, v := range locals {
        newLocals[k] = v
    }
    newLocals[p] = r.cont
    newLocals[q] = sloc.cont
    
    if glob, ok := liveness(newLocals); ok {
        return newPassGlobal(r.ptype, newParticipantType(p, r.line), r.tdef, glob, r.line), true
    }
    return nil, false
}

/******************************************************************************
 * choice
 ******************************************************************************/

type choiceLocal struct {
    baseNode
    ptype *participantType
    labels []*labelType
    locals []local
    labelsMap map[string]local
    symbol string
}

func (c *choiceLocal) setFilename(filename string) {
    c.ptype.setFilename(filename)
    for i, loc := range c.locals {
        c.labels[i].setFilename(filename)
        loc.setFilename(filename)
    }
    c.baseNode.setFilename(filename)
}

func (c *choiceLocal) typeCheck(ctx *typeCheckContext, log util.ErrorLog) {
    ctx.resetLabels()
    for _, lab := range c.labels {
        if !ctx.addLabel(lab) {
            lab.reportErrorf(log, "duplicate definition of choice label: %q.", lab.label)
        }
    }
    for _, loc := range c.locals {
        loc.typeCheck(ctx, log)
    }
}

func (c *choiceLocal) projectionCheck(ctx *projectionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    if !ctx.containsParticipant(c.ptype) {
        c.reportErrorf(elog, "undefined participant: %q.", c.ptype.String())
    }
    for _, loc := range c.locals {
        loc.projectionCheck(ctx, elog, rlog)
    }
}

func (c *choiceLocal) substitute(substitution map[string]string) (*participantType, []local) {
    id, ok := substitution[c.ptype.participant]
    if !ok {
        id = c.ptype.participant
    }
    ptype := newParticipantType(id, c.ptype.line)
    locals := make([]local, len(c.locals))
    for i, loc := range c.locals {
        locals[i] = loc.substitute(substitution)
    }
    return ptype, locals
}

func (c *choiceLocal) prettyPrint(iw util.IndentedWriter) {
    iw.Print(c.symbol + " ")
    c.ptype.prettyPrint(iw)
    iw.Println(" {")
    for i := range c.locals {
        if i != 0 {
            iw.Println(" or {")
        }
        iw.Inc()
        c.labels[i].prettyPrint(iw)
        iw.Print(":\t")
        iw.Inc()
        c.locals[i].prettyPrint(iw)
        iw.Dec()
        iw.Dec()
        iw.Println()
        iw.Print("}")
    }
}

func (c *choiceLocal) hasParticipant(ptype *participantType) bool {
    if c.ptype.subtypeOf(ptype) {
        return true
    }
    for _, loc := range c.locals {
        if loc.hasParticipant(ptype) {
            return true
        }
    }
    return false
}

func (c *choiceLocal) String() string {
    s := c.symbol + " " + c.ptype.String() + " { "
    for i := range c.locals {
        if i != 0 {
            s += " or { "
        }
        s += c.labels[i].String() + ": "
        s += c.locals[i].String()
        s += " }"
    }
    return s
}

/******************************************************************************
 * select local
 ******************************************************************************/

type selectLocal struct {
    choiceLocal
}

func newSelectLocal(ptype *participantType, labels []*labelType, locals []local, line int) *selectLocal {
    labelsMap := make(map[string]local)
    for i := range locals {
        labelsMap[labels[i].label] = locals[i]
    }
    return &selectLocal {
        choiceLocal: choiceLocal {
            baseNode: baseNode {line: line},
            ptype: ptype,
            labels: labels,
            locals: locals,
            labelsMap: labelsMap,
            symbol: "select",
        },
    }
}

func (s *selectLocal) subtypeOf(loc local, visited util.Set[typePair]) bool {
    sel, ok := loc.(*selectLocal)
    if !ok {
        return false
    }

    if !s.ptype.subtypeOf(sel.ptype) {
        return false
    }

    for i := range s.locals {
        targetLoc, ok := sel.labelsMap[s.labels[i].label]
        if !ok {
            return false
        }
        if !s.locals[i].subtypeOf(targetLoc, visited) {
            return false
        }
    }
    return true
}

func (s *selectLocal) join(loc local) (local, bool) {
    sel, ok := loc.(*selectLocal)
    if !ok {
        return s, false
    }
    if !s.ptype.subtypeOf(sel.ptype) {
        return s, false
    }
    ok2 := true
    labels := make([]*labelType, 0, len(s.labels))
    locals := make([]local, 0, len(s.locals))
    for i, lab := range s.labels {
        loc1, ok := sel.labelsMap[lab.label]
        var loc2 local
        if ok {
            var matched bool
            loc2, matched = s.locals[i].join(loc1)
            if !matched {
                ok2 = false
                loc2 = s.locals[i]
            }
        } else {
            loc2 = s.locals[i]
        }
        labels = append(labels, lab)
        locals = append(locals, loc2)
    }
    for i, lab := range sel.labels {
        if _, ok := s.labelsMap[lab.label]; !ok {
            labels = append(labels, lab)
            locals = append(locals, sel.locals[i])
        }
    }
    if len(labels) == 0 {
        return s, false
    }
    return newSelectLocal(s.ptype, labels, locals, s.line), ok2
}

func (s *selectLocal) equiv(loc local) bool {
    sel, ok := loc.(*selectLocal)
    if !ok {
        return false
    }
    if !s.ptype.subtypeOf(sel.ptype) || len(s.labels) != len(sel.labels) {
        return false
    }
    for i, lab := range s.labels {
        targetLoc, ok := sel.labelsMap[lab.label]
        if !ok || !s.locals[i].equiv(targetLoc) {
            return false
        }
    }
    return true
}

func (s *selectLocal) merge(loc local) (local, bool) {
    return s, s.equiv(loc)
}

func (s *selectLocal) substitute(substitution map[string]string) local {
    ptype, locals := s.choiceLocal.substitute(substitution)
    return newSelectLocal(ptype, s.labels, locals, s.line)
}

func (s *selectLocal) liveness(p string, locals map[string]local) (global, bool) {
    q := s.ptype.participant
    loc, ok := locals[q]
    if !ok {
        return nil, false
    }

    bra, ok := loc.(*branchLocal)
    if !ok {
        return nil, false
    }

    if p != bra.ptype.participant {
        return nil, false
    }

    for _, lab := range s.labels {
        _, ok := bra.labelsMap[lab.label]
        if !ok {
            return nil, false
        }
    }

    globals := make([]global, len(s.locals))
    for i, lab := range s.labels {
        newLocals := make(map[string]local)
        for k, v := range locals {
            newLocals[k] = v
        }
        newLocals[p] = s.locals[i]
        newLocals[q] = bra.labelsMap[lab.label]
        globals[i], ok = liveness(newLocals)
        if !ok {
            return nil, false
        }
    }
    return newChoiceGlobal(newParticipantType(p, s.line), s.ptype, s.labels, globals, s.line), true
}

/******************************************************************************
 * branch local
 ******************************************************************************/

type branchLocal struct {
    choiceLocal
}

func newBranchLocal(ptype *participantType, labels []*labelType, locals []local, line int) *branchLocal {
    labelsMap := make(map[string]local)
    for i := range locals {
        labelsMap[labels[i].label] = locals[i]
    }
    return &branchLocal {
        choiceLocal: choiceLocal {
            baseNode: baseNode {line: line},
            ptype: ptype,
            labels: labels,
            locals: locals,
            labelsMap: labelsMap,
            symbol: "branch",
        },
    }
}

func (b *branchLocal) subtypeOf(loc local, visited util.Set[typePair]) bool {
    bra, ok := loc.(*branchLocal)
    if !ok {
        return false
    }

    if !b.ptype.subtypeOf(bra.ptype) {
        return false
    }

    for i := range bra.locals {
        targetLoc, ok := b.labelsMap[bra.labels[i].label]
        if !ok {
            return false
        }
        if !targetLoc.subtypeOf(bra.locals[i], visited) {
            return false
        }
    }
    return true
}

func (b *branchLocal) join(loc local) (local, bool) {
    bra, ok1 := loc.(*branchLocal)
    if !ok1 {
        return b, false
    }
    if !b.ptype.subtypeOf(bra.ptype) {
        return b, false
    }
    ok2 := true
    labels := make([]*labelType, 0, len(b.labels))
    locals := make([]local, 0, len(b.locals))
    for i, lab := range b.labels {
        loc1, ok := bra.labelsMap[lab.label]
        if ok {
            labels = append(labels, lab)
            loc2, ok3 := b.locals[i].join(loc1)
            if !ok3 {
                ok2 = false
                loc2 = b.locals[i]
            }
            locals = append(locals, loc2)
        }
    }
    if len(labels) == 0 {
        return b, false
    }
    return newBranchLocal(b.ptype, labels, locals, b.line), ok2
}

func (b *branchLocal) equiv(loc local) bool {
    bra, ok := loc.(*branchLocal)
    if !ok {
        return false
    }
    if !b.ptype.subtypeOf(bra.ptype) || len(b.labels) != len(bra.labels) {
        return false
    }
    for i, lab := range b.labels {
        targetLoc, ok := bra.labelsMap[lab.label]
        if !ok || !b.locals[i].equiv(targetLoc) {
            return false
        }
    }
    return true
}

func (b *branchLocal) merge(loc local) (local, bool) {
    bra, ok := loc.(*branchLocal)
    if !ok {
        return b, false
    }
    if !b.ptype.subtypeOf(bra.ptype) {
        return b, false
    }
    matched := true
    labels := make([]*labelType, 0)
    locals := make([]local, 0)
    for i, lab := range b.labels {
        loc1, ok1 := bra.labelsMap[lab.label]
        ok2 := false
        var loc2 local
        if ok1 {
            loc2, ok2 = b.locals[i].merge(loc1)
            if !ok2 {
                matched = false
            }
        }
        if !ok2 {
            loc2 = b.locals[i]
        }
        labels = append(labels, lab)
        locals = append(locals, loc2)
    }
    for _, lab := range bra.labels {
        _, ok := b.labelsMap[lab.label]
        if !ok {
            labels = append(labels, lab)
            locals = append(locals, bra.labelsMap[lab.label])
        }
    }
    return newBranchLocal(b.ptype, labels, locals, b.line), matched
}

func (b *branchLocal) substitute(substitution map[string]string) local {
    ptype, locals := b.choiceLocal.substitute(substitution)
    return newBranchLocal(ptype, b.labels, locals, b.line)
}

func (b *branchLocal) liveness(p string, locals map[string]local) (global, bool) {
    q := b.ptype.participant

    loc, ok := locals[q]
    if !ok {
        return nil, false
    }

    sel, ok := loc.(*selectLocal)
    if !ok {
        return nil, false
    }

    if p != sel.ptype.participant {
        return nil, false
    }

    for _, lab := range sel.labels {
        _, ok := b.labelsMap[lab.label]
        if !ok {
            return nil, false
        }
    }

    globals := make([]global, len(sel.locals))
    for i, lab := range sel.labels {
        newLocals := make(map[string]local)
        for k, v := range locals {
            newLocals[k] = v
        }
        newLocals[p] = sel.locals[i]
        newLocals[q] = b.labelsMap[lab.label]
        globals[i], ok = liveness(newLocals)
        if !ok {
            return nil, false
        }
    }
    // return newChoiceGlobal(b.ptype, newParticipantType(p, b.line), b.labels, globals, b.line), true
    return newChoiceGlobal(b.ptype, newParticipantType(p, b.line), sel.labels, globals, b.line), true
}

/******************************************************************************
 * rec local
 ******************************************************************************/

type recLocal struct {
    baseNode
}

func newRecLocal(line int) *recLocal {
    return &recLocal {
        baseNode: baseNode {line: line},
    }
}

func (r *recLocal) setFilename(filename string) {
    r.baseNode.setFilename(filename)
}

func (_ *recLocal) typeCheck(_ *typeCheckContext, _ util.ErrorLog) {}
func (_ *recLocal) projectionCheck(_ *projectionCheckContext, _ util.ErrorLog, _ util.ReportLog) {}

func (_ *recLocal) subtypeOf(loc local, _ util.Set[typePair]) bool {
    _, ok := loc.(*recLocal)
    return ok
}

func (r *recLocal) join(loc local) (local, bool) {
    _, ok := loc.(*recLocal)
    return r, ok
}

func (_ *recLocal) equiv(loc local) bool {
    _, ok := loc.(*recLocal)
    return ok
}

func (r *recLocal) merge(loc local) (local, bool) { return r, r.equiv(loc) }
func (r *recLocal) substitute(substitution map[string]string) local { return r }

func (r *recLocal) liveness(_ string, locals map[string]local) (global, bool) {
    for _, loc := range locals {
        if _, ok := loc.(*recLocal); !ok {
            return nil, false
        }
    }
    return newRecGlobal(r.line), true
}

func (_ *recLocal) hasParticipant(_ *participantType) bool  { return false }
func (r *recLocal) prettyPrint(iw util.IndentedWriter)      { iw.Print(r.String()) }
func (_ *recLocal) String() string                          { return "rec" }
