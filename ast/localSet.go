package ast

import (
    "sessions/ast/util"
)

/******************************************************************************
 * local context
 ******************************************************************************/

type localSet struct {
    globalConfigImpl
    locals []local
    localMap map[string]local
}

func newLocalSet(ptypes []*participantType, locals []local, line int) *localSet {
    ls := &localSet {
        globalConfigImpl: globalConfigImpl {
            baseNode: baseNode{line: line},
            ptypes: ptypes,
        },
        locals: locals,
        localMap: make(map[string]local, len(locals)),
    }
    for i, loc := range locals {
        ls.localMap[ls.participant(i)] = loc
    }
    return ls
}

func (ls *localSet) setFilename(filename string) {
    ls.globalConfigImpl.setFilename(filename)
    for _, loc := range ls.locals {
        loc.setFilename(filename)
    }
}

func (ls *localSet) getType() typedef { return ls }

func (ls *localSet) typeCheck(ctx *typeCheckContext, log util.ErrorLog) {
    ls.globalConfigImpl.typeCheck(ctx, log)
    for _, loc := range ls.locals {
        loc.typeCheck(ctx, log)
    }
}

func (ls *localSet) projectionCheck(ctx *projectionCheckContext, elog util.ErrorLog, rlog util.ReportLog) {
    ctx.push()
    defer ctx.pop()
    ls.globalConfigImpl.projectionCheck(ctx, elog, rlog)
}

func (ls *localSet) subtypeOf(tdef typedef) bool {
    visited := util.NewHashSet[typePair]()
    return ls.subtypeOf_(tdef, visited)
}

func (ls *localSet) subtypeOf_(tdef typedef, visited util.Set[typePair]) bool {
    tPair := typePair{a: ls, b:tdef}

    if visited.Contains(tPair) {
        return true
    }

    visited.Add(tPair)

    switch gconfig := tdef.getType().(type) {
        case *nothingType:
            return true
        case *localSet:
            for i, loc1 := range gconfig.locals {
                p := gconfig.participant(i)
                loc2, ok := ls.localMap[p]
                if !ok || !loc2.subtypeOf(loc1, visited){
                    return false
                }
            }
            return true
        case *localContext:
            for i, loc1 := range gconfig.locals {
                p := gconfig.participant(i)
                loc2, ok := ls.localMap[p]
                if !ok || !loc2.subtypeOf(loc1, visited){
                    return false
                }
            }
            return true
        case *globalDef:
            lcont := gconfig.projectionSet()
            return ls.subtypeOf_(lcont, visited)
        default:
            return false
    }
}

func (ls *localSet) join(tdef typedef) (typedef, bool) {
    switch gconfig := tdef.getType().(type) {
        case *nothingType:
            return ls, true
        case *localSet:
            locals := make([]local, 0)
            participants := make([]*participantType, 0)
            // Do not sustitute names here.
            for i, loc1 := range gconfig.locals {
                p := gconfig.participant(i)                
                loc2, ok := ls.localMap[p]
                if !ok {
                    return ls, false
                }
                loc, ok := loc2.join(loc1)
                if !ok {
                    return ls, false
                }
                participants = append(participants, gconfig.participants()[i])
                locals = append(locals, loc)
            }
            return newLocalSet(participants, locals, ls.line), true
        case *localContext:
            locals := make([]local, 0)
            participants := make([]*participantType, 0)
            // Do not sustitute names here.
            for i, loc1 := range gconfig.locals {
                p := gconfig.participant(i)                
                loc2, ok := ls.localMap[p]
                if !ok {
                    return ls, false
                }
                loc, ok := loc2.join(loc1)
                if !ok {
                    return ls, false
                }
                participants = append(participants, gconfig.participants()[i])
                locals = append(locals, loc)
            }
            return newLocalSet(participants, locals, ls.line), true
        case *globalDef:
            lcont := gconfig.projectionSet()
            return ls.join(lcont)
        default:
            return ls, false
    }
}

func (ls *localSet) substitute(ptypes []*participantType) *localSet {
    substitution, participants := ls.globalConfigImpl.substitute(ptypes)
    locals := make([]local, len(ls.locals))
    for i := range ls.locals {
        locals[i] = ls.locals[i].substitute(substitution)
    }
    return newLocalSet(participants, locals, ls.line)
}

func (ls *localSet) project(ptype *participantType) *localAbstraction {
    var loc local = newEndLocal(ls.line)
    for i, pt := range ls.participants() {
        if ptype.subtypeOf(pt) {
            loc = ls.locals[i]
            break
        }
    }
    participants := make([]*participantType, 0, len(ls.participants()))
    for _, p := range ls.participants() {
        if loc.hasParticipant(p) {
            participants = append(participants, p)
        }
    }
    return newLocalAbstraction(participants, loc, ls.line)
}

func (ls *localSet) defaultValue() expression {
    return newNothingf(ls.line, "unexpected local context type")
}

func (ls *localSet) prettyPrint(iw util.IndentedWriter) {
    iw.Println("{")
    iw.Inc()
    for i := range ls.participants() {
        ls.participants()[i].prettyPrint(iw)
        iw.Print(":\t")
        iw.Inc()
        ls.locals[i].prettyPrint(iw)
        iw.Dec()
        iw.Println()
    }
    iw.Dec()
    iw.Println("}")
    return
}

func (ls *localSet) String() string {
    s := "{"
    for i := range ls.participants() {
        s += " " + ls.participants()[i].String() + ": " + ls.locals[i].String()
    }
    return s + " }"
}